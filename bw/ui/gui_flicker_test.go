package ui

import (
	"bw/core"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/visualfc/atk/tk"
)

// --- row expansion repaint measurement
//
// expanding a row should look like one change: the children appear below their parent
// and the rows beneath move down. every extra frame between the click and the settled
// table is flicker. these helpers measure how many frames an expand takes, and how many
// repaint procs tablelist ran to get there.

const flicker_ns_major = "flicker"

// a row with the shape of an installed addon: a name and a handful of short fields.
type flicker_item struct {
	Name string
}

var flicker_item_keys = []string{core.ITEM_FIELD_NAME, "source", "source-id", "installed-version", "available-version", "game-version", "updated-date", "size"}

func (item flicker_item) ItemKeys() []string { return flicker_item_keys }

func (item flicker_item) ItemMap() map[string]string {
	return map[string]string{
		core.ITEM_FIELD_NAME: item.Name,
		"source":             "github",
		"source-id":          "someone/" + item.Name,
		"installed-version":  "1.2.3",
		"available-version":  "1.2.4",
		"game-version":       "11.0.2",
		"updated-date":       "2026-10-01",
		"size":               "1.2MB",
	}
}

func (item flicker_item) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (item flicker_item) ItemChildren(*core.App) []core.Result { return nil }

var _ core.ItemInfo = flicker_item{}

// a row whose children load on demand, as a directory in the files tab does. loading
// returns `Dirs` further lazy rows and `Files` leaves.
type lazy_item struct {
	Name  string
	Dirs  int
	Files int
}

func (item lazy_item) ItemKeys() []string { return []string{core.ITEM_FIELD_NAME} }

func (item lazy_item) ItemMap() map[string]string {
	return map[string]string{core.ITEM_FIELD_NAME: item.Name}
}

func (item lazy_item) ItemHasChildren() core.ITEM_CHILDREN_LOAD { return core.ITEM_CHILDREN_LOAD_LAZY }

func (item lazy_item) ItemChildren(*core.App) []core.Result {
	children := []core.Result{}
	for i := range item.Dirs {
		name := fmt.Sprintf("%s/sub-%02d", item.Name, i)
		children = append(children, core.MakeResult(core.MakeNS(lazy_ns_major, "fs", "dir"), lazy_item{Name: name}, name))
	}
	for i := range item.Files {
		name := fmt.Sprintf("%s/file-%02d", item.Name, i)
		children = append(children, core.MakeResult(core.MakeNS(lazy_ns_major, "fs", "file"), leaf_item{Name: name}, name))
	}
	return children
}

// a row that never has children, as a file in the files tab.
type leaf_item struct {
	Name string
}

func (item leaf_item) ItemKeys() []string { return []string{core.ITEM_FIELD_NAME} }

func (item leaf_item) ItemMap() map[string]string {
	return map[string]string{core.ITEM_FIELD_NAME: item.Name}
}

func (item leaf_item) ItemHasChildren() core.ITEM_CHILDREN_LOAD { return core.ITEM_CHILDREN_LOAD_FALSE }

func (item leaf_item) ItemChildren(*core.App) []core.Result { return nil }

var _ core.ItemInfo = lazy_item{}
var _ core.ItemInfo = leaf_item{}

const lazy_ns_major = "lazyfs"

// returns `count` top-level lazy rows, each loading three lazy rows and two leaves.
func lazy_results(count int) []core.Result {
	result_list := []core.Result{}
	for i := range count {
		name := fmt.Sprintf("dir-%02d", i)
		result_list = append(result_list, core.MakeResult(core.MakeNS(lazy_ns_major, "fs", "dir"), lazy_item{Name: name, Dirs: 3, Files: 2}, name))
	}
	return result_list
}

// the columns shown for `flicker_item` rows, name first so it carries the tree.
func flicker_columns() []UIColumn {
	column_list := []UIColumn{}
	for _, key := range flicker_item_keys {
		// the app hides columns too
		column_list = append(column_list, UIColumn{Title: key, Hidden: key == "source-id" || key == "size"})
	}
	return column_list
}

// returns `top_level` top-level results, the one at `parent_pos` having `children` child
// results, all in the `flicker` namespace so a tab can filter for them.
// child names are no longer than their parent's, so expanding does not widen a column.
func flicker_results(top_level int, parent_pos int, children int) []core.Result {
	ns := core.MakeNS(flicker_ns_major, "test", "item")
	result_list := []core.Result{}
	for i := range top_level {
		id := fmt.Sprintf("row-%02d", i)
		result_list = append(result_list, core.MakeResult(ns, flicker_item{Name: fmt.Sprintf("SomeAddon%02d", i)}, id))
	}
	parent_id := result_list[parent_pos].ID
	for i := range children {
		id := fmt.Sprintf("%s/child-%02d", parent_id, i)
		child := core.MakeResult(ns, flicker_item{Name: fmt.Sprintf("Sub%02d", i)}, id)
		child.ParentID = parent_id
		result_list = append(result_list, child)
	}
	// a second parent whose children are wider than any top-level row, as an addon's
	// sub-directories often are. expanding it widens the tree column.
	wide_parent := result_list[parent_pos+7].ID
	for i := range 3 {
		id := fmt.Sprintf("%s/child-%02d", wide_parent, i)
		child := core.MakeResult(ns, flicker_item{Name: fmt.Sprintf("SomeAddon10_Options_Extra%02d", i)}, id)
		child.ParentID = wide_parent
		result_list = append(result_list, child)
	}
	return result_list
}

// the screen rectangle covered by the tablelist of `tab`.
// must be called on the Tk thread.
func tablelist_screen_region(tab *GUITab) image.Rectangle {
	id := tab.table_widj.Tablelist.Id()
	out, err := tk.MainInterp().EvalAsString(fmt.Sprintf("list [winfo rootx %s] [winfo rooty %s] [winfo width %s] [winfo height %s]", id, id, id, id))
	if err != nil {
		return image.Rectangle{}
	}
	fields := strings.Fields(out)
	if len(fields) != 4 {
		return image.Rectangle{}
	}
	x, _ := strconv.Atoi(fields[0])
	y, _ := strconv.Atoi(fields[1])
	w, _ := strconv.Atoi(fields[2])
	h, _ := strconv.Atoi(fields[3])
	return image.Rect(x, y, x+w, y+h)
}

// the screen coordinates of a point on the row `fkey` in `tab`: half way down it and a
// little way in from its left edge, where its name is.
// errors when the point is off screen, as it is when the display is smaller than the
// window.
// must be called on the Tk thread.
func row_screen_point(tab *GUITab, fkey string) (int, int, error) {
	id := tab.table_widj.Tablelist.Id()
	// `bbox` is relative to the tablelist widget, header included
	out, err := tk.MainInterp().EvalAsString(fmt.Sprintf("list {*}[%s bbox %s] [winfo rootx %s] [winfo rooty %s]", id, fkey, id, id))
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 6 {
		return 0, 0, fmt.Errorf("unexpected bbox output: %q", out)
	}
	n := make([]int, 6)
	for i, f := range fields {
		n[i], _ = strconv.Atoi(f)
	}
	x, y := n[4]+n[0]+60, n[5]+n[1]+n[3]/2
	screen, err := tk.MainInterp().EvalAsString("list [winfo screenwidth .] [winfo screenheight .]")
	if err != nil {
		return 0, 0, err
	}
	var width, height int
	fmt.Sscanf(screen, "%d %d", &width, &height)
	if x < 0 || y < 0 || x >= width || y >= height {
		return 0, 0, fmt.Errorf("row point %d,%d is off a %dx%d screen", x, y, width, height)
	}
	return x, y, nil
}

// whether the row `fkey` of `tab` is drawn in a stripe.
// must be called on the Tk thread.
func row_striped(tab *GUITab, fkey string) bool {
	id := tab.table_widj.Tablelist.Id()
	out, err := tk.MainInterp().EvalAsString(fmt.Sprintf("expr {[lsearch -exact [[%s bodypath] tag names [expr {[%s index %s] + 1}].0] stripe] >= 0}", id, id, fkey))
	return err == nil && out == "1"
}

// the stripe of every result in `id_list`, read on the Tk thread.
func row_stripes(gui *GUIUI, tab *GUITab, id_list ...string) map[string]bool {
	stripes := map[string]bool{}
	gui.TkSync(func() {
		for _, id := range id_list {
			stripes[id] = row_striped(tab, tab.ItemFkeyIndex[id])
		}
	})
	return stripes
}

// the outcome of one measured operation.
type repaint_measurement struct {
	Frames   []frame
	Counters map[string]int
}

// runs `op` and returns the frames drawn and the tablelist procs called until the
// screen has been still for 300ms.
// frames are nil when there is no framebuffer to watch, see `new_frame_recorder`.
// frames are written as images to the directory named by `BW_FRAME_DUMP_DIR`, under
// `label`, when that variable is set.
func measure_repaints(t *testing.T, gui *GUIUI, tab *GUITab, label string, op func()) repaint_measurement {
	t.Helper()

	rec, err := new_frame_recorder()
	require.NoError(t, err)

	var region image.Rectangle
	var install_err, reset_err error
	gui.TkSync(func() {
		install_err = install_tk_counters()
		reset_err = reset_tk_counters()
		region = tablelist_screen_region(tab)
	})
	require.NoError(t, install_err)
	require.NoError(t, reset_err)
	require.Greater(t, region.Dx(), 1, "tablelist is not on screen")

	baseline := 0
	if rec != nil {
		rec.start()
		// a baseline frame, and whatever was still being painted before the operation
		rec.wait_quiet(100*time.Millisecond, 2*time.Second)
		baseline = max(rec.frame_count()-1, 0)
	}

	// a marker in the window title, for anything tracing the X connection
	mark := func(phase string) {
		gui.TkSync(func() { tk.MainInterp().Eval(fmt.Sprintf("wm title . {MARK %s %s}", phase, label)) })
	}
	mark("start")
	op()
	var op_end time.Duration
	if rec != nil {
		op_end = time.Since(rec.started)
	}

	var measurement repaint_measurement
	if rec != nil {
		// from the settled screen before the operation onwards
		measurement.Frames = rec.stop_when_quiet(300*time.Millisecond, 3*time.Second)[baseline:]
	} else {
		time.Sleep(300 * time.Millisecond)
	}
	mark("end")
	var millis map[string]float64
	gui.TkSync(func() {
		measurement.Counters, err = read_tk_counters()
		if err == nil {
			millis, err = read_tk_millis()
		}
	})
	require.NoError(t, err)

	lag := ""
	if n := len(measurement.Frames); n > 1 && measurement.Frames[n-1].At > op_end {
		lag = fmt.Sprintf(", last frame %dms after the operation ended", (measurement.Frames[n-1].At - op_end).Milliseconds())
	}
	t.Logf("%s: %s%s", label, frame_summary(measurement.Frames), lag)
	t.Logf("%s: %s", label, format_tk_counters(measurement.Counters, millis))

	if dir := os.Getenv("BW_FRAME_DUMP_DIR"); dir != "" && rec != nil {
		require.NoError(t, dump_frames(rec.layout, measurement.Frames, region, filepath.Join(dir, label)))
	}
	return measurement
}

// runs xdotool with `args`, driving a real pointer through the X server.
func xdotool(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("xdotool", args...).CombinedOutput()
	require.NoError(t, err, string(out))
}

// double clicks the screen position `x`,`y` with a real pointer, through the X server.
// Tk counts a press within 500ms of the previous one as part of the same multi-click, so
// a pause first keeps this double click from becoming the tail of the last one.
func xdotool_double_click(t *testing.T, x int, y int) {
	t.Helper()
	time.Sleep(600 * time.Millisecond)
	out, err := exec.Command("xdotool", "mousemove", strconv.Itoa(x), strconv.Itoa(y), "click", "--repeat", "2", "--delay", "40", "1").CombinedOutput()
	require.NoError(t, err, string(out))
	park_pointer(t)
}

// moves the pointer to the bottom right of the screen: Xvfb draws its cursor into the
// framebuffer, where it would count as a change to the table.
func park_pointer(t *testing.T) {
	t.Helper()
	var corner string
	gui_for_pointer.TkSync(func() {
		corner, _ = tk.MainInterp().EvalAsString("list [expr {[winfo screenwidth .] - 1}] [expr {[winfo screenheight .] - 1}]")
	})
	fields := strings.Fields(corner)
	require.Len(t, fields, 2)
	xdotool(t, "mousemove", fields[0], fields[1])
}

// the GUI whose Tk thread `park_pointer` asks for the screen size, set by `flicker_subtests`.
var gui_for_pointer *GUIUI

// the child row count of `result_id` in `tab`, read on the Tk thread.
func row_child_count(gui *GUIUI, tab *GUITab, result_id string) int {
	var count int
	gui.TkSync(func() { count = tab.RowChildCount(result_id) })
	return count
}

// whether the row of `result_id` in `tab` is expanded, read on the Tk thread.
func row_expanded(gui *GUIUI, tab *GUITab, result_id string) bool {
	var expanded bool
	gui.TkSync(func() { expanded = tab.RowExpanded(result_id) })
	return expanded
}

// measures expanding a row whose children load on demand, as the files tab does.
// the tab shows the name and namespace of each row, as the files tab does.
func lazy_flicker_subtests(t *testing.T, app *core.App, gui *GUIUI) {
	gui.AddTab("lazy", func(r core.Result) bool { return r.NS.Major == lazy_ns_major })
	tab := gui.GetTab("lazy")
	tab.SetColumnAttrs([]UIColumn{{Title: core.ITEM_FIELD_NAME}, {Title: "ns"}})
	gui.SetActiveTab("lazy")
	gui.ApplyTablelistStyling()
	gui.TkSync(func() { gui.offscreen_tables() }) // the tab is new, see `GUIUI.Show`

	app.AddReplaceResults(lazy_results(20)...).Wait()
	gui.WaitForIdle()

	parent_id := "dir-03"
	var fkey string
	gui.TkSync(func() { fkey = tab.ItemFkeyIndex[parent_id] })
	require.NotEmpty(t, fkey)

	t.Run("expand a lazy row by command", func(t *testing.T) {
		var hidden string
		gui.TkSync(func() {
			hidden, _ = tk.MainInterp().EvalAsString(tab.table_widj.Tablelist.Id() + " rowcget " + tab.placeholder_rows[parent_id] + " -hide")
		})
		assert.Equal(t, "1", hidden, "the placeholder starts hidden")
		measure_repaints(t, gui, tab, "lazy-expand-command", func() {
			tab.ExpandRow(fkey)
			gui.WaitForIdle() // the children load and arrive
		})
		assert.Equal(t, 5, row_child_count(gui, tab, parent_id))
		measure_repaints(t, gui, tab, "lazy-collapse-command", func() { tab.CollapseRow(fkey) })
	})

	if _, err := exec.LookPath("xdotool"); err == nil {
		var other_fkey string
		gui.TkSync(func() { other_fkey = tab.ItemFkeyIndex["dir-05"] })
		t.Run("expand a lazy row by double click", func(t *testing.T) {
			var x, y int
			var err error
			gui.TkSync(func() { x, y, err = row_screen_point(tab, other_fkey) })
			if err != nil {
				t.Skip(err.Error())
			}
			measure_repaints(t, gui, tab, "lazy-expand-click", func() {
				xdotool_double_click(t, x, y)
				gui.WaitForIdle()
			})
			assert.Equal(t, 5, row_child_count(gui, tab, "dir-05"))
		})
	}
}

// measures expanding and collapsing a row, by command and by double click.
// a sub-test of `Test_gui_live`: only one Tk interpreter can run per process.
func flicker_subtests(t *testing.T, app *core.App, gui *GUIUI) {
	gui_for_pointer = gui
	gui.AddTab("flicker", func(r core.Result) bool { return r.NS.Major == flicker_ns_major })
	tab := gui.GetTab("flicker")
	tab.SetColumnAttrs(flicker_columns())
	gui.SetActiveTab("flicker")
	gui.ApplyTablelistStyling() // the theme's stripes, fonts and separators, as the app has them
	gui.Show()
	require.Eventually(t, func() bool {
		var region image.Rectangle
		gui.TkSync(func() { region = tablelist_screen_region(tab) })
		return region.Dx() > 1
	}, 5*time.Second, 20*time.Millisecond, "window never mapped")

	var offscreen bool
	gui.TkSync(func() { offscreen = tk.OffscreenSupported() })
	t.Logf("tables painted off screen: %v", offscreen && os.Getenv("BW_OFFSCREEN") != "0")

	result_list := flicker_results(60, 3, 5)
	app.AddReplaceResults(result_list...).Wait()
	gui.WaitForIdle()
	tab.SetColumnAttrs(flicker_columns()) // hides columns, now that they exist

	parent_id := "row-03"
	var fkey string
	gui.TkSync(func() { fkey = tab.ItemFkeyIndex[parent_id] })
	require.NotEmpty(t, fkey)

	below := []string{"row-04", "row-05", "row-06", "row-07"}
	children := []string{"row-03/child-00", "row-03/child-01", "row-03/child-02", "row-03/child-03", "row-03/child-04"}

	t.Run("expand by command", func(t *testing.T) {
		assert.False(t, row_expanded(gui, tab, parent_id))
		before := row_stripes(gui, tab, below...)
		assert.Equal(t, map[string]bool{"row-04": false, "row-05": true, "row-06": false, "row-07": true}, before, "top-level rows alternate")

		m := measure_repaints(t, gui, tab, "expand-command", func() { tab.ExpandRow(fkey) })
		assert.True(t, row_expanded(gui, tab, parent_id))
		assert.Equal(t, 0, m.Counters["::update"], "no forced repaint")

		assert.Equal(t, before, row_stripes(gui, tab, below...), "the rows below keep their stripes")
		parent_stripe := row_stripes(gui, tab, parent_id)[parent_id]
		for id, striped := range row_stripes(gui, tab, children...) {
			assert.Equal(t, parent_stripe, striped, "%s shares its parent's stripe", id)
		}
	})

	t.Run("collapse by command", func(t *testing.T) {
		before := row_stripes(gui, tab, below...)
		m := measure_repaints(t, gui, tab, "collapse-command", func() { tab.CollapseRow(fkey) })
		assert.False(t, row_expanded(gui, tab, parent_id))
		assert.Equal(t, 0, m.Counters["::update"], "no forced repaint")
		assert.Equal(t, before, row_stripes(gui, tab, below...), "the rows below keep their stripes")
	})

	t.Run("expand and collapse a row with wide children", func(t *testing.T) {
		var wide_fkey string
		gui.TkSync(func() { wide_fkey = tab.ItemFkeyIndex["row-10"] })
		measure_repaints(t, gui, tab, "expand-wide", func() { tab.ExpandRow(wide_fkey) })
		measure_repaints(t, gui, tab, "collapse-wide", func() { tab.CollapseRow(wide_fkey) })
	})

	t.Run("a menu posted over the table and taken down", func(t *testing.T) {
		var menu string
		var err error
		gui.TkSync(func() {
			menu, err = tk.MainInterp().EvalAsString("lindex [winfo children [. cget -menu]] 0")
		})
		require.NoError(t, err)
		require.NotEmpty(t, menu)
		var x, y int
		gui.TkSync(func() { x, y, err = row_screen_point(tab, fkey) })
		if err != nil {
			t.Skip(err.Error())
		}
		measure_repaints(t, gui, tab, "menu-over-table", func() {
			gui.TkSync(func() { tk.MainInterp().Eval(fmt.Sprintf("%s post %d %d", menu, x, y)) })
			time.Sleep(200 * time.Millisecond)
			gui.TkSync(func() { tk.MainInterp().Eval(menu + " unpost") })
		})
	})

	if _, err := exec.LookPath("xdotool"); err == nil {
		// rows move as rows are added above them, so each pointer test finds its row afresh
		row_point := func(t *testing.T, fkey string) (int, int) {
			var x, y int
			var err error
			gui.TkSync(func() { x, y, err = row_screen_point(tab, fkey) })
			if err != nil {
				t.Skip(err.Error())
			}
			return x, y
		}

		drag_column := func(t *testing.T, label string) {
			var bx, by int
			var err error
			gui.TkSync(func() {
				id := tab.table_widj.Tablelist.Id()
				// just inside the right edge of the first column's label, where the resize handle is
				script := "set l [%s labelpath 0]; list [expr {[winfo rootx $l] + [winfo width $l] - 2}] [expr {[winfo rooty $l] + [winfo height $l] / 2}]"
				out, eval_err := tk.MainInterp().EvalAsString(fmt.Sprintf(script, id))
				err = eval_err
				fmt.Sscanf(out, "%d %d", &bx, &by)
			})
			require.NoError(t, err)
			measure_repaints(t, gui, tab, label, func() {
				xdotool(t, "mousemove", strconv.Itoa(bx), strconv.Itoa(by), "mousedown", "1")
				for step := 1; step <= 20; step++ {
					time.Sleep(10 * time.Millisecond)
					xdotool(t, "mousemove", strconv.Itoa(bx+5*step), strconv.Itoa(by))
				}
				xdotool(t, "mouseup", "1")
				park_pointer(t)
			})
		}

		t.Run("dragging a column border", func(t *testing.T) {
			row_point(t, fkey)
			drag_column(t, "column-drag")
		})

		t.Run("expand by double click", func(t *testing.T) {
			x, y := row_point(t, fkey)
			measure_repaints(t, gui, tab, "expand-click", func() { xdotool_double_click(t, x, y) })
			gui.WaitForIdle()
			assert.True(t, row_expanded(gui, tab, parent_id))
		})

		t.Run("collapse by double click", func(t *testing.T) {
			x, y := row_point(t, fkey)
			measure_repaints(t, gui, tab, "collapse-click", func() { xdotool_double_click(t, x, y) })
			gui.WaitForIdle()
			assert.False(t, row_expanded(gui, tab, parent_id))
		})

		t.Run("dragging a column border of a large table", func(t *testing.T) {
			row_point(t, fkey)
			// thousands of rows, most of them collapsed children, as a browsed directory tree has
			ns := core.MakeNS(flicker_ns_major, "test", "item")
			big := []core.Result{}
			for i := range 300 {
				parent := fmt.Sprintf("big-%03d", i)
				big = append(big, core.MakeResult(ns, flicker_item{Name: parent}, parent))
				for j := range 10 {
					id := fmt.Sprintf("%s/%02d", parent, j)
					child := core.MakeResult(ns, flicker_item{Name: id}, id)
					child.ParentID = parent
					big = append(big, child)
				}
			}
			app.AddReplaceResults(big...).Wait()
			gui.WaitForIdle()
			drag_column(t, "column-drag-large")
		})
	}
}

// --- search filter latency
//
// typing in a tab's search box runs the tab's search fn over every row and hides or
// shows each one, on the Tk thread, so keystrokes queue behind it. these measure that
// work on catalogue-sized tables.

const searchable_ns_major = "searchable"

// returns `count` rows in the `searchable` namespace, their names spread over the
// alphabet so a needle of one letter matches a fraction of them.
func searchable_results(count int) []core.Result {
	ns := core.MakeNS(searchable_ns_major, "test", "item")
	result_list := []core.Result{}
	for i := range count {
		name := fmt.Sprintf("%cddon-%05d", 'a'+rune(i%26), i)
		result_list = append(result_list, core.MakeResult(ns, flicker_item{Name: name}, name))
	}
	return result_list
}

// a search fn matching rows whose name contains `input`, as the catalogue search does.
func searchable_filter(input string, row map[string]string) bool {
	needle := strings.ToLower(strings.TrimSpace(input))
	return needle == "" || strings.Contains(strings.ToLower(row[core.ITEM_FIELD_NAME]), needle)
}

// times `ApplySearchFilter` for `text` on `tab`, logging the time and the tablelist
// procs it ran.
func measure_search(t *testing.T, gui *GUIUI, tab *GUITab, label string, text string) time.Duration {
	t.Helper()
	var err error
	gui.TkSync(func() {
		err = install_tk_counters()
		if err == nil {
			err = reset_tk_counters()
		}
	})
	require.NoError(t, err)

	var took time.Duration
	gui.TkSync(func() {
		started := time.Now()
		tab.ApplySearchFilter(text)
		took = time.Since(started)
	})

	var counts map[string]int
	var millis map[string]float64
	gui.TkSync(func() {
		counts, err = read_tk_counters()
		if err == nil {
			millis, err = read_tk_millis()
		}
	})
	require.NoError(t, err)
	t.Logf("%s: filter %q took %dms", label, text, took.Milliseconds())
	t.Logf("%s: %s", label, format_tk_counters(counts, millis))
	return took
}

// measures the search filter on tables of increasing size.
// a sub-test of `Test_gui_live`: only one Tk interpreter can run per process.
func search_latency_subtests(t *testing.T, app *core.App, gui *GUIUI) {
	gui.AddTab("searchable", func(r core.Result) bool { return r.NS.Major == searchable_ns_major })
	tab := gui.GetTab("searchable")
	tab.SetColumnAttrs([]UIColumn{{Title: core.ITEM_FIELD_NAME}, {Title: "source"}, {Title: "installed-version"}})
	tab.SetSearchFilter(searchable_filter)
	gui.SetActiveTab("searchable")
	gui.ApplyTablelistStyling()
	gui.TkSync(func() { gui.offscreen_tables() })

	rows := 0
	for _, size := range []int{2000, 5000} { // the strongbox catalogue has about 4300 rows
		app.AddReplaceResults(searchable_results(size)[rows:]...).Wait()
		gui.WaitForIdle()
		rows = size
		label := fmt.Sprintf("search-%d-rows", size)
		t.Run(label, func(t *testing.T) {
			measure_search(t, gui, tab, label, "zzz")  // hides every row
			measure_search(t, gui, tab, label, "a")    // shows one row in 26
			measure_search(t, gui, tab, label, "ddon") // shows every row
			measure_search(t, gui, tab, label, "")     // shows every row, already shown
		})
	}
}
