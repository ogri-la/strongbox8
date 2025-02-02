package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
)

/*
   addon data loading, merging, wrangling.
*/

// --- Source Map
// used to know where an addon came from and other locations it may live.

type SourceMap struct {
	Source   Source     `json:"source"`
	SourceID FlexString `json:"source-id"`
}

// --- Source Updates
// extra data a source (wowinterface, github, etc) provides about an addon.

// todo: rename 'release' or similar? release.type: 'lib', 'nolib'. release.stability: 'stable', 'beta', 'alpha', etc.
type SourceUpdate struct {
	//Type string // lib, nolib
	//Stability string // beta, alpha, etc
	Version          string `json:"version"`
	DownloadURL      string
	GameTrackID      GameTrackID
	InterfaceVersion int
}

// --- InstalledAddon
// the collection of .toc data and .strongbox.json data for an addon directory.

type InstalledAddon struct {
	URL string

	// an addon may have many .toc files, keyed by game track.
	// the toc data eventually used is determined by the selected addon dir's game track.
	TOCMap map[GameTrackID]TOC // required, >= 1

	// an installed addon has zero or one `strongbox.json` 'nfo' files,
	// however that nfo file may contain a list of data when mutual dependencies are involved.
	// new in v8, all nfo data is now a list
	NFOList []NFO // optional
}

var _ core.ItemInfo = (*InstalledAddon)(nil)

func (a InstalledAddon) SomeTOC() (TOC, error) {
	var some_toc TOC
	if len(a.TOCMap) < 1 {
		return some_toc, errors.New("InstalledAddon has an empty tocmap")
	}
	for _, toc := range a.TOCMap {
		some_toc = toc
		break
	}
	return some_toc, nil
}

// priority: parent, toc
func (a InstalledAddon) Attr(field string) string {
	if len(a.TOCMap) < 1 {
		panic("programming error. empty `InstalledAddon` accessed `.Attr`")
	}

	some_toc := TOC{}
	for _, val := range a.TOCMap {
		some_toc = val
		break
	}

	//has_nfo := len(a.NFOList) > 0

	switch field {
	case "id":
		return core.UniqueID()

	case "name":
		return some_toc.DirName

	}
	panic(fmt.Sprintf("programming error, unknown field: %s", field))
}

// an InstalledAddon has 1+ .toc files that can be loaded immediately.
func (ia InstalledAddon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (ia InstalledAddon) ItemKeys() []string {
	return []string{
		"source",
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_URL,
	}
}

func (ia InstalledAddon) ItemMap() map[string]string {
	row := map[string]string{
		"source":             "",
		core.ITEM_FIELD_NAME: ia.Attr(core.ITEM_FIELD_NAME),
		core.ITEM_FIELD_URL:  ia.URL,
	}
	return row
}

func (ia InstalledAddon) ItemChildren(_ *core.App) []core.Result {
	// todo: what would be natural children for an installed addon? the .toc files? the file listing?
	foo := []core.Result{}
	for _, toc := range ia.TOCMap {
		foo = append(foo, core.Result{ID: toc.Attr("id"), NS: NS_TOC, Item: toc})
	}
	return foo
}

// --- Addon

// an 'addon' represents one or a group of installed addons.
// the group has a representative 'primary' addon,
// representative TOC data according to the selected game track of the addon dir the addon lives in,
// representative NFO data according to whether the addon is overriding or being overridden by other addons.
type Addon struct {
	AddonGroup []InstalledAddon // required >= 1
	Primary    *InstalledAddon  // required

	TOC            *TOC            // required, Addon.Primary.TOC[$gametrack]
	NFO            *NFO            // optional, Addon.Primary.NFO[-1]
	CatalogueAddon *CatalogueAddon // optional, the catalogue match

	Ignored bool // required, `Addon.Primary.NFO[-1].Ignored` or `Addon.Primary.TOC[$gametrack].Ignored`

	SourceUpdateList []SourceUpdate
	SourceUpdate     *SourceUpdate // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
}

var _ core.ItemInfo = (*Addon)(nil)

func (a Addon) ItemKeys() []string {
	return []string{
		"source",
		core.ITEM_FIELD_NAME,
		core.ITEM_FIELD_DESC,
		core.ITEM_FIELD_URL,
		"tags",
		"created",
		"updated",
		"size",
		"installed-version",
		"available-version",
		"game-version",
	}
}

func (a Addon) ItemMap() map[string]string {
	return map[string]string{
		"source":             a.Attr("source"),
		core.ITEM_FIELD_NAME: a.Attr("dirname"),
		core.ITEM_FIELD_DESC: a.Attr("description"),
		core.ITEM_FIELD_URL:  a.Attr("url"),
		"tags":               "foo,bar,baz",
		"created":            "[todo]",
		"updated":            a.Attr("updated"),
		"size":               "0",
		"installed-version":  a.Attr("installed-version"),
		"available-version":  a.Attr("available-version"),
		"game-version":       a.Attr("game-version"),
	}
}

// an Addon may be grouping multiple InstalledAddons.
// if so, they can be loaded immediately.
func (a Addon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (a Addon) ItemChildren(_ *core.App) []core.Result {
	children := []core.Result{}
	for _, installed_addon := range a.AddonGroup {
		children = append(children, core.NewResult(NS_INSTALLED_ADDON, installed_addon, installed_addon.Attr("id")))
	}
	return children
}

// attribute picked for an addon.
// order of precedence (typically) is: source_updates (tbd), catalogue_addon, nfo, toc
func (a Addon) Attr(field string) string {
	if a.TOC == nil {
		slog.Error("addon TOC is nil!", "addon", a, "field", field)
		panic("nil toc")
	}
	// has_toc := a.TOC != nil // always true, must be strict.
	has_nfo := a.NFO != nil
	has_match := a.CatalogueAddon != nil
	has_updates := false
	//has_parent := false // is this an addon within a group?
	switch field {

	// unique identifier for this addon
	case "id":
		return core.UniqueID()

	// raw title. does anything use this?
	case "title":
		if has_match {
			return a.CatalogueAddon.Label // "AdiBags"
		}
		return a.TOC.Title // may be empty, use "label" for a safer "title"

	// human friendly addon title
	case "label":
		if has_match {
			return a.CatalogueAddon.Label // "AdiBags"
		}
		return a.TOC.Label // "AdiBags", "Group Title *"

	// normalised title
	case "name":
		if has_match {
			return a.CatalogueAddon.Name // "adibags"
		}
		if has_nfo {
			return a.NFO.Name
		}

		return a.TOC.Name

	case "description":
		if has_match {
			return a.CatalogueAddon.Description
		}
		return a.TOC.Notes

	case core.ITEM_FIELD_URL:
		if has_match {
			return a.CatalogueAddon.URL
		}
		return ""

	case "dirname":
		return a.TOC.DirName

	case "interface-version": // 100105, 30402
		return core.IntToString(a.TOC.InterfaceVersion)

	case "game-version": // "10.1.5", "3.4.2"
		v, err := InterfaceVersionToGameVersion(a.TOC.InterfaceVersion)
		if err == nil {
			return v
		}

	case "installed-version": // v1.2.3, foo-bar.zip.v1, 10.12.0v1.4.2, 12312312312
		if has_nfo {
			return a.NFO.InstalledVersion
		}
		return a.TOC.InstalledVersion

	case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
		if has_updates {
			return a.SourceUpdate.Version
		}
		return a.TOC.InstalledVersion

	case "source":
		if has_match {
			return a.CatalogueAddon.Source
		}
		if has_nfo {
			return a.NFO.Source
		}

		// can we do anything with a.TOC.SourceMapList if one exists?
		// I guess those are just *potential* sources rather than the *actual* source.

	case "source-id":
		if has_match {
			return string(a.CatalogueAddon.SourceID)
		}
		if has_nfo {
			return string(a.NFO.SourceID)
		}

	case "updated":
		/* todo: use the latest update value we can find?
		if has_updates {
			// ...
		}
		*/

		if has_match {
			return a.CatalogueAddon.UpdatedDate
		}

	default:
		panic(fmt.Sprintf("programming error, unknown field: %s", field))
	}

	return ""
}

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	installed_addon := InstalledAddon{
		URL: "file://" + addon_dir,
	}
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		slog.Error("error parsing toc file", "error", err)
		return installed_addon, err
	}
	installed_addon.TOCMap = toc_map

	nfo_list := ReadNFO(addon_dir)
	installed_addon.NFOList = nfo_list
	return installed_addon, nil

}

// --- public

// addon.clj/load-all-installed-addons
// toc.clj/parse-addon-toc-guard
// reads the toc and nfo data from *all* addons in the given `addon_dir`,
// groups them and returns the result.
func LoadAllInstalledAddons(addons_dir AddonsDir) ([]Addon, error) {
	empty_addon_list := []Addon{}
	dir_list, err := core.DirList(addons_dir.Path)
	if err != nil {
		return empty_addon_list, err
	}

	installed_addon_list := []InstalledAddon{}
	for _, full_path := range dir_list {
		if BlizzardAddon(full_path) {
			continue
		}
		addon, err := load_installed_addon(full_path)
		if err != nil {
			slog.Error("failed to load addon", "error", err)
			continue
		}
		installed_addon_list = append(installed_addon_list, addon)
	}

	// group installed addons // addon.clj/group-addons
	nogroup := ""

	// an installed addon may be part of a bundle.
	// we can only group addons once they've all been loaded and have the group-ids
	installed_addon_groups := core.GroupBy(installed_addon_list, func(installed_addon InstalledAddon) string {
		if len(installed_addon.NFOList) == 0 {
			// no nfo data.
			// it either wasn't found or was bad and ignored.
			return nogroup
		}
		nfo, _ := PickNFO(installed_addon.NFOList)
		return nfo.GroupID
	})

	addon_list := []Addon{}

	// for each group of `InstalledAddon`, create an `Addon` and select the primary
	for group_id, installed_addon_group := range installed_addon_groups {
		installed_addon_group := installed_addon_group

		if group_id == "" {
			// the no-group group.
			// *valid* NFO data requires a GroupID, so treat them all as installed but not strongbox-installed.
			for _, installed_addon := range installed_addon_group {
				installed_addon := installed_addon
				addon_list = append(addon_list, Addon{
					AddonGroup: []InstalledAddon{installed_addon},
					Primary:    &installed_addon,
					// TOC: is set later when we know the game track
					// NFO: not found/bad data/invalid data
					Ignored: false,
				})
			}
		} else {
			// regular group

			if len(installed_addon_group) == 1 {
				// perfect case, no grouping.
				new_addon_group := []InstalledAddon{installed_addon_group[0]}
				nfo, _ := PickNFO(new_addon_group[0].NFOList)
				addon_list = append(addon_list, Addon{
					AddonGroup: new_addon_group,
					Primary:    &new_addon_group[0],
					// TOC: is set later when we know the game track
					NFO:     &nfo,
					Ignored: NFOIgnored(nfo),
				})
			} else {
				// multiple addons in group

				primary := &installed_addon_group[0] // default. todo: sort by NFO for reproducible testing.
				group_ignore := false
				for _, installed_addon := range installed_addon_group {
					installed_addon := installed_addon
					nfo, _ := PickNFO(installed_addon.NFOList)
					if nfo.Primary {
						primary = &installed_addon
					}
					if nfo.Ignored != nil && *nfo.Ignored {
						group_ignore = true
					}
				}
				primary_nfo, _ := PickNFO(primary.NFOList)
				addon_list = append(addon_list, Addon{
					AddonGroup: installed_addon_group,
					Primary:    primary,
					// TOC: set later
					NFO:     &primary_nfo,
					Ignored: group_ignore,
				})
			}
		}
	}

	return addon_list, nil
}

// addon.clj/-load-installed-addon
// previously tightly integrated into data loading in v7, separate in v8
func SetInstalledAddonGameTrack(addon_dir AddonsDir, addon_list []Addon) []Addon {
	slog.Info("setting game track", "strict?", addon_dir.Strict, "game-track", addon_dir.GameTrackID)
	gt_pref_list := GT_PREF_MAP[addon_dir.GameTrackID]
	new_addon_list := []Addon{}
	for _, addon := range addon_list {
		if addon_dir.Strict {
			// in strict mode, toc data for selected game track is either present or it's not.
			toc, present := addon.Primary.TOCMap[addon_dir.GameTrackID]
			if !present {
				continue
			}
			addon.TOC = &toc
			new_addon_list = append(new_addon_list, addon)

		} else {
			// in relaxed mode, if there is *any* toc data it will be used.
			// use the preference map to decide the best one to use.
			for _, gt := range gt_pref_list {
				toc, present := addon.Primary.TOCMap[gt]
				if !present {
					continue
				}
				addon.TOC = &toc
				new_addon_list = append(new_addon_list, addon)
				break
			}
		}

		if addon.TOC == nil {
			slog.Warn("failed to set TOC data for installed addon", "addon-dir", addon_dir.Path, "group-id", addon.NFO) //.GroupID)
		}
	}
	return new_addon_list
}
