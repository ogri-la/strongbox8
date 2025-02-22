package strongbox

import (
	"bw/core"
	"errors"
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

	// --- derived fields

	Name string // derived from NFOList[0].Name, see NewInstalledAddon
}

func NewInstalledAddon(url string, toc_map map[GameTrackID]TOC, nfo_list []NFO) *InstalledAddon {
	ia := &InstalledAddon{
		URL:     url,
		TOCMap:  toc_map,
		NFOList: nfo_list,
	}

	// warning! non-deterministic code
	for _, val := range toc_map {
		ia.Name = val.Name
		break
	}

	return ia
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
		core.ITEM_FIELD_NAME: ia.Name,
		core.ITEM_FIELD_URL:  ia.URL,
	}
	return row
}

// the natural children for an InstalledAddon,
// from the pov of an addon manager,
// are .toc files.
func (ia InstalledAddon) ItemChildren(_ *core.App) []core.Result {
	toc_result_list := []core.Result{}
	for _, toc := range ia.TOCMap {
		toc_result := core.NewResult(NS_TOC, toc, core.UniqueID())
		toc_result_list = append(toc_result_list, toc_result)
	}
	return toc_result_list
}

// --- Addon

// an 'addon' represents one or a group of installed addons.
// the group has a representative 'primary' addon.
type Addon struct {
	InstalledAddonGroup []InstalledAddon // required >= 1
	CatalogueAddon      *CatalogueAddon  // optional, the catalogue match
	SourceUpdateList    []SourceUpdate

	// --- fields derived from the above.

	Primary      *InstalledAddon // required, one of Addon.AddonGroup
	NFO          *NFO            // optional, Addon.Primary.NFO[-1] // todo: make this a list of NFO
	TOC          *TOC            // required, Addon.Primary.TOC[$gametrack]
	SourceUpdate *SourceUpdate   // chosen from Addon.SourceUpdateList by gametrack + sourceupdate type ('classic' + 'nolib')
	Ignored      bool            // required, `Addon.Primary.NFO[-1].Ignored` or `Addon.Primary.TOC[$gametrack].Ignored`

	// --- formerly only accessible for Addon.Attr.
	// for now these values are just the stringified versions of the original values. may change!

	ID       string
	Source   string
	SourceID string
	DirName  string
	//Title            string // ???
	Name             string // normalised label
	Label            string // preferred label
	Description      string
	URL              string
	Tags             string
	Created          string
	Updated          string
	Size             string
	InstalledVersion string
	AvailableVersion string
	GameVersion      string
	InterfaceVersion string
}

func NewAddon(installed_addon_list []InstalledAddon, primary_addon *InstalledAddon, toc *TOC, nfo *NFO) *Addon {
	a := &Addon{
		InstalledAddonGroup: installed_addon_list,
		Primary:             primary_addon,
		TOC:                 toc,
		NFO:                 nfo,
	}

	// ---

	has_toc := a.TOC != nil
	has_nfo := a.NFO != nil
	has_match := a.CatalogueAddon != nil
	has_updates := false
	//has_parent := false // is this an addon within a group?

	if has_nfo {
		a.Ignored = NFOIgnored(*nfo)
	}

	// raw title. does anything use this?
	/*
		if has_match {
			a.Title = a.CatalogueAddon.Label // "AdiBags"
		} else {
			// may be empty, use "label" for a safer "title"
			a.Title = a.TOC.Title
		}
	*/

	// human friendly addon title
	if has_match {
		a.Label = a.CatalogueAddon.Label // "AdiBags"
	} else if has_toc {
		a.Label = a.TOC.Label // "AdiBags", "Group Title *"
	}

	// normalised title
	if has_match {
		a.Name = a.CatalogueAddon.Name // "adibags"
	} else if has_nfo {
		a.Name = a.NFO.Name
	} else if has_toc {
		a.Name = a.TOC.Name
	} else {
		a.Name = primary_addon.Name
	}

	// description
	if has_match {
		a.Description = a.CatalogueAddon.Description
	} else if has_toc {
		a.Description = a.TOC.Notes
	}

	// url
	if has_match {
		a.URL = a.CatalogueAddon.URL
	}

	if has_toc {
		a.DirName = a.TOC.DirName
	}

	// interface-version "100105", "30402"
	if has_toc {
		a.InterfaceVersion = core.IntToString(a.TOC.InterfaceVersion)
	}

	// case "game-version": // "10.1.5", "3.4.2"
	if has_toc {
		v, err := InterfaceVersionToGameVersion(a.TOC.InterfaceVersion)
		if err != nil {
			a.GameVersion = v
		}
	}

	// case "installed-version": // v1.2.3, foo-bar.zip.v1, 10.12.0v1.4.2, 12312312312
	if has_nfo {
		a.InstalledVersion = a.NFO.InstalledVersion
	} else if has_toc {
		a.InstalledVersion = a.TOC.InstalledVersion
	}

	// case "available-version": // v1.2.4, foo-bar.zip.v2, 10.12.0v1.4.3, 22312312312
	if has_updates {
		a.AvailableVersion = a.SourceUpdate.Version
	} else if has_toc {
		a.AvailableVersion = a.TOC.InstalledVersion
	}

	// case "source":
	if has_match {
		a.Source = a.CatalogueAddon.Source
	} else if has_nfo {
		a.Source = a.NFO.Source
	} else {
		// can we do anything with a.TOC.SourceMapList if one exists?
		// I guess those are just *potential* sources rather than the *actual* source.
	}

	//case "source-id":
	if has_match {
		a.SourceID = string(a.CatalogueAddon.SourceID)
	} else if has_nfo {
		a.SourceID = string(a.NFO.SourceID)
	}

	// case "updated":
	if has_match {
		a.Updated = a.CatalogueAddon.UpdatedDate
	}

	return a
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
		"source":             a.Source,
		core.ITEM_FIELD_NAME: a.DirName,
		core.ITEM_FIELD_DESC: a.Description,
		core.ITEM_FIELD_URL:  a.URL,
		"tags":               a.Tags,
		"created":            a.Created,
		"updated":            a.Updated,
		"size":               a.Size,
		"installed-version":  a.InstalledVersion,
		"available-version":  a.AvailableVersion,
		"game-version":       a.GameVersion,
	}
}

// an Addon may be grouping multiple InstalledAddons.
// if so, they can be loaded immediately.
func (a Addon) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_TRUE
}

func (a Addon) ItemChildren(_ *core.App) []core.Result {
	children := []core.Result{}
	for _, installed_addon := range a.InstalledAddonGroup {
		ia_result := core.NewResult(NS_INSTALLED_ADDON, installed_addon, core.UniqueID())
		children = append(children, ia_result)
	}
	return children
}

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	empty_result := InstalledAddon{}
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		slog.Error("error parsing toc file", "error", err)
		return empty_result, err
	}
	url := "file://" + addon_dir
	nfo_list := ReadNFO(addon_dir)
	return *NewInstalledAddon(url, toc_map, nfo_list), nil
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
				// TOC: is set later when we know the game track
				// NFO: not found/bad data/invalid data
				addon := NewAddon([]InstalledAddon{installed_addon}, &installed_addon, nil, nil)
				addon_list = append(addon_list, *addon)
			}
		} else {
			// regular group

			if len(installed_addon_group) == 1 {
				// perfect case, no grouping.
				new_addon_group := []InstalledAddon{installed_addon_group[0]}
				// TOC: is set later when we know the game track
				nfo, _ := PickNFO(new_addon_group[0].NFOList)
				addon := NewAddon(new_addon_group, &new_addon_group[0], nil, &nfo)
				addon_list = append(addon_list, *addon)
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
				// TOC: set later
				primary_nfo, _ := PickNFO(primary.NFOList)
				addon := NewAddon(installed_addon_group, primary, nil, &primary_nfo)
				addon.Ignored = group_ignore
				addon_list = append(addon_list, *addon)
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
