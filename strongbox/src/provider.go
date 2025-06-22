package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"reflect"
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
/*
func LoadAddonDirService(app *core.App, fnargs core.ServiceArgs) core.ServiceResult {
	arg0 := fnargs.ArgList[0]

	var addons_dir AddonsDir
	if reflect.TypeOf(arg0.Val) == reflect.TypeOf(&core.Result{}) {
		addons_dir = arg0.Val.(*core.Result).Item.(AddonsDir) // urgh
	} else {
		path := fnargs.ArgList[0].Val.(string) // "addon-dir". todo: maybe add a fnargs.ArgMap[key] ? it would capture intent ..
		addons_dir = AddonsDir{Path: path, Strict: true, GameTrackID: GAMETRACK_RETAIL}
	}

	// set selected addons dir
	// loads addons dir
	// refresh the GUI somehow

	// this is already being called by simply adding an addons dir to app state.
	// it's children() fn calls this.
	// so, already loaded.
	// we just need to implement changing the addons dir, check for updates
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
*/

func SelectAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	arg0 := fnargs.ArgList[0]
	addons_dir := arg0.Val.(*core.Result).Item.(AddonsDir) // urgh

	select_addons_dir(app, addons_dir)

	SaveSettings(app) // a refresh will save the settings

	//Refresh(app) // I want to move away from these 'refresh' calls

	return core.ServiceResult{}
}

func RemoveAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	switch t := fnargs.ArgList[0].Val.(type) {
	case PathToDir:
		// called from a form submission
		RemoveAddonsDir(app, t).Wait()
	case *core.Result:
		// called from context menu
		RemoveAddonsDir(app, t.Item.(AddonsDir).Path).Wait()
	default:
		slog.Error("RemoveAddonsDirService called with unsupported argument type", "type", fmt.Sprintf("%T", t))
		panic("programming error")
	}

	SaveSettings(app)

	return core.ServiceResult{}
}

// takes the results of reading the settings and adds them to the app's state
func LoadSettingsService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	LoadSettings(app)
	Refresh(app)
	return core.ServiceResult{}
}

// pulls settings values from app state and writes results as json to a file
func SaveSettingsService(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
	//settings_file := args.ArgList[0].Val.(string)
	//fmt.Println(settings_file)
	err := save_settings_file(app)
	if err != nil {
		return core.MakeServiceResultError(err, "failed to save settings")
	}
	return core.ServiceResult{}
}

func RefreshService(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
	Refresh(app)
	return core.ServiceResult{}
}

func UpdateAddonsService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	update_all_addons(app)
	return core.ServiceResult{}
}

func CheckForUpdatesService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	CheckForUpdates(app)
	return core.ServiceResult{}
}

func NewAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	CreateAddonsDir(app, fnargs.ArgList[0].Val.(PathToDir)).Wait()
	SaveSettings(app)
	return core.ServiceResult{}
}

func InstallCatalogueAddonService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	slog.Error("not implemented")
	return core.ServiceResult{}
}

// ---

func StopService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	Stop(app)
	return core.ServiceResult{}
}

func StartService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	err := Start(app)
	if err != nil {
		return core.MakeServiceResultError(err, "failed to start provider")
	}
	return core.ServiceResult{}
}

// common args

// a simple 'are you sure?' argument
func confirm_argdef() core.ArgDef {
	return core.ArgDef{
		ID:      "confirm",
		Label:   "Confirm",
		Default: "false",
		ValidatorList: []core.PredicateFn{
			core.IsTruthyFalsey,
		},
		Parser: core.ParseTruthyFalseyAsBool,
	}
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

// select an existing addons dir from a list of choices.
func extant_addons_dir_argdef() core.ArgDef {

	return core.ArgDef{
		ID:    "addons-dir",
		Label: "Addons Directory",

		// valid input is constrained to just these
		// todo: shouldn't these also be in the validator?
		// todo: should this even be a thing? why don't we just give them a drop down widget?

		Widget: core.InputWidgetSelection,

		Choice: &core.ArgChoice{
			ChoiceFn: func(app *core.App) []any {
				// hrm, this is what I want but it's kinda sucky
				choice_list := []any{}
				for _, i := range app.FilterResultListByNS(NS_ADDONS_DIR) {
					choice_list = append(choice_list, i)
				}
				return choice_list
			},
			Exclusivity: core.ArgChoiceExclusive,
		},
		DefaultFn: func(app *core.App) string {
			cur_selected, _ := selected_addon_dir(app)
			return cur_selected.Path // on error, .Path is empty string
		},
		ValidatorList: []core.PredicateFn{
			// todo: ensure directory is one of the given choices
			core.IsDirValidator, // ensure directory actually is a directory
			// todo: ensure directory is also readable/writeable?
		},
	}
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
				Fn: SaveSettingsService,
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
				Fn:          RefreshService,
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
			{
				ID:          "install-catalogue-addon",
				Label:       "Install catalogue addon",
				Description: "Install an addon from the catalogue.",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						extant_addons_dir_argdef(),
					},
				},
				Fn: InstallCatalogueAddonService,
			},
		},
	}

	addons_dir_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "addons-dir", Type: "service"},
		ServiceList: []core.Service{
			{
				ID:          "new-addons-dir",
				Label:       "New addons directory",
				Description: "Create a new addons directory",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						{
							ID:    "addons-dir",
							Label: "Addons Directory",
							//Widget:        core.InputWidgetDirSelection,
							Widget:        core.InputWidgetTextField,
							ValidatorList: []core.PredicateFn{core.IsDirValidator},
						},
					},
				},
				Fn: NewAddonsDirService,
			},
			{
				ID:          "remove-addons-dir",
				Label:       "Remove addons directory",
				Description: "Remove an addons directory",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						extant_addons_dir_argdef(),
						//confirm_argdef(),
					},
				},
				Fn: RemoveAddonsDirService,
			},
			/*
				{
					ID:          "load-addons-dir",
					Label:       "Load addons directory",
					Description: "Loads a list of addons within an addons directory",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							{
								ID:    "addons-dir",
								Label: "Addons Directory",
								Choice: &core.ArgChoice{
									ChoiceFn: func(app *core.App) []any {
										// hrm, this is what I want but it's kinda sucky
										choice_list := []any{}
										for _, i := range app.FilterResultListByNS(NS_ADDONS_DIR) {
											choice_list = append(choice_list, i)
										}
										return choice_list
									},
									Exclusivity: core.ArgChoiceExclusive,
								},
								//Parser:        nil,                  // todo: needs to select from known addon dirs
								//ValidatorList: []core.PredicateFn{}, // todo: ensure directory is readable?
							},
						},
					},
					Fn: LoadAddonDirService,
				},
			*/
			{
				ID:          "select-addons-dir",
				Label:       "Select addons directory",
				Description: "Selects an addons directory to check for updates",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						extant_addons_dir_argdef(),
						confirm_argdef(),
					},
				},
				Fn: SelectAddonsDirService,
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
				Description: "Install an addon from the filesystem",
			},
			{
				Label:       "Import addon",
				Description: "Install an addon from a (supported) remote source",
			},
			{
				Label:       "Un-install addon",
				Description: "Remove an addon, including any bundled addons",
			},
			{
				Label:       "Re-install addon",
				Description: "Install an addon again, possibly for the first time through Strongbox",
			},
			{
				ID:          "check-addon",
				Label:       "Check addon",
				Description: "Check online for any updates but do not install them",
			},
			{
				ID:          "update-addon",
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
		state_services,
		catalogue_services,
		addons_dir_services,
		addon_services,
		//search_services,
	}
}

// ---

type StrongboxProvider struct{}

var _ core.Provider = (*StrongboxProvider)(nil)

func (sp *StrongboxProvider) ID() string {
	return "strongbox"
}

func (sp *StrongboxProvider) ServiceList() []core.ServiceGroup {
	return provider()
}

func GetKey[K comparable, V any](key K, m map[K]V) V {
	v, present := m[key]
	if !present {
		panic(fmt.Sprintf("programming error, key not found: %v", key))
	}
	return v
}

// a mapping of item type to a group of services.
// the idea is that a provider can raise their hand and say 'I support $thing! Here are services that use it',
// and then the selected thing + any other input + parsing + validation happens.
func (sp *StrongboxProvider) ItemHandlerMap() map[reflect.Type][]core.Service {
	// urughurhgurhg. ok. we're making an index of service-id => service so we can find individual services by ID
	// and then associate them with a type.
	services := provider()
	service_idx := map[string]core.Service{} // {service-id: Service, ...}
	for _, sg := range services {
		for _, s := range sg.ServiceList {
			service_idx[s.ID] = s
		}
	}

	// for now, we just want items of type `AddonsDir` to be associated with specific services.
	// we can get more/less clever about this later
	rv := make(map[reflect.Type][]core.Service)
	rv[reflect.TypeOf(AddonsDir{})] = []core.Service{
		// not keen on this not failing if key doesn't exist.
		// generate all of this automatically? tag services with the item types they support?
		//revidx["new-addons-directory"],
		GetKey("select-addons-dir", service_idx), // this is better, but overall it's still too manual
		GetKey("remove-addons-dir", service_idx),
	}
	rv[reflect.TypeOf(Addon{})] = []core.Service{
		GetKey("check-addon", service_idx),
		GetKey("update-addon", service_idx),
	}
	rv[reflect.TypeOf([]Addon{})] = []core.Service{
		GetKey("check-addon", service_idx),
		GetKey("update-addon", service_idx),
	}
	rv[reflect.TypeOf(CatalogueAddon{})] = []core.Service{
		GetKey("install-catalogue-addon", service_idx),
	}
	rv[reflect.TypeOf([]CatalogueAddon{})] = []core.Service{
		GetKey("install-catalogue-addon", service_idx),
	}
	return rv
}

// ---

func Provider(app *core.App) *StrongboxProvider {
	return &StrongboxProvider{}
}
