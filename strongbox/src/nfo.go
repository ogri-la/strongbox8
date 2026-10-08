package strongbox

import (
	"bw/core"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// --- NFO
// strongbox curated data about an addon or group of addons.
// created when an addon is installed through strongbox.
// derived from toc, catalogue, per-addon user preferences, etc.
// lives in .strongbox.json files in the addon's root.
//
// the file is written so strongbox 7 can read it: a single JSON object when one addon owns
// the directory, a JSON array only when several addons share it.

type NFO struct {
	InstalledVersion     string      `json:"installed-version,omitempty"`
	Name                 string      `json:"name,omitempty"`
	GroupID              string      `json:"group-id"`
	Primary              bool        `json:"primary?"`
	Source               Source      `json:"source,omitempty"`
	InstalledGameTrackID GameTrackID `json:"installed-game-track,omitempty"`
	SourceID             FlexString  `json:"source-id,omitempty"` // ints become strings, new in v8
	SourceMapList        []SourceMap `json:"source-map-list,omitempty"`
	Ignored              *bool       `json:"ignore?,omitempty"` // null means the user hasn't explicitly ignored or explicitly un-ignored it
	PinnedVersion        string      `json:"pinned-version,omitempty"`
}

func NewNFO() NFO {
	return NFO{
		SourceMapList: []SourceMap{},
		Ignored:       nil,
	}
}

// returns `true` when the nfo has no group ID.
// every nfo describing an addon has one, so an nfo without one is unusable.
func (n NFO) IsEmpty() bool {
	return n.GroupID == ""
}

// the contents of an nfo file: a sum type, exactly one of its two fields is set.
// a stack, not a set: the order of addons sharing a directory matters, the last one
// installed describes it.
type NFOFile struct {
	Stack      []NFO // the addons owning the directory, oldest first
	IgnoreFlag *bool // an ignore-only file, `{"ignore?": true}`, for a directory no addon owns
}

// returns `true` when the file holds nothing and should not exist.
func (f NFOFile) IsEmpty() bool {
	return len(f.Stack) == 0 && f.IgnoreFlag == nil
}

// returns the nfo describing the directory, the most recently installed, and `true`, or
// an empty nfo and `false` when no addon owns the directory.
func (f NFOFile) Top() (NFO, bool) {
	if len(f.Stack) == 0 {
		return NFO{}, false
	}
	return f.Stack[len(f.Stack)-1], true
}

// returns the explicit ignore flag: the ignore-only file's, else the top nfo's, else nil
// when the user has neither ignored nor un-ignored the directory.
func (f NFOFile) Ignored() *bool {
	if f.IgnoreFlag != nil {
		return f.IgnoreFlag
	}
	if top, ok := f.Top(); ok {
		return top.Ignored
	}
	return nil
}

// returns `true` when more than one addon shares the directory.
func (f NFOFile) IsMutualDependency() bool {
	return len(f.Stack) > 1
}

// returns the path to the nfo file within the given `addon_dir`.
func nfo_path(addon_dir PathToAddon) string {
	return filepath.Join(addon_dir, NFO_FILENAME) // "/path/to/addon-dir/Addon/.strongbox.json
}

// returns the VCS directory found if given path contains a VCS directory,
// otherwise an empty string.
func version_control(addon_dir PathToAddon) (string, error) {
	path_list, err := core.DirList(addon_dir)
	if err != nil {
		return "", err
	}
	for _, path := range path_list {
		dirname := filepath.Base(path)
		if VCS_DIR_SET.Contains(dirname) {
			return dirname, nil
		}
	}
	return "", nil
}

// returns `true` when the given `addon_dir` holds a VCS directory.
// an unreadable directory is treated as `false`.
func version_controlled(addon_dir PathToAddon) bool {
	vcs, err := version_control(addon_dir)
	if err != nil {
		return false
	}
	return vcs != ""
}

var ErrNFODNE = errors.New("nfo data file does not exist")
var ErrNFOInvalid = errors.New("nfo data is invalid")

// returns an error when `nfo`, read from disk, cannot describe an addon.
// reading is more forgiving than writing: an addon installed long ago from a host that
// has since gone, such as curseforge, still groups its directories and keeps its flags.
func valid_nfo_for_read(nfo NFO) error {
	if strings.TrimSpace(nfo.GroupID) == "" {
		return errors.New("no group ID")
	}
	if nfo.InstalledGameTrackID != "" && !ALL_GAME_TRACKS.Contains(nfo.InstalledGameTrackID) {
		return fmt.Errorf("unknown installed game track: %s", nfo.InstalledGameTrackID)
	}
	return nil
}

// returns `nfo` with a source map list, built from its source, when it has none.
// strongbox 7 'v1' nfo data predates source map lists.
func nfo_with_source_map_list(nfo NFO) NFO {
	if nfo.Source != "" && len(nfo.SourceMapList) == 0 {
		nfo.SourceMapList = []SourceMap{{Source: nfo.Source, SourceID: nfo.SourceID}}
	}
	return nfo
}

// returns the nfo file described by the file contents `b`.
// returns an error wrapping `ErrNFOInvalid` when `b` matches none of the nfo shapes: an
// array of nfo objects, a single nfo object, or an object holding only `ignore?`.
func parse_nfo_file(b []byte) (NFOFile, error) {
	b = bytes.TrimSpace(b)
	invalid := func(reason string) (NFOFile, error) {
		return NFOFile{}, fmt.Errorf("%w: %s", ErrNFOInvalid, reason)
	}

	if len(b) > 0 && b[0] == '[' {
		nfo_list := []NFO{}
		if err := json.Unmarshal(b, &nfo_list); err != nil {
			return invalid(err.Error())
		}
		if len(nfo_list) == 0 {
			return invalid("empty list")
		}
		for i, nfo := range nfo_list {
			if err := valid_nfo_for_read(nfo); err != nil {
				return invalid(fmt.Sprintf("entry %d: %s", i+1, err))
			}
			nfo_list[i] = nfo_with_source_map_list(nfo)
		}
		return NFOFile{Stack: nfo_list}, nil
	}

	key_map := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &key_map); err != nil {
		return invalid(err.Error())
	}

	// an ignore-only file
	if raw, present := key_map["ignore?"]; present && len(key_map) == 1 {
		flag := false
		if err := json.Unmarshal(raw, &flag); err != nil {
			return invalid("'ignore?' is not a boolean")
		}
		return NFOFile{IgnoreFlag: &flag}, nil
	}

	nfo := NFO{}
	if err := json.Unmarshal(b, &nfo); err != nil {
		return invalid(err.Error())
	}
	if err := valid_nfo_for_read(nfo); err != nil {
		return invalid(err.Error())
	}
	return NFOFile{Stack: []NFO{nfo_with_source_map_list(nfo)}}, nil
}

// reads the nfo file in the given `addon_dir`.
// returns `ErrNFODNE` when the file does not exist, and an error wrapping `ErrNFOInvalid`
// when its contents are not nfo data. an invalid file is never deleted.
// panics if `addon_dir` is the path of the nfo file rather than the directory holding it.
func read_nfo_file(addon_dir PathToAddon) (NFOFile, error) {
	if strings.HasSuffix(addon_dir, NFO_FILENAME) {
		slog.Error("given addon dir is suffixed with nfo file and looks like a _file_", "addon-dir", addon_dir)
		panic("programming error")
	}

	path := nfo_path(addon_dir)
	if !core.FileExists(path) {
		return NFOFile{}, ErrNFODNE
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return NFOFile{}, err
	}
	f, err := parse_nfo_file(b)
	if err != nil {
		return NFOFile{}, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// returns the bytes to write for the nfo file `f`: a single object for one owner, an array
// for several, an object holding only `ignore?` for an ignore-only file.
// returns an error when any nfo is invalid, or `f` is empty.
func marshal_nfo_file(f NFOFile) ([]byte, error) {
	if f.IsEmpty() {
		return nil, errors.New("nfo file is empty")
	}
	if len(f.Stack) == 0 {
		return json.Marshal(map[string]bool{"ignore?": *f.IgnoreFlag})
	}
	for _, nfo := range f.Stack {
		if issues := nfo.Valid(); issues != nil {
			return nil, fmt.Errorf("%w: %s", ErrNFOInvalid, format_zog_issues(issues))
		}
	}
	if len(f.Stack) == 1 {
		return json.Marshal(f.Stack[0])
	}
	return json.Marshal(f.Stack)
}

// writes the nfo file `f` into `addon_dir`, atomically, or deletes the nfo file when `f`
// is empty.
// invalid nfo data is never written: it is a program error, logged at ERROR.
func write_nfo_file(addon_dir PathToAddon, f NFOFile) error {
	path := nfo_path(addon_dir)
	if f.IsEmpty() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove empty nfo file: %w", err)
		}
		return nil
	}
	b, err := marshal_nfo_file(f)
	if err != nil {
		slog.Error("refusing to write invalid nfo data, this is a program error, please report it", "path", path, "error", err)
		return fmt.Errorf("refusing to write nfo data: %w", err)
	}
	return write_atomic(path, b)
}

// --- pure edits to nfo files

// returns `f` with `nfo` installed on top, replacing any entry with the same group ID.
// an ignore-only flag is dropped, the new nfo carries the addon's flags.
// the returned message tells the user when another addon's directory was taken over,
// empty otherwise.
func nfo_file_add(f NFOFile, nfo NFO, dir_name string) (NFOFile, string) {
	user_msg := ""
	if top, ok := f.Top(); ok && top.GroupID != nfo.GroupID {
		nom := func(n NFO) string {
			if n.Name != "" {
				return n.Name
			}
			return n.GroupID
		}
		version := func(n NFO) string {
			if n.InstalledVersion != "" {
				return fmt.Sprintf(" (%s)", n.InstalledVersion)
			}
			return ""
		}
		// '"everyotheraddon" (5.6.7) replaced directory "EveryAddon-BundledAddon" of addon "everyaddon" (0.1.2)'
		user_msg = fmt.Sprintf(`"%s"%s replaced directory "%s" of addon "%s"%s`,
			nom(nfo), version(nfo), dir_name, nom(top), version(top))
	}

	stack := slices.DeleteFunc(slices.Clone(f.Stack), func(n NFO) bool { return n.GroupID == nfo.GroupID })
	return NFOFile{Stack: append(stack, nfo)}, user_msg
}

// returns `f` without the entry for `group_id`.
func nfo_file_rm(f NFOFile, group_id string) NFOFile {
	stack := slices.DeleteFunc(slices.Clone(f.Stack), func(n NFO) bool { return n.GroupID == group_id })
	if len(stack) == 0 {
		stack = nil
	}
	return NFOFile{Stack: stack, IgnoreFlag: f.IgnoreFlag}
}

// returns `f` with the top nfo changed by `fn`.
// `f` is returned unchanged when no addon owns the directory.
func nfo_file_update_top(f NFOFile, fn func(NFO) NFO) NFOFile {
	if len(f.Stack) == 0 {
		return f
	}
	stack := slices.Clone(f.Stack)
	stack[len(stack)-1] = fn(stack[len(stack)-1])
	return NFOFile{Stack: stack}
}

// returns `f` with the directory explicitly ignored.
// a directory no addon owns gets an ignore-only file.
// `implicit` is whether the directory would be ignored anyway, being under version
// control or holding an unrendered version: an explicit 'not ignored' flag on such a
// directory is removed rather than flipped, so it reverts to being implicitly ignored.
// clj: `nfo/ignore!`, `update-nfo-data-with-ignore-flags`
func nfo_file_ignore(f NFOFile, implicit bool) NFOFile {
	if len(f.Stack) == 0 {
		if implicit && f.IgnoreFlag != nil && !*f.IgnoreFlag {
			return NFOFile{}
		}
		return NFOFile{IgnoreFlag: new(true)}
	}
	return nfo_file_update_top(f, func(n NFO) NFO {
		n.Ignored = new(true)
		return n
	})
}

// returns `f` with the directory no longer ignored.
// when the directory would be ignored anyway, `implicit`, an explicit 'not ignored' flag
// is written, otherwise the flag is removed. an ignore-only file left with no flag is
// empty and should be deleted.
// clj: `addon/clear-ignore!`, `nfo/stop-ignoring!`, `nfo/clear-ignore!`
func nfo_file_stop_ignoring(f NFOFile, implicit bool) NFOFile {
	if len(f.Stack) == 0 {
		if implicit {
			return NFOFile{IgnoreFlag: new(false)}
		}
		return NFOFile{}
	}
	return nfo_file_update_top(f, func(n NFO) NFO {
		if implicit {
			n.Ignored = new(false)
		} else {
			n.Ignored = nil
		}
		return n
	})
}

// returns `f` with the top nfo pinned to `version`, or unpinned when `version` is empty.
// clj: `nfo/pin!`, `nfo/unpin!`
func nfo_file_pin(f NFOFile, version string) NFOFile {
	return nfo_file_update_top(f, func(n NFO) NFO {
		n.PinnedVersion = version
		return n
	})
}

// returns the nfo to preserve on disk for the given addon `a`, typically written just
// after the addon has been unzipped.
// `is_primary` marks this directory as the addon's main one when an addon spans several.
// a complete nfo needs a source, a source ID and a source update. without all three the
// result is a 'just grouped' nfo, carrying only enough to group related directories.
// panics if `a` has no nfo or no group ID.
func derive_nfo(a Addon, is_primary bool) NFO {
	if a.NFO == nil || a.NFO.GroupID == "" {
		slog.Error("`derive_nfo` *must* be given an Addon with a NFO with a GroupID as a minimum")
		panic("programming error")
	}

	nfo := NFO{}

	// groups all of an addon's directories together
	nfo.GroupID = a.NFO.GroupID

	// if addon is one of multiple addons, is this addon considered the 'primary' one?
	nfo.Primary = is_primary

	// users can set this in the nfo file manually or
	// it can be drived later in the process by examining the addon's toc file and/or subdirs, or
	// it may be present when upgrading an existing nfo file and should be preserved
	nfo.Ignored = a.Ignored

	nfo.PinnedVersion = a.PinnedVersion

	if a.Source == "" || a.SourceID == "" || a.SourceUpdate == nil {
		// any one of these conditions means we can't generate a complete NFO file - we're missing vital data.
		// our next best bet is a 'just grouped' nfo file that contains just enough information to group related addons together.

	} else {
		// where the addon came from and how it was identified
		nfo.Source = a.Source
		nfo.SourceID = FlexString(a.SourceID)

		nfo.InstalledVersion = a.SourceUpdate.Version

		// used to filter available updates.
		// also, knowing the regime the addon was installed under allows us to export and later re-import the correct version.
		nfo.InstalledGameTrackID = installed_game_track(a)

		// normalised name.
		// once used to match to online addon (we now use source+source-id)
		nfo.Name = a.Name

		// record the origin and it's ID so we can switch back to it later if other sources present themselves.
		nfo.SourceMapList = source_map_list_for_nfo(a)
	}

	return nfo
}

// returns the game track an addon is being installed under: the addons dir's game track
// when the chosen update supports it, otherwise the update's most preferred game track,
// which happens in a relaxed addons dir.
func installed_game_track(a Addon) GameTrackID {
	if a.SourceUpdate == nil || a.SourceUpdate.GameTrackIDSet == nil || a.SourceUpdate.GameTrackIDSet.Contains(a.AddonsDir.GameTrackID) {
		return a.AddonsDir.GameTrackID
	}
	for _, gt := range GAMETRACK_PREF_MAP[a.AddonsDir.GameTrackID] {
		if a.SourceUpdate.GameTrackIDSet.Contains(gt) {
			return gt
		}
	}
	return a.AddonsDir.GameTrackID
}

// returns the source map list to record for `a`: its current source first, then any other
// sources it is known by, without duplicates or dead hosts.
func source_map_list_for_nfo(a Addon) []SourceMap {
	current := SourceMap{Source: a.Source, SourceID: FlexString(a.SourceID)}
	sml := []SourceMap{current}
	for _, sm := range a.SourceMapList {
		if sm != current && SUPPORTED_HOSTS.Contains(sm.Source) && !slices.Contains(sml, sm) {
			sml = append(sml, sm)
		}
	}
	return sml
}

// returns `true` when `nfo` has a pinned version.
func nfo_pinned(nfo NFO) bool {
	return nfo.PinnedVersion != ""
}
