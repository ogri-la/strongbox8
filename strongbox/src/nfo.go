package strongbox

import (
	"bw/core"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// --- NFO
// strongbox curated data about an addon or group of addons.
// created when an addon is installed through strongbox.
// derived from toc, catalogue, per-addon user preferences, etc.
// lives in .strongbox.json files in the addon's root.

// we *could* create these upon first detecting an addon so that nfo data is *always* available,
// but first time users would be left with .strongbox files hanging around.
// a solution might be to not store these per-directory and instead keep a central database.
// should that happen we still may not have enough data to create a valid nfo file as we need
// a catalogue match.

type NFO struct {
	InstalledVersion     string      `json:"installed-version,omitempty"`
	Name                 string      `json:"name,omitempty"`
	GroupID              string      `json:"group-id"`
	Primary              bool        `json:"primary?"` // TODO: rename IsPrimary
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

// returns `true` when the nfo is considered 'empty'.
// very basic validation check
func (n *NFO) IsEmpty() bool {
	empty_nfo := NFO{}
	if n == &empty_nfo {
		return true
	}

	// all NFO data *must* have a non-empty group-id
	if n.GroupID == "" {
		return true
	}

	// todo: stop here. need proper validation

	return false
}

// "given an installation directory and the directory name of an addon, return the absolute path to the nfo file."
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

func version_controlled(addon_dir PathToAddon) bool {
	vcs, err := version_control(addon_dir)
	if err != nil {
		return false
	}
	return vcs != ""
}

var ErrNFODNE = errors.New("nfo data file does not exist")

// "reads the nfo file at the given `path` with basic transformations.
// an error is returned if the data cannot be loaded or the data is invalid.
func read_nfo_file(addon_dir PathToAddon) ([]NFO, error) {
	empty_data := []NFO{}

	if strings.HasSuffix(addon_dir, NFO_FILENAME) {
		slog.Error("given addon dir is suffixed with nfo file and looks like a _file_", "addon-dir", addon_dir)
		panic("programming error")
	}

	path := nfo_path(addon_dir)
	if !core.FileExists(path) {
		return empty_data, ErrNFODNE
	}

	data := NFO{}
	nfo_list := []NFO{}

	nfo_bytes, err := os.ReadFile(path)
	if err != nil {
		return empty_data, err
	}

	err = json.Unmarshal(nfo_bytes, &data)
	if err != nil {
		err2 := json.Unmarshal(nfo_bytes, &nfo_list)
		if err2 != nil {
			return empty_data, err2
		}
	} else {
		nfo_list = append(nfo_list, data)
	}

	for _, nfo := range nfo_list {
		// add a SourceMapList if one isn't present
		// new in v8: previously only applied to top-level nfo
		if nfo.Source != "" && len(nfo.SourceMapList) == 0 {
			sm := SourceMap{Source: nfo.Source, SourceID: nfo.SourceID}
			nfo.SourceMapList = append(nfo.SourceMapList, sm)
		}

		// implicitly ignore addon when VCS directory present
		vcs := version_controlled(addon_dir)
		if nfo.Ignored != nil && err == nil && vcs {
			slog.Warn("addon directory contains a .git/.hg/.svn folder, ignoring", "addon-dir", addon_dir)
			ignored := true
			nfo.Ignored = &ignored
		}
	}

	return nfo_list, nil
}

// "parses the contents of the .nfo file and checks if addon should be ignored or not"
// failure to load the json results in the file being deleted.
// failure to validate the json data results in the file being deleted."
/*
func read_nfo(addon_dir PathToAddon) ([]NFO, error) {
	empty_response := []NFO{}
	nfo_data_list, err := read_nfo_file(addon_dir)
	if err != nil {
		// todo: previous behaviour was to delete file if it contains bad/invalid data
		return empty_response, fmt.Errorf("failed to read NFO data: %w", err)
	}
	if len(nfo_data_list) == 0 {
		slog.Warn("NFO data was empty", "path", addon_dir)
	}
	return nfo_data_list, nil
}
*/

func nfo_ignored(nfo NFO) bool {
	if nfo.Ignored == nil {
		return false
	}
	return *nfo.Ignored
}

// the last nfo is always the one to use
func pick_nfo(nfo_list []NFO) (NFO, error) {
	if len(nfo_list) == 0 {
		return NFO{}, fmt.Errorf("no nfo to pick")
	}
	return nfo_list[len(nfo_list)-1], nil
}

// returns `true` if multiple sets of nfo data exist in file.
// slightly different in 8.0, reading nfo data will always return a list
func is_mutual_dependency(nfo_data []NFO) bool {
	return len(nfo_data) > 1
}

// reads nfo data at `addon_path`,
// returns `nfo_data_list` excluding nfo that matches `group_id`.
// todo: needs some attention.
// * why does it read from disk but then not immediately write results back to disk?
// * why does it write empty nfo data to a file when deleting from a single nfo?
// * should it just delete the whole file?
// * should we refuse to remove the nfo?
// * reading empty nfo from a nfo file was an error until I just commented it out...
func rm_nfo(addon_path PathToAddon, group_id string) ([]NFO, error) {
	empty_response := []NFO{}
	nfo_data_list, err := read_nfo_file(addon_path)
	if err != nil {
		// cannot remove nfo data for whatever reason
		return empty_response, fmt.Errorf("failed to remove nfo data: %w", err)
	}
	updated_nfo := []NFO{}
	for _, nfo := range nfo_data_list {
		if nfo.GroupID != group_id {
			updated_nfo = append(updated_nfo, nfo)
		}
	}
	return updated_nfo, nil
}

// adds the given nfo data to the end of the list (most recent) and removes it from any other position in the list.
// if the nfo doesn't exist, it will be created.
func add_nfo(addon_path PathToAddon, nfo NFO) ([]NFO, string, error) {
	empty_response := []NFO{}
	extant_nfo_list, err := read_nfo_file(addon_path)
	if err != nil {
		if errors.Is(err, ErrNFODNE) {
			// we can recover!
			return []NFO{nfo}, "", nil
		}
		return empty_response, "", err
	}

	// remove any matching nfo
	new_nfo := []NFO{}
	for _, extant_nfo := range extant_nfo_list {
		if extant_nfo.GroupID != nfo.GroupID {
			new_nfo = append(new_nfo, extant_nfo)
		}
	}

	user_msg := ""
	if len(extant_nfo_list) > 1 {
		target, _ := pick_nfo(extant_nfo_list)
		nom := func(nfo NFO) string {
			if nfo.Name != "" {
				return nfo.Name
			}
			return nfo.GroupID
		}
		version := func(nfo NFO) string {
			if nfo.InstalledVersion != "" {
				return fmt.Sprintf(` (%s)`, nfo.InstalledVersion)
			}
			return ""
		}

		// catalogue overwriting catalogue
		// '"Healbot Continued" (9.2.0.12) replaced dir 'HealBot/' of addon "Healbot Continued" (9.2.0.7)'

		// catalogue overwriting file install
		// '"Healbot Continued" (9.2.0.12) replaced dir 'HealBot/' of addon "healbot-continued-abcdef12345'

		// file install overwriting catalogue
		// '"healbot-continued-abcdef12345' replaced dir 'HealBot/' of addon "Healbot Continued" (9.2.0.12)'
		user_msg = fmt.Sprintf(`"%s"%s replaced directory "%s" of addon "%s"%s`,
			nom(nfo), version(nfo),
			filepath.Base(addon_path),
			nom(target), version(nfo))
	}

	// append new nfo to end
	new_nfo = append(new_nfo, nfo)
	return new_nfo, user_msg, nil
}

// given an installation directory and an addon, select the neccessary bits (`prune`) and write them to a nfo file
func write_nfo(addon_path PathToAddon, nfo_data_list []NFO) error {
	if len(nfo_data_list) == 0 {
		return fmt.Errorf("refusing to write nfo data to disk: nfo data is empty")
	}

	for _, nfo := range nfo_data_list {
		if nfo.IsEmpty() {
			return fmt.Errorf("refusing to write nfo data to disk: nfo data list contains empty nfo")
		}
	}

	// todo: more data validation
	valid := true
	if !valid {
		err := errors.New("some error")
		return fmt.Errorf("refusing to write nfo data to disk: nfo data is invalid: %w", err)
	}

	path := nfo_path(addon_path)

	bytes, err := json.Marshal(nfo_data_list)
	if err != nil {
		return fmt.Errorf("failed to marshal nfo data: %w", err)
	}

	err = core.Spit(path, bytes)
	if err != nil {
		return fmt.Errorf("failed to write nfo data to disk: %w", err)
	}

	return nil
}

// extract a set of values from the given Addon data to preserve on disk.
// the nfo data is typically derived and written to disk just after the addon has been unzipped.
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
		nfo.InstalledGameTrackID = a.AddonsDir.GameTrackID

		// normalised name.
		// once used to match to online addon (we now use source+source-id)
		nfo.Name = a.Name

		// record the origin and it's ID so we can switch back to it later if other sources present themselves.
		nfo.SourceMapList = []SourceMap{
			{Source: a.Source, SourceID: FlexString(a.SourceID)},
		}
	}

	return nfo
}

func nfo_unpin(nfo NFO) NFO {
	nfo.PinnedVersion = ""
	return nfo
}

func nfo_pinned(nfo NFO) bool {
	return nfo.PinnedVersion != ""
}
