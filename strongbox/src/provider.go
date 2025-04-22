package strongbox

import (
	"bw/core"
	"fmt"
)

// provider.go pulls together the logic from the rest of the strongbox logic and presents an
// interface to the rest of the app.
// it shouldn't do much more than describe services, call logic and stick results into state.

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
func LoadAddonDirService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	path := fnargs.ArgList[0].Val.(string) // "addon-dir". todo: maybe add a fnargs.ArgMap[key] ? it would capture intent ..
	addons_dir := AddonsDir{Path: path, Strict: true, GameTrackID: GAMETRACK_RETAIL}
	result_list, err := load_addons_dir(addons_dir)
	if err != nil {
		return core.ServiceResult{
			Err: fmt.Errorf("failed to load addons from selected addon dir: %w", err),
		}
	}
	return core.ServiceResult{
		Result: result_list,
	}
}

// takes the results of reading the settings and adds them to the app's state
func LoadSettingsService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	settings_file := fnargs.ArgList[0].Val.(string)
	result_list, err := strongbox_settings_service_load(settings_file)
	if err != nil {
		return core.NewServiceResultError(err, "loading settings")
	}
	return core.ServiceResult{Result: result_list}
}

// pulls settings values from app state and writes results as json to a file
func strongbox_settings_service_save(app *core.App, args core.ServiceArgs) core.ServiceResult {
	//settings_file := args.ArgList[0].Val.(string)
	//fmt.Println(settings_file)
	return core.ServiceResult{}
}

func strongbox_settings_service_refresh(app *core.App, _ core.ServiceArgs) core.ServiceResult {
	refresh(app)
	return core.ServiceResult{}
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

// ---

func UpdateAddonsService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	update_all_addons(app)
	return core.ServiceResult{}
}

func CheckForUpdatesService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	check_for_updates(app)
	return core.ServiceResult{}
}

func StopService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	Stop(app)
	return core.ServiceResult{}
}

func StartService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	Start(app)
	return core.ServiceResult{}
}

// ---

func provider() []core.ServiceGroup {
	// the absolute bare minimum to get strongbox bootstrapped and running.
	// everything else is optional and can be disabled without breaking anything.
	required_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "state", Type: "required"},
		ServiceList: []core.Service{
			core.StartProviderService(StartService),
			core.StopProviderService(StopService),
		},
	}
	state_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "state", Type: "service"},
		ServiceList: []core.Service{
			{
				Label:       "Load settings",
				Description: "Reads the settings file, creating one if it doesn't exist, and loads the contents into state.",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				Fn: LoadSettingsService,
			},
			{
				Label:       "Save settings",
				Description: "Writes a settings file to disk.",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						settings_file_argdef(),
					},
				},
				Fn: strongbox_settings_service_save,
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
				Fn:          strongbox_settings_service_refresh,
			},
		},
	}

	catalogue_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "catalogue", Type: "service"},
		ServiceList: []core.Service{
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

	dir_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "addon-dir", Type: "service"},
		ServiceList: []core.Service{
			{
				Label:       "New addons directory",
				Description: "Create a new addons directory",
			},
			{
				Label:       "Remove addons directory",
				Description: "Remove an addons directory",
			},
			{
				Label:       "Load addons directory",
				Description: "Loads a list of addons within an addons directory",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						{
							ID:            "addon-dir",
							Label:         "Addon Directory",
							Parser:        nil,                  // todo: needs to select from known addon dirs
							ValidatorList: []core.PredicateFn{}, // todo: ensure directory is readable?
						},
					},
				},
				Fn: LoadAddonDirService,
			},
			{
				Label:       "Browse an addons directory",
				Description: "Opens an addons directory in a file browser",
			},
			{
				Label:       "Check for updates",
				Description: "Checks all addons for updates in an addons directory.",
				Fn:          CheckForUpdatesService,
			},
			{
				Label:       "Update addons",
				Description: "Download and install updates for all addons in an addons directory",
				Fn:          CheckForUpdatesService,
			},
		},
	}

	addon_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "addon", Type: "service"},
		ServiceList: []core.Service{
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

	search_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "search", Type: "service"},
		ServiceList: []core.Service{
			{
				Label:       "Search",
				Description: "Search catalogue for an addon by name and description.",
			},
		},
	}

	// general services, like clearing cache, pruning zip files, etc

	if false {
		fmt.Println(addon_services, search_services, catalogue_services, state_services)
	}

	return []core.ServiceGroup{
		required_services,
		//state_services,
		//catalogue_services,
		dir_services,
		//addon_services,
		//search_services,
	}
}

// ---

type StrongboxProvider struct{}

var _ core.Provider = (*StrongboxProvider)(nil)

func (sp *StrongboxProvider) ServiceList() []core.ServiceGroup {
	return provider()
}

// ---

func Provider(app *core.App) *StrongboxProvider {
	return &StrongboxProvider{}
}
