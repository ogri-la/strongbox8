package strongbox

import (
	"bw/core"
	"fmt"
)

/*
   strongbox provider and service wrangling for boardwalk.
   'services' are functions that the strongbox provider exposes to the system/user.
   the user can pass in parameters and call them,
   combinations of services and arguments can be saved to be called again later,
   services can be inspected,
   args can be thoroughly validated,
   other logic can call services, etc.

   these service functions should be thin wrappers around core logic,
   and be suffixed with 'Service',
   but it's not mandatory.
*/

// loads the addons in a specific AddonDir
func LoadAddonDirService(app *core.App, fnargs core.FnArgs) core.FnResult {
	addon_dir := fnargs.ArgList[0].Val.(string) // "addon-dir". todo: maybe add a fnargs.ArgMap[key] ? it would capture intent ..
	result_list, err := load_addons_dir(addon_dir)
	if err != nil {
		return core.FnResult{
			Err: fmt.Errorf("failed to load addons from selected addon dir: %w", err),
		}
	}
	return core.FnResult{
		Result: result_list,
	}
}

// takes the results of reading the settings and adds them to the app's state
func LoadSettingsService(app *core.App, fnargs core.FnArgs) core.FnResult {
	settings_file := fnargs.ArgList[0].Val.(string)
	result_list, err := strongbox_settings_service_load(settings_file)
	if err != nil {
		return core.NewErrorFnResult(err, "loading settings")
	}
	return core.FnResult{Result: result_list}
}

// pulls settings values from app state and writes results as json to a file
func strongbox_settings_service_save(app *core.App, args core.FnArgs) core.FnResult {
	//settings_file := args.ArgList[0].Val.(string)
	//fmt.Println(settings_file)
	return core.FnResult{}
}

func strongbox_settings_service_refresh(app *core.App, _ core.FnArgs) core.FnResult {
	refresh(app)
	return core.FnResult{}
}

func settings_file_argdef() core.ArgDef {
	return core.ArgDef{
		ID:      "settings-file",
		Label:   "Settings file",
		Default: core.HomePath("/.config/strongbox/config.json"), // todo: pull this from keyvals.strongbox.paths.cfg-file
		Parser:  core.ParseStringAsPath,                          // todo: create a settings file if one doesn't exist
		ValidatorList: []core.PredicateFn{
			core.IsFilenameValidator,
			core.FileDirIsWriteableValidator,
			core.FileIsWriteableValidator,
		},
	}
}

func StopService(app *core.App, fnargs core.FnArgs) core.FnResult {
	Stop(app)
	return core.FnResult{}
}

func StartService(app *core.App, fnargs core.FnArgs) core.FnResult {
	Start(app)
	return core.FnResult{}
}

// ---

func provider() []core.Service {
	state_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "state", Type: "service"},
		FnList: []core.Fn{
			core.StartProviderService(StartService),
			core.StopProviderService(StopService),
			{
				Label:       "Load settings",
				Description: "Reads the settings file, creating one if it doesn't exist, and loads the contents into state.",
				Interface: core.FnInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				TheFn: LoadSettingsService,
			},
			{
				Label:       "Save settings",
				Description: "Writes a settings file to disk.",
				Interface: core.FnInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				TheFn: strongbox_settings_service_save,
			},
			/*
				{
					Label:       "Default settings",
					Description: "Replace current settings with default settings. Does not save unless you 'save settings'!",
				},
				{
					Label: "Set preference",
				},
			*/
			{
				Label:       "Refresh",
				Description: "Reload addons, reload catalogues, check addons for updates, flush settings to disk, etc",
				TheFn:       strongbox_settings_service_refresh,
			},
		},
	}

	catalogue_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "catalogue", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Catalogue info",
				Description: "Displays information about each available catalogue, including the emergency catalogue.",
			},
			{
				Label: "Update catalogues",
			},
			{
				Label: "Switch active catalogue",
			},
		},
	}

	dir_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "addon-dir", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "New addon directory",
				Description: "Adds a new addon directory to configuration.",
			},
			{
				Label:       "Remove addon directory",
				Description: "Remove an addon directory from configuration.",
			},
			{
				Label:       "Load addon directory",
				Description: "Loads a list of addons in an addon directory.",
				Interface: core.FnInterface{
					ArgDefList: []core.ArgDef{
						{
							ID:            "addon-dir",
							Label:         "Addon Directory",
							Parser:        nil,                  // todo: needs to select from known addon dirs
							ValidatorList: []core.PredicateFn{}, // todo: ensure directory is readable?
						},
					},
				},
				TheFn: LoadAddonDirService,
			},
			{
				Label:       "Browse addon directory",
				Description: "Opens an addon directory in a file browser.",
			},
		},
	}

	addon_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "addon", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Install addon",
				Description: "Download and unzip an addon from the catalogue.",
			},
			{
				Label:       "Import addon",
				Description: "Install an addon from outside of the catalogue.",
			},
			{
				Label:       "Un-install addon",
				Description: "Removes an addon from an addon directory, including all bundled addons.",
			},
			{
				Label:       "Re-install addon",
				Description: "Install the addon again, possible for the first time through Strongbox.",
			},
			{
				Label:       "Check addon",
				Description: "Check online for any updates but do not install them.",
			},
			{
				Label:       "Update addon",
				Description: "Download and install any updates for the selected addon",
			},
			{
				Label:       "Pin addon",
				Description: "Prevent updates to this addon.",
			},
			{
				Label:       "Un-pin addon",
				Description: "If an addon is pinned, this will un-pin it.",
			},
			{
				Label:       "Ignore addon",
				Description: "Do not touch this addon. Do not update it, remove it, overwrite it not pin it.",
			},
			{
				Label:       "Stop ignoring addon",
				Description: "If an addon is being ignored, this will stop ignoring it.",
			},

			// ungroup addon
			// set primary addon
			// find similar addons
			// switch source
		},
	}

	search_services := core.Service{
		NS: core.NS{Major: "strongbox", Minor: "search", Type: "service"},
		FnList: []core.Fn{
			{
				Label:       "Search",
				Description: "Search catalogue for an addon by name and description.",
			},
		},
	}

	// general services, like clearing cache, pruning zip files, etc

	return []core.Service{
		state_services,
		catalogue_services,
		dir_services,
		addon_services,
		search_services,
	}
}
