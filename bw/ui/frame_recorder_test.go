package ui

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/visualfc/atk/tk"
)

// --- framebuffer capture
//
// `Xvfb -fbdir DIR` keeps its framebuffer in a memory-mapped XWD file, so a test can
// watch what is on screen without the X round trips of `xwd` or `import`. the recorder
// polls that file and keeps one entry per distinct frame: the sequence of frames is what
// the user would see, and the gaps between them are how long each was visible.

// the fixed part of an XWD file header: 25 big-endian 32-bit fields.
const xwd_header_fixed_size = 100

// a frame that differed from the one before it.
type frame struct {
	At     time.Duration // since recording started
	Hash   uint32
	Pixels []byte // a copy of the raw pixel data, BGRX
}

// what `Xvfb -fbdir` wrote: the geometry needed to read the pixels back.
type xwd_layout struct {
	Width, Height  int
	BytesPerLine   int
	BitsPerPixel   int
	PixelOffset    int
	PixelByteCount int
}

// parses the layout of the XWD file `data`.
// only the 32 bits-per-pixel layout Xvfb writes for 24 bit screens is supported.
func parse_xwd_layout(data []byte) (xwd_layout, error) {
	if len(data) < xwd_header_fixed_size {
		return xwd_layout{}, fmt.Errorf("xwd header too short: %d bytes", len(data))
	}
	field := func(i int) int { return int(binary.BigEndian.Uint32(data[i*4 : i*4+4])) }
	layout := xwd_layout{
		Width:        field(4),
		Height:       field(5),
		BitsPerPixel: field(11),
		BytesPerLine: field(12),
	}
	header_size := field(0)
	ncolors := field(19)
	if layout.BitsPerPixel != 32 {
		return xwd_layout{}, fmt.Errorf("unsupported xwd bits per pixel: %d", layout.BitsPerPixel)
	}
	layout.PixelOffset = header_size + ncolors*12
	layout.PixelByteCount = layout.BytesPerLine * layout.Height
	if len(data) < layout.PixelOffset+layout.PixelByteCount {
		return xwd_layout{}, fmt.Errorf("xwd file too short: %d bytes, need %d", len(data), layout.PixelOffset+layout.PixelByteCount)
	}
	return layout, nil
}

// returns the pixels of `fr` as an image, cropped to `region` when it is not empty.
func frame_image(layout xwd_layout, fr frame, region image.Rectangle) *image.RGBA {
	if region.Empty() {
		region = image.Rect(0, 0, layout.Width, layout.Height)
	}
	img := image.NewRGBA(image.Rect(0, 0, region.Dx(), region.Dy()))
	for y := region.Min.Y; y < region.Max.Y; y++ {
		row := fr.Pixels[y*layout.BytesPerLine:]
		for x := region.Min.X; x < region.Max.X; x++ {
			px := row[x*4 : x*4+4]
			img.Set(x-region.Min.X, y-region.Min.Y, color.RGBA{R: px[2], G: px[1], B: px[0], A: 255})
		}
	}
	return img
}

// whether the pixels of `region` differ between the frames `a` and `b`.
func region_changed(layout xwd_layout, a frame, b frame, region image.Rectangle) bool {
	for y := region.Min.Y; y < region.Max.Y; y++ {
		from, to := y*layout.BytesPerLine+region.Min.X*4, y*layout.BytesPerLine+region.Max.X*4
		if !bytes.Equal(a.Pixels[from:to], b.Pixels[from:to]) {
			return true
		}
	}
	return false
}

// the colour of the pixel at `x`,`y` in `fr`.
func frame_pixel(layout xwd_layout, fr frame, x int, y int) color.RGBA {
	px := fr.Pixels[y*layout.BytesPerLine+x*4:]
	return color.RGBA{R: px[2], G: px[1], B: px[0], A: 255}
}

type frame_recorder struct {
	path   string
	layout xwd_layout
	poll   time.Duration

	mu      sync.Mutex
	started time.Time
	frames  []frame
	stop_ch chan struct{}
	done_ch chan struct{}
}

// returns a recorder watching the framebuffer file of the X display in the directory
// named by the `XVFB_FBDIR` environment variable.
// returns nil when the variable is unset, so callers can skip frame metrics.
func new_frame_recorder() (*frame_recorder, error) {
	dir := os.Getenv("XVFB_FBDIR")
	if dir == "" {
		return nil, nil
	}
	path := filepath.Join(dir, "Xvfb_screen0")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	layout, err := parse_xwd_layout(data)
	if err != nil {
		return nil, err
	}
	return &frame_recorder{path: path, layout: layout, poll: time.Millisecond}, nil
}

// starts polling. the first frame recorded is the screen as it was at the start.
func (rec *frame_recorder) start() {
	rec.mu.Lock()
	rec.frames = nil
	rec.mu.Unlock()
	rec.stop_ch = make(chan struct{})
	rec.done_ch = make(chan struct{})
	rec.started = time.Now()
	started := rec.started
	go func() {
		defer close(rec.done_ch)
		file, err := os.Open(rec.path)
		if err != nil {
			return
		}
		defer file.Close()
		buf := make([]byte, rec.layout.PixelByteCount)
		var last_hash uint32
		table := crc32.MakeTable(crc32.Castagnoli)
		for {
			select {
			case <-rec.stop_ch:
				return
			default:
			}
			_, err := file.ReadAt(buf, int64(rec.layout.PixelOffset))
			if err != nil {
				return
			}
			hash := crc32.Checksum(buf, table)
			if hash != last_hash {
				last_hash = hash
				rec.mu.Lock()
				rec.frames = append(rec.frames, frame{At: time.Since(started), Hash: hash, Pixels: append([]byte(nil), buf...)})
				rec.mu.Unlock()
			}
			time.Sleep(rec.poll)
		}
	}()
}

// stops polling and returns the distinct frames seen since `start`, the first being the
// screen before anything happened.
func (rec *frame_recorder) stop() []frame {
	close(rec.stop_ch)
	<-rec.done_ch
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return rec.frames
}

// returns once the screen has been still for `quiet`, or after `limit`.
func (rec *frame_recorder) wait_quiet(quiet time.Duration, limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		rec.mu.Lock()
		n := len(rec.frames)
		still := n > 0 && time.Since(rec.started)-rec.frames[n-1].At > quiet
		rec.mu.Unlock()
		if still {
			return
		}
	}
}

// the number of distinct frames seen so far.
func (rec *frame_recorder) frame_count() int {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	return len(rec.frames)
}

// stops polling once the screen has been still for `quiet`, or after `limit`.
func (rec *frame_recorder) stop_when_quiet(quiet time.Duration, limit time.Duration) []frame {
	rec.wait_quiet(quiet, limit)
	return rec.stop()
}

// writes `frames` as PNG files into `dir`, cropped to `region`, one per frame.
// used to look at what changed between frames.
func dump_frames(layout xwd_layout, frames []frame, region image.Rectangle, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, fr := range frames {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("frame-%02d-%05dms.png", i, fr.At.Milliseconds())))
		if err != nil {
			return err
		}
		err = png.Encode(f, frame_image(layout, fr, region))
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// a summary of `frames`: how many distinct frames were drawn after the first, and the
// time between the first change and the last.
func frame_summary(frames []frame) string {
	if len(frames) < 2 {
		return "0 frames"
	}
	times := []string{}
	for _, fr := range frames[1:] {
		times = append(times, fmt.Sprintf("%dms", fr.At.Milliseconds()))
	}
	settle := frames[len(frames)-1].At - frames[1].At
	return fmt.Sprintf("%d frames over %dms at [%s]", len(frames)-1, settle.Milliseconds(), strings.Join(times, " "))
}

// --- Tk-level counters
//
// counts calls to the tablelist procs that repaint rows, and to Tk's `update`, which
// forces a repaint before the command issuing it has finished. these are deterministic,
// unlike frames, so tests can assert on them.

var tk_counted_procs = []string{
	"::update",
	"::tablelist::makeStripes",
	"::tablelist::redisplayCol",
	"::tablelist::computeColWidth",
	"::tablelist::displayItems",
	"::tablelist::doRowConfig",
	"::tablelist::adjustColumns",
	"::tablelist::configrowlistSubCmd",
	"::tablelist::updateCell",
	"::tablelist::findTabs",
	"::tablelist::adjustElem",
	"::tablelist::getAuxData",
	"::tablelist::getIndentData",
	"::tablelist::updateColors",
	"::tablelist::redisplay",
	"::tablelist::redisplayVisibleItems",
	"::tablelist::adjustElidedText",
	"::tablelist::adjustSeps",
	"::tablelist::updateVScrlbar",
	"::tablelist::displayIndent",
}

// wraps each counted proc so that calling it bumps a counter and adds its running time,
// in milliseconds, to a total. the original runs at the caller's level, so procs
// inspecting their caller still work. idempotent.
// must be called on the Tk thread.
func install_tk_counters() error {
	script := `
namespace eval ::flicker {
    variable counts
    variable millis
    array set counts {}
    array set millis {}
    variable installed
}
if {![info exists ::flicker::installed]} {
    set ::flicker::installed 1
    foreach name {` + strings.Join(tk_counted_procs, " ") + `} {
        set ::flicker::counts($name) 0
        set ::flicker::millis($name) 0
        rename $name ${name}__orig
        proc $name args "
            incr ::flicker::counts($name)
            set t \[clock microseconds\]
            set result \[uplevel 1 \[list ${name}__orig {*}\$args\]\]
            set ::flicker::millis($name) \[expr {\$::flicker::millis($name) + (\[clock microseconds\] - \$t) / 1000.0}\]
            return \$result
        "
    }
}
`
	return tk.MainInterp().Eval(script)
}

// zeroes every counter.
// must be called on the Tk thread.
func reset_tk_counters() error {
	return tk.MainInterp().Eval(`
foreach name [array names ::flicker::counts] { set ::flicker::counts($name) 0 }
foreach name [array names ::flicker::millis] { set ::flicker::millis($name) 0 }`)
}

// returns the total running time of each counted proc, in milliseconds, keyed by name.
// must be called on the Tk thread.
func read_tk_millis() (map[string]float64, error) {
	return read_tk_number_array("::flicker::millis")
}

// returns the Tcl array `name` of numbers as a map, keyed by element name.
// must be called on the Tk thread.
func read_tk_number_array(name string) (map[string]float64, error) {
	out, err := tk.MainInterp().EvalAsString("array get " + name)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(out)
	numbers := map[string]float64{}
	for i := 0; i+1 < len(fields); i += 2 {
		var n float64
		fmt.Sscanf(fields[i+1], "%g", &n)
		numbers[fields[i]] = n
	}
	return numbers, nil
}

// returns a copy of the counters, keyed by proc name.
// must be called on the Tk thread.
func read_tk_counters() (map[string]int, error) {
	numbers, err := read_tk_number_array("::flicker::counts")
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for name, n := range numbers {
		counts[name] = int(n)
	}
	return counts, nil
}

// renders counters as "name=n" pairs, with the running time of any proc that took a
// millisecond or more, sorted, for logs.
func format_tk_counters(counts map[string]int, millis map[string]float64) string {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	bits := []string{}
	for _, name := range names {
		bit := fmt.Sprintf("%s=%d", strings.TrimPrefix(name, "::tablelist::"), counts[name])
		if millis[name] >= 1 {
			bit += fmt.Sprintf("(%.0fms)", millis[name])
		}
		bits = append(bits, bit)
	}
	return strings.Join(bits, " ")
}
