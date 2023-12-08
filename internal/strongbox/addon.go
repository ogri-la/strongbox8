package strongbox

// --- public

// reads the toc and nfo data from *all* addons in the given `addon_dir`,
// groups them and returns the result.
func LoadAllInstalledAddons(addon_dir AddonDir) []Addon {
	panic("not implemented")
}

// previously "core.clj/match-all-installed-addons-with-catalogue".
// compares the list of addons installed with the catalogue of known addons, match the two up, merge
// the two together and update the list of installed addons.
func Reconcile(installed_addon_list []Addon, catalogue Catalogue) []Addon {
	panic("not implemented")
}
