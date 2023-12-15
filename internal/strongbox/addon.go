package strongbox

import (
	"bw/internal/core"
	//"fmt"
	"log/slog"
)

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	//fmt.Println(addon_dir)
	installed_addon := InstalledAddon{}
	toc_map, err := ParseAllAddonTocFiles(addon_dir)
	if err != nil {
		slog.Error("error parsing toc file", "error", err)
		return installed_addon, err
	}
	installed_addon.TOC = toc_map

	nfo_list := ReadNFO(addon_dir)
	installed_addon.NFO = nfo_list

	//fmt.Println(core.QuickJSON(installed_addon))
	//fmt.Println("--- done")

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
	installed_addon_groups := core.GroupBy[InstalledAddon](installed_addon_list, func(installed_addon InstalledAddon) string {
		if len(installed_addon.NFO) == 0 {
			// no nfo data.
			// it either wasn't found or was bad and ignored.
			return nogroup
		}
		// the first nfo is always the one to use
		return installed_addon.NFO[0].GroupID
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
					// TOC: set later
					// NFO: not found/bad data/invalid data
					Ignored: false,
				})
			}
		} else {
			// regular group

			if len(installed_addon_group) == 1 {
				// perfect case, no grouping.
				new_addon_group := []InstalledAddon{installed_addon_group[0]}
				addon_list = append(addon_list, Addon{
					AddonGroup: new_addon_group,
					Primary:    &new_addon_group[0],
					// TOC: set later
					NFO:     &new_addon_group[0].NFO[0],
					Ignored: NFOIgnored(new_addon_group[0].NFO[0]),
				})
			} else {
				// multiple addons in group

				primary := &installed_addon_group[0] // default. todo: sort by NFO for reproducible testing.
				group_ignore := false
				for _, installed_addon := range installed_addon_group {
					if installed_addon.NFO[0].Primary {
						primary = &installed_addon
					}
					if installed_addon.NFO[0].Ignored != nil && *installed_addon.NFO[0].Ignored {
						group_ignore = true
					}
				}
				addon_list = append(addon_list, Addon{
					AddonGroup: installed_addon_group,
					Primary:    primary,
					// TOC: set later
					NFO:     &primary.NFO[0],
					Ignored: group_ignore,
				})
			}
		}
	}

	//fmt.Println(core.QuickJSON(addon_list))

	return addon_list, nil
}

// previously "core.clj/match-all-installed-addons-with-catalogue".
// compares the list of addons installed with the catalogue of known addons, match the two up, merge
// the two together and update the list of installed addons.
func Reconcile(addon_list []Addon, catalogue Catalogue) []Addon {
	panic("not implemented")
}
