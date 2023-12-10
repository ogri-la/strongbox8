package strongbox

import (
	"bw/internal/core"
	"log/slog"
)

// correlates to addon.clj/-load-installed-addon
// unlike strongbox v7, v8 will attempt to load everything it can about an addon,
// regardless of game track, strictness, pinned status, ignore status, etc.
func load_installed_addon(addon_dir string) (InstalledAddon, error) {
	ParseAllAddonTocFiles(addon_dir)

	return InstalledAddon{}, nil //errors.New("not implemented")

}

// --- public

// reads the toc and nfo data from *all* addons in the given `addon_dir`,
// groups them and returns the result.
func LoadAllInstalledAddons(addons_dir AddonsDir) ([]InstalledAddon, error) {
	addon_list := []InstalledAddon{}
	dir_list, err := core.DirList(addons_dir.Path)
	if err != nil {
		return addon_list, err
	}
	for _, full_path := range dir_list {
		if BlizzardAddon(full_path) {
			continue
		}
		addon, err := load_installed_addon(full_path)
		if err != nil {
			slog.Error("failed to load addon", "error", err)
			continue
		}
		addon_list = append(addon_list, addon)
	}
	return addon_list, nil
}

// previously "core.clj/match-all-installed-addons-with-catalogue".
// compares the list of addons installed with the catalogue of known addons, match the two up, merge
// the two together and update the list of installed addons.
func Reconcile(installed_addon_list []InstalledAddon, catalogue Catalogue) []Addon {
	panic("not implemented")
}
