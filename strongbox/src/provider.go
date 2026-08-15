package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"reflect"
)

const TAB_LABEL_INSTALLED = "installed"

// the strongbox provider: the interface strongbox presents to boardwalk.
// 'services' are the functions the provider exposes to the system and the user. they can
// be called with validated arguments, inspected, saved with their arguments to be called
// again, and called by other logic.
// a service function should be a thin wrapper around the logic elsewhere in this package,
// doing no more than calling it and putting the results into state.
// service functions are suffixed with 'Service' by convention, not by requirement.

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

// selects an addons dir and refreshes.
// panics if the first argument is not a `*core.Result` holding an `AddonsDir`.
func SelectAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	arg0 := fnargs.ArgList[0]
	addons_dir := arg0.Val.(*core.Result).Item.(AddonsDir) // urgh
	SelectAddonsDir(app, addons_dir.Path).Wait()
	Refresh(app)
	return core.ServiceResult{}
}

// removes an addons dir and saves the settings.
// the first argument is either a path, from a form submission, or a `*core.Result`, from
// the context menu.
// panics on any other argument type.
func RemoveAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	switch t := fnargs.ArgList[0].Val.(type) {
	case PathToDir:
		RemoveAddonsDir(app, t).Wait()
	case *core.Result:
		RemoveAddonsDir(app, t.Item.(AddonsDir).Path).Wait()
	default:
		slog.Error("RemoveAddonsDirService called with unsupported argument type", "type", fmt.Sprintf("%T", t))
		panic("programming error")
	}

	SaveSettings(app)

	return core.ServiceResult{}
}

// loads the settings into app state and refreshes.
// the settings-file argument is ignored, the path in app state is used.
func LoadSettingsService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	LoadSettings(app)
	Refresh(app)
	return core.ServiceResult{}
}

// writes the settings held in app state to disk.
// the settings-file argument is ignored, the path in app state is used.
func SaveSettingsService(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
	//settings_file := args.ArgList[0].Val.(string)
	//fmt.Println(settings_file)
	err := SaveSettings(app)
	if err != nil {
		return core.MakeServiceResultError(err, "failed to save settings")
	}
	return core.ServiceResult{}
}

func RefreshService(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
	Refresh(app)
	return core.ServiceResult{}
}

// not implemented yet: updates nothing and returns an empty result.
func UpdateAddonsService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	//update_all_addons(app) // todo: finish implementing
	return core.ServiceResult{}
}

func CheckForUpdatesService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	CheckForUpdates(app)
	return core.ServiceResult{}
}

// creates a new addons dir, selects it and saves the settings.
// panics if the first argument is not a path.
func NewAddonsDirService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	addons_dir := fnargs.ArgList[0].Val.(PathToDir)
	CreateAddonsDir(app, addons_dir).Wait()
	SelectAddonsDir(app, addons_dir).Wait()
	SaveSettings(app)
	return core.ServiceResult{}
}

// https://vitaneri.com/posts/implementing-map-filter-and-reduce-using-generic-in-go
func Map[T1, T2 any](s []T1, f func(T1) T2) []T2 {
	r := make([]T2, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

// installs one or many catalogue addons into the selected addons dir, then reconciles.
// the first argument is a `*core.Result` or a list of them, holding `CatalogueAddon`
// items. any other type is logged and nothing is installed.
// switches the UI to the installed tab before installing.
// returns an error only when there is no addons dir to install into.
func InstallCatalogueAddonService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	ad, err := selected_addon_dir(app)
	if err != nil {
		msg := "failed to find an addon directory to install addon(s) into"
		return core.MakeServiceResultError(err, msg)
	}

	switch t := fnargs.ArgList[0].Val.(type) {
	case *core.Result:
		app.DispatchAction(core.Action{Type: core.ACTION_SWITCH_TAB, Payload: TAB_LABEL_INSTALLED})
		install_addon_from_catalogue(app, ad, t.Item.(CatalogueAddon))
	case []*core.Result:
		app.DispatchAction(core.Action{Type: core.ACTION_SWITCH_TAB, Payload: TAB_LABEL_INSTALLED})
		cal := Map(t, func(r *core.Result) CatalogueAddon {
			return r.Item.(CatalogueAddon)
		})
		install_many_addons_from_catalogue(app, ad, cal)
	default:
		slog.Error("expected a list of catalogue addons", "got", t)
	}

	//Refresh(app) // doesn't refresh gui contents either
	Reconcile(app) // todo: doesn't refresh gui contents

	return core.ServiceResult{}
}

// removes one or many addons from disk and from app state, then refreshes.
// the first argument is a `*core.Result` or a list of them, holding `Addon` items.
// any other type is logged and nothing is removed.
// a failure to remove an individual addon is swallowed.
func RemoveAddonsService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	switch t := fnargs.ArgList[0].Val.(type) {
	case *core.Result:
		RemoveAddon(app, t)

	case []*core.Result:
		for _, r := range t {
			RemoveAddon(app, r)
		}

	default:
		slog.Error("expected a list of Addons", "got", t)
	}

	Refresh(app)

	return core.ServiceResult{}
}

// checks one or many addons for updates.
// the first argument is a `*core.Result` or a list of them, holding `Addon` items.
// any other type is logged and nothing is checked.
func CheckAddonService(app *core.App, fnargs core.ServiceFnArgs) core.ServiceResult {
	switch t := fnargs.ArgList[0].Val.(type) {
	case *core.Result:
		CheckAddon(app, t)

	case []*core.Result:
		for _, r := range t {
			CheckAddon(app, r)
		}

	default:
		slog.Error("expected a Result or list of Results", "got", t)
	}

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

// returns an 'are you sure?' argument, defaulting to 'false'.
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

// returns an argument for selecting one of the existing addons dirs.
// defaults to the currently selected addons dir, or an empty string when none is
// selected.
// todo: the choices are not enforced by the validator, only by the widget.
func extant_addons_dir_argdef() core.ArgDef {
	return core.ArgDef{
		ID:     "addons-dir",
		Label:  "Addons Directory",
		Widget: core.InputWidgetSelection,

		Choice: &core.ArgChoice{
			ChoiceFn: func(app *core.App) []any {
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
			core.IsDirValidator,
		},
	}
}

// ---

const SERVICE_ID_NEW_ADDONS_DIR = "new-addons-dir"

// returns every service group strongbox offers.
// a service with no `Fn` is a placeholder and does nothing when called.
func provider() []core.ServiceGroup {
	// the bare minimum to bootstrap strongbox.
	// every other group is optional and can be disabled without breaking anything.
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
				ID:          SERVICE_ID_NEW_ADDONS_DIR,
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
				Fn:          UpdateAddonsService,
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
				Description: "Install an addon using a URL from a (supported) source",
			},
			{
				ID:          "uninstall-addon",
				Label:       "Un-install addon",
				Description: "Remove an addon, including any bundled addons",
				Interface: core.ServiceInterface{
					ArgDefList: []core.ArgDef{
						{
							ID:            "selected",
							Label:         "Selected Addons",
							Widget:        core.InputWidgetTextField,
							ValidatorList: []core.PredicateFn{},
						},
						//confirm_argdef(),
					},
				},
				Fn: RemoveAddonsService,
			},
			{
				Label:       "Re-install addon",
				Description: "Install an addon again, possibly for the first time through Strongbox",
			},
			{
				ID:          "check-addon",
				Label:       "Check for updates",
				Description: "Check online for any updates but do not install them",
				Fn:          CheckAddonService,
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

// returns the value for `key` in the map `m`.
// panics if the key is absent: callers use it for keys that must exist, so a miss is a
// wiring defect rather than a runtime condition.
func GetKey[K comparable, V any](key K, m map[K]V) V {
	v, present := m[key]
	if !present {
		panic(fmt.Sprintf("programming error, key not found: %v", key))
	}
	return v
}

// returns the services that accept each item type, used to build the context menu of a
// selected item.
// panics if a service named here is not in the service list.
// todo: the mapping is written out by hand. tag services with the item types they accept
// and derive it instead.
func (sp *StrongboxProvider) ItemHandlerMap() map[reflect.Type][]core.Service {
	services := provider()
	// an index of service-id => service, so services can be found by ID below
	service_idx := map[string]core.Service{} // {service-id: Service, ...}
	for _, sg := range services {
		for _, s := range sg.ServiceList {
			service_idx[s.ID] = s
		}
	}

	rv := map[reflect.Type][]core.Service{}
	rv[reflect.TypeFor[AddonsDir]()] = []core.Service{
		GetKey("select-addons-dir", service_idx),
		GetKey("remove-addons-dir", service_idx),
	}
	rv[reflect.TypeFor[Addon]()] = []core.Service{
		GetKey("check-addon", service_idx),
		GetKey("update-addon", service_idx),
		GetKey("uninstall-addon", service_idx),
	}
	rv[reflect.TypeFor[[]Addon]()] = []core.Service{
		GetKey("check-addon", service_idx),
		GetKey("update-addon", service_idx),
		GetKey("uninstall-addon", service_idx),
	}
	rv[reflect.TypeFor[CatalogueAddon]()] = []core.Service{
		GetKey("install-catalogue-addon", service_idx),
	}
	rv[reflect.TypeFor[[]CatalogueAddon]()] = []core.Service{
		GetKey("install-catalogue-addon", service_idx),
	}
	return rv
}

func (sp *StrongboxProvider) Menu() []core.Menu {
	donothing := func(_ *core.App) {
		slog.Info("not implemented")
	}

	return []core.Menu{
		{Name: "File", MenuItemList: []core.MenuItem{
			{Name: "Install Addon From File", Fn: donothing},
			{Name: "Import Addon", Fn: donothing},
			core.MENU_SEP,
			{Name: "New Addons Directory", ServiceID: SERVICE_ID_NEW_ADDONS_DIR},
			{Name: "Update All", Fn: donothing},
		}},
		{Name: "Edit", MenuItemList: []core.MenuItem{
			{Name: "Columns", Fn: donothing},
		}},
		{Name: "View", MenuItemList: []core.MenuItem{
			{Name: "Refresh", Fn: donothing},
		}},
	}
}

// ---

func Provider(app *core.App) *StrongboxProvider {
	return &StrongboxProvider{}
}
