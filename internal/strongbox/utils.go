package strongbox

func AddonID(addon Addon) string {
	//dirname := addon.TOC.DirName    // not good. this will be 'Addons' for regular users.
	source := addon.NFO.Source      // "github"
	source_id := addon.NFO.SourceID // "adiaddons/adibags"
	return source + "/" + source_id // "github/adiaddons/adibags", "wowinterface/adibags"
}
