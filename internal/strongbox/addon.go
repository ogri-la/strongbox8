package strongbox

import (
	"bw/internal/core"
	"log/slog"
)

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir PathToAddon) (InstalledAddon, error) {
	installed_addon := InstalledAddon{}
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
