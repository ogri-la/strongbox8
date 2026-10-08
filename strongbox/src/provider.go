package strongbox

import (
	"bw/core"
	"fmt"
	"log/slog"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
)

const TAB_LABEL_INSTALLED = "installed"

// the strongbox provider: the interface strongbox presents to boardwalk.
// 'services' are the functions the provider exposes to the system and the user. they can
// be called with validated arguments, inspected, saved with their arguments to be called
// again, and called by other logic.
// a service function is a thin wrapper around the logic elsewhere in this package. it
// declares the items it accepts and when it applies, so boardwalk builds the context
// menus from data.

// --- arguments

// returns the results the user selected, given as the first argument either as one
// result or as a list of them.
func selected_results(args core.ServiceFnArgs) []*core.Result {
	if len(args.ArgList) == 0 {
		return nil
	}
	switch t := args.ArgList[0].Val.(type) {
	case *core.Result:
		return []*core.Result{t}
	case []*core.Result:
		return t
	}
	return nil
}

// returns the value of the argument `id`, or nil.
func arg(args core.ServiceFnArgs, id string) any {
	for _, kv := range args.ArgList {
		if kv.Key == id {
			return kv.Val
		}
	}
	return nil
}

// returns the item of type `T` held by every selected result.
func selected_items[T any](args core.ServiceFnArgs) []T {
	items := []T{}
	for _, r := range selected_results(args) {
		if item, ok := r.Item.(T); ok {
			items = append(items, item)
		}
	}
	return items
}

// returns `true` when `pred` holds for any of the items of type `T` in `selected`.
func any_item[T any](selected []core.Result, pred func(T) bool) bool {
	for _, r := range selected {
		if item, ok := r.Item.(T); ok && pred(item) {
			return true
		}
	}
	return false
}

// the argument filled with the user's selection.
func selected_argdef() core.ArgDef {
	return core.ArgDef{ID: "selected", Label: "Selected", FromSelection: true}
}

// accepts one or many items of type `T`.
func accepts_many[T any]() *core.Accepts {
	return &core.Accepts{Types: []reflect.Type{reflect.TypeFor[T]()}, Many: true}
}

// accepts exactly one item of type `T`.
func accepts_one[T any]() *core.Accepts {
	return &core.Accepts{Types: []reflect.Type{reflect.TypeFor[T]()}}
}

// returns the result of a service: empty on success, the error otherwise.
func service_result(err error, msg string) core.ServiceResult {
	if err != nil {
		return core.MakeServiceResultError(err, msg)
	}
	return core.ServiceResult{}
}

// runs a refresh in the background as a job started now, so the app is not idle before
// the refresh begins and the service worker is not held up by the network.
func background_refresh(app *core.App) {
	job := app.StartJob("starting refresh", 0)
	go func() {
		defer job.Finish()
		Refresh(app)
	}()
}

// returns `true` when there is an available addons dir to install into.
func can_install(app *core.App) bool {
	_, err := install_target_dir(app)
	return err == nil
}

// returns the labels of the selected addons, joined.
func selected_labels(args core.ServiceFnArgs) string {
	labels := []string{}
	for _, a := range selected_items[Addon](args) {
		labels = append(labels, a.Label)
	}
	return strings.Join(labels, ", ")
}

// returns the IDs of the selected results.
func selected_ids(args core.ServiceFnArgs) []string {
	ids := []string{}
	for _, r := range selected_results(args) {
		ids = append(ids, r.ID)
	}
	return ids
}

// --- services

const (
	SERVICE_ID_NEW_ADDONS_DIR      = "new-addons-dir"
	SERVICE_ID_INSTALL_FROM_FILE   = "install-addon-from-file"
	SERVICE_ID_IMPORT_ADDON        = "import-addon"
	SERVICE_ID_UPDATE_ALL          = "update-all"
	SERVICE_ID_REFRESH             = "refresh"
	SERVICE_ID_SWITCH_CATALOGUE    = "switch-catalogue"
	SERVICE_ID_REFRESH_USER_CAT    = "refresh-user-catalogue"
	SERVICE_ID_PREFERENCES         = "preferences"
	SERVICE_ID_INSTALL_CAT_ADDON   = "install-catalogue-addon"
	SERVICE_ID_UNINSTALL_ADDON     = "uninstall-addon"
	SERVICE_ID_REMOVE_ADDONS_DIR   = "remove-addons-dir"
	SERVICE_ID_SELECT_ADDONS_DIR   = "select-addons-dir"
	SERVICE_ID_SET_GAME_TRACK      = "set-game-track"
	SERVICE_ID_SET_STRICTNESS      = "set-strictness"
	SERVICE_ID_BROWSE_ADDONS_DIR   = "browse-addons-dir"
	SERVICE_ID_CHECK_ADDON         = "check-addon"
	SERVICE_ID_UPDATE_ADDON        = "update-addon"
	SERVICE_ID_REINSTALL_ADDON     = "reinstall-addon"
	SERVICE_ID_ADDON_RELEASES      = "addon-releases"
	SERVICE_ID_SWITCH_SOURCE       = "switch-source"
	SERVICE_ID_PIN_ADDON           = "pin-addon"
	SERVICE_ID_UNPIN_ADDON         = "unpin-addon"
	SERVICE_ID_IGNORE_ADDON        = "ignore-addon"
	SERVICE_ID_STOP_IGNORING_ADDON = "stop-ignoring-addon"
	SERVICE_ID_STAR_ADDON          = "star-addon"
	SERVICE_ID_STAR_CAT_ADDON      = "star-catalogue-addon"
	SERVICE_ID_UNSTAR_CAT_ADDON    = "unstar-catalogue-addon"
)

// returns the services for addons dirs.
func addons_dir_services() core.ServiceGroup {
	game_track_choice := &core.ArgChoice{
		Exclusivity: core.ArgChoiceExclusive,
		ChoiceFn: func(*core.App) []any {
			choices := []any{}
			for _, gt := range GAME_TRACK_LIST {
				choices = append(choices, gt.ID)
			}
			return choices
		},
		LabelFn: func(v any) string { return GameTrackLabel(v.(GameTrackID)) },
	}
	selection := core.ServiceInterface{ArgDefList: []core.ArgDef{selected_argdef()}}
	return core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "addons-dir", Type: "service"},
		ServiceList: []core.Service{
			{
				ID:          SERVICE_ID_NEW_ADDONS_DIR,
				Label:       "New addons directory",
				Description: "Add a WoW addons directory and select it.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{{
					ID: "addons-dir", Label: "Addons directory", Widget: core.InputWidgetDirSelection,
					ValidatorList: []core.PredicateFn{core.IsDirValidator},
				}}},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					err := AddAddonsDir(app, fmt.Sprint(arg(args, "addons-dir")))
					if err == nil {
						background_refresh(app)
					}
					return service_result(err, "failed to add addons directory")
				},
			},
			{
				ID: SERVICE_ID_SELECT_ADDONS_DIR, Label: "Select",
				Description: "Select this addons directory, loading its addons and checking them for updates.",
				Interface:   selection, Accepts: accepts_one[AddonsDir](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(ad AddonsDir) bool { return ad.Available() && !ad.selected })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, ad := range selected_items[AddonsDir](args) {
						if err := SelectAddonsDir(app, ad.Path); err != nil {
							return service_result(err, "failed to select addons directory")
						}
					}
					background_refresh(app)
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_SET_GAME_TRACK, Label: "Set game track",
				Description: "Choose which version of WoW this addons directory is for.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
					selected_argdef(),
					{ID: "game-track", Label: "Game track", Widget: core.InputWidgetSelection, Choice: game_track_choice},
				}},
				Accepts: accepts_one[AddonsDir](),
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					game_track, _ := arg(args, "game-track").(GameTrackID)
					for _, ad := range selected_items[AddonsDir](args) {
						if err := SetAddonsDirGameTrack(app, ad.Path, game_track); err != nil {
							return service_result(err, "failed to set game track")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_SET_STRICTNESS, Label: "Set strictness",
				Description: "When strict, only releases for this game track are installed. When relaxed, the nearest game track is used when there is none.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
					selected_argdef(),
					{ID: "strict", Label: "Strict", Widget: core.InputWidgetCheckbox, Default: "true", Parser: core.ParseTruthyFalseyAsBool},
				}},
				Accepts: accepts_one[AddonsDir](),
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					strict, _ := arg(args, "strict").(bool)
					for _, ad := range selected_items[AddonsDir](args) {
						SetAddonsDirStrict(app, ad.Path, strict)
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_BROWSE_ADDONS_DIR, Label: "Browse",
				Description: "Open this addons directory in the file manager.",
				Interface:   selection, Accepts: accepts_one[AddonsDir](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(ad AddonsDir) bool { return ad.Available() })
				},
				Fn: func(_ *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, ad := range selected_items[AddonsDir](args) {
						cmd := exec.Command("xdg-open", ad.Path)
						if err := cmd.Start(); err != nil {
							slog.Warn("failed to open the file manager", "path", ad.Path, "error", err)
							continue
						}
						go cmd.Wait() // detached, the file manager outlives the call
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_REMOVE_ADDONS_DIR, Label: "Remove",
				Description: "Remove this addons directory from strongbox. No files are deleted.",
				Interface:   selection, Accepts: accepts_one[AddonsDir](),
				Confirm: func(_ *core.App, args core.ServiceFnArgs) string {
					paths := []string{}
					for _, ad := range selected_items[AddonsDir](args) {
						paths = append(paths, ad.Path)
					}
					return fmt.Sprintf("Remove the addons directory %s from strongbox?\nNo files are deleted, it can be added again at any time.", strings.Join(paths, ", "))
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, ad := range selected_items[AddonsDir](args) {
						RemoveAddonsDir(app, ad.Path)
					}
					return core.ServiceResult{}
				},
			},
		},
	}
}

// returns the services for installed addons.
func addon_services() core.ServiceGroup {
	release_choice := &core.ArgChoice{
		Exclusivity: core.ArgChoiceExclusive,
		ChoiceArgsFn: func(_ *core.App, args map[string]any) []any {
			choices := []any{}
			if r, ok := args["selected"].(*core.Result); ok {
				if a, ok := r.Item.(Addon); ok {
					for _, su := range addon_releases(a) {
						choices = append(choices, su.Version)
					}
				}
			}
			return choices
		},
	}
	source_choice := &core.ArgChoice{
		Exclusivity: core.ArgChoiceExclusive,
		ChoiceArgsFn: func(_ *core.App, args map[string]any) []any {
			choices := []any{}
			if r, ok := args["selected"].(*core.Result); ok {
				if a, ok := r.Item.(Addon); ok {
					for _, sm := range a.SourceMapList {
						if sm.Source != a.Source || string(sm.SourceID) != a.SourceID {
							choices = append(choices, sm)
						}
					}
				}
			}
			return choices
		},
		LabelFn: func(v any) string {
			sm := v.(SourceMap)
			return fmt.Sprintf("%s %s", sm.Source, sm.SourceID)
		},
	}

	selection := core.ServiceInterface{ArgDefList: []core.ArgDef{selected_argdef()}}
	return core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "addon", Type: "service"},
		ServiceList: []core.Service{
			{
				ID: SERVICE_ID_CHECK_ADDON, Label: "Check for updates", Description: "Check online for updates without installing them.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(a Addon) bool { return should_check(a) == CHECK })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(CheckAddons(app, selected_results(args)), "failed to check for updates")
				},
			},
			{
				ID: SERVICE_ID_UPDATE_ADDON, Label: "Update", Description: "Download and install the update.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, Updateable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(UpdateAddons(app, selected_ids(args)), "failed to update")
				},
			},
			{
				ID: SERVICE_ID_REINSTALL_ADDON, Label: "Re-install", Description: "Install the installed version again.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, re_installable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(ReinstallAddons(app, selected_ids(args)), "failed to re-install")
				},
			},
			{
				ID: SERVICE_ID_ADDON_RELEASES, Label: "Releases", Description: "Install a particular release.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
					selected_argdef(),
					{ID: "release", Label: "Release", Widget: core.InputWidgetSelection, Choice: release_choice},
				}},
				Accepts: accepts_one[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(a Addon) bool { return !a.IsIgnored && !a.IsPinned && len(addon_releases(a)) > 0 })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					version := fmt.Sprint(arg(args, "release"))
					for _, id := range selected_ids(args) {
						if err := InstallAddonRelease(app, id, version); err != nil {
							return service_result(err, "failed to install release")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_SWITCH_SOURCE, Label: "Switch source", Description: "Get updates for this addon from another host.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
					selected_argdef(),
					{ID: "source", Label: "Source", Widget: core.InputWidgetSelection, Choice: source_choice},
				}},
				Accepts:    accepts_one[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, source_switchable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					sm, _ := arg(args, "source").(SourceMap)
					for _, r := range selected_results(args) {
						if err := SwitchSource(app, r, sm); err != nil {
							return service_result(err, "failed to switch source")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_PIN_ADDON, Label: "Pin", Description: "Keep this addon at its installed version.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, pinnable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(PinAddons(app, selected_results(args)), "failed to pin")
				},
			},
			{
				ID: SERVICE_ID_UNPIN_ADDON, Label: "Unpin", Description: "Allow this addon to be updated again.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, unpinnable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(UnpinAddons(app, selected_results(args)), "failed to unpin")
				},
			},
			{
				ID: SERVICE_ID_IGNORE_ADDON, Label: "Ignore", Description: "Leave this addon alone: never update, overwrite or remove it.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool { return any_item(selected, ignorable) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(IgnoreAddons(app, selected_results(args)), "failed to ignore")
				},
			},
			{
				ID: SERVICE_ID_STOP_IGNORING_ADDON, Label: "Stop ignoring", Description: "Manage this addon again.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(a Addon) bool { return a.IsIgnored })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(StopIgnoringAddons(app, selected_results(args)), "failed to stop ignoring")
				},
			},
			{
				ID: SERVICE_ID_STAR_ADDON, Label: "Star", Description: "Add this addon to your user catalogue.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(a Addon) bool { return a.CatalogueAddon != nil && !a.CatalogueAddon.Starred() })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, a := range selected_items[Addon](args) {
						if a.CatalogueAddon == nil {
							return service_result(fmt.Errorf("%s has no catalogue entry", a.Label), "failed to star")
						}
						if err := StarCatalogueAddon(app, *a.CatalogueAddon); err != nil {
							return service_result(err, "failed to star")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_UNINSTALL_ADDON, Label: "Uninstall", Description: "Remove this addon, including any directories it brought with it.",
				Interface: selection, Accepts: accepts_many[Addon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(a Addon) bool { return !a.IsIgnored })
				},
				Confirm: func(_ *core.App, args core.ServiceFnArgs) string {
					return fmt.Sprintf("Uninstall %s?\nTheir files are deleted.", selected_labels(args))
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(RemoveAddons(app, selected_results(args)), "failed to uninstall")
				},
			},
		},
	}
}

// returns the services for catalogue addons, the catalogue and the user catalogue.
func catalogue_services() core.ServiceGroup {
	catalogue_choice := &core.ArgChoice{
		Exclusivity: core.ArgChoiceExclusive,
		ChoiceFn: func(app *core.App) []any {
			choices := []any{}
			for _, cl := range FindSettings(app).CatalogueLocationList {
				choices = append(choices, cl.Name)
			}
			return choices
		},
	}
	selection := core.ServiceInterface{ArgDefList: []core.ArgDef{selected_argdef()}}
	return core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "catalogue", Type: "service"},
		ServiceList: []core.Service{
			{
				ID: SERVICE_ID_INSTALL_CAT_ADDON, Label: "Install", Description: "Install into the selected addons directory.",
				Interface: selection, Accepts: accepts_many[CatalogueAddon](),
				Applicable: func(app *core.App, _ []core.Result) bool { return can_install(app) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					app.DispatchAction(core.Action{Type: core.ACTION_SWITCH_TAB, Payload: TAB_LABEL_INSTALLED})
					return service_result(InstallCatalogueAddons(app, selected_items[CatalogueAddon](args)), "failed to install")
				},
			},
			{
				ID: SERVICE_ID_STAR_CAT_ADDON, Label: "Star", Description: "Add to your user catalogue.",
				Interface: selection, Accepts: accepts_many[CatalogueAddon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(ca CatalogueAddon) bool { return !ca.Starred() })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, ca := range selected_items[CatalogueAddon](args) {
						if err := StarCatalogueAddon(app, ca); err != nil {
							return service_result(err, "failed to star")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_UNSTAR_CAT_ADDON, Label: "Unstar", Description: "Remove from your user catalogue.",
				Interface: selection, Accepts: accepts_many[CatalogueAddon](),
				Applicable: func(_ *core.App, selected []core.Result) bool {
					return any_item(selected, func(ca CatalogueAddon) bool { return ca.Starred() })
				},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					for _, ca := range selected_items[CatalogueAddon](args) {
						if err := UnstarCatalogueAddon(app, ca); err != nil {
							return service_result(err, "failed to unstar")
						}
					}
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_SWITCH_CATALOGUE, Label: "Switch catalogue", Description: "Choose the catalogue to search and to match installed addons against.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{{
					ID: "catalogue", Label: "Catalogue", Widget: core.InputWidgetSelection, Choice: catalogue_choice,
					DefaultFn: func(app *core.App) string { return FindSettings(app).Preferences.SelectedCatalogue },
				}}},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					err := SwitchCatalogue(app, fmt.Sprint(arg(args, "catalogue")))
					if err == nil {
						if rerr := Reconcile(app); rerr != nil {
							slog.Debug("not matched after switching catalogue", "error", rerr)
						}
					}
					return service_result(err, "failed to switch catalogue")
				},
			},
			{
				ID: SERVICE_ID_REFRESH_USER_CAT, Label: "Refresh user catalogue", Description: "Bring the addons in your user catalogue up to date.",
				Fn: func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
					return service_result(RefreshUserCatalogue(app), "failed to refresh the user catalogue")
				},
			},
		},
	}
}

// parses a blank value as nil and anything else as a non-negative number of zips.
func parse_zips_to_keep(_ *core.App, val string) (any, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return (*int)(nil), nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("must be blank, to keep every zip, or a number of zips to keep: %q", val)
	}
	return &n, nil
}

// returns the zip files chosen in the file picker value `val`.
func chosen_files(val any) []string {
	switch v := val.(type) {
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

// returns the services that act on strongbox as a whole.
func app_services() core.ServiceGroup {
	return core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "app", Type: "service"},
		ServiceList: []core.Service{
			{
				ID: SERVICE_ID_INSTALL_FROM_FILE, Label: "Install addon from file", Description: "Install addons from zip files.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{{
					ID: "zipfiles", Label: "Zip files", Widget: core.InputWidgetFileSelection,
					FilePicker: &core.FilePickerOpts{Multiple: true, Extensions: []string{".zip"}, InitialDirFn: func(app *core.App) string {
						ad, _ := selected_addon_dir(app)
						return ad.Path
					}},
				}}},
				Applicable: func(app *core.App, _ []core.Result) bool { return can_install(app) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					zipfiles := chosen_files(arg(args, "zipfiles"))
					if len(zipfiles) == 0 {
						return service_result(fmt.Errorf("no zip files chosen"), "failed to install")
					}
					return service_result(InstallAddonsFromZips(app, zipfiles), "failed to install from file")
				},
			},
			{
				ID: SERVICE_ID_IMPORT_ADDON, Label: "Import addon", Description: "Install an addon from a GitHub, GitLab or WoWInterface URL.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{{
					ID: "url", Label: "URL", Widget: core.InputWidgetTextField, Parser: core.ParseStringStripWhitespace,
					Description: "https://github.com/owner/repository, https://gitlab.com/group/project or https://www.wowinterface.com/downloads/info12345",
				}}},
				Applicable: func(app *core.App, _ []core.Result) bool { return can_install(app) },
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					return service_result(InstallAddonFromURL(app, fmt.Sprint(arg(args, "url"))), "failed to import addon")
				},
			},
			{
				ID: SERVICE_ID_UPDATE_ALL, Label: "Update all", Description: "Update every addon that has an update.",
				Applicable: func(app *core.App, _ []core.Result) bool { return can_install(app) },
				Fn: func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
					return service_result(UpdateAll(app), "failed to update")
				},
			},
			{
				ID: SERVICE_ID_REFRESH, Label: "Refresh", Description: "Reload the addons directory and catalogue, and check addons for updates.",
				Fn: func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
					if ad, err := selected_addon_dir(app); err == nil && ad.Available() {
						if err := ReloadAddonsDir(app, ad); err != nil {
							slog.Warn("failed to reload addons directory", "error", err)
						}
					}
					background_refresh(app)
					return core.ServiceResult{}
				},
			},
			{
				ID: SERVICE_ID_PREFERENCES, Label: "Preferences", Description: "Change how strongbox behaves.",
				Interface: core.ServiceInterface{ArgDefList: []core.ArgDef{
					{ID: "addon-zips-to-keep", Label: "Downloaded zips to keep per addon", Widget: core.InputWidgetTextField, Parser: parse_zips_to_keep,
						Description: "blank keeps every zip",
						DefaultFn: func(app *core.App) string {
							if n := FindSettings(app).Preferences.AddonZipsToKeep; n != nil {
								return strconv.Itoa(*n)
							}
							return ""
						}},
					{ID: "keep-user-catalogue-updated", Label: "Keep the user catalogue updated", Widget: core.InputWidgetCheckbox, Parser: core.ParseTruthyFalseyAsBool,
						DefaultFn: func(app *core.App) string {
							return strconv.FormatBool(FindSettings(app).Preferences.KeepUserCatalogueUpdated)
						}},
					{ID: "check-for-update", Label: "Check for a newer strongbox at startup", Widget: core.InputWidgetCheckbox, Parser: core.ParseTruthyFalseyAsBool,
						DefaultFn: func(app *core.App) string { return strconv.FormatBool(FindSettings(app).Preferences.CheckForUpdate) }},
				}},
				Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
					zips, _ := arg(args, "addon-zips-to-keep").(*int)
					keep, _ := arg(args, "keep-user-catalogue-updated").(bool)
					check, _ := arg(args, "check-for-update").(bool)
					apply_settings(app, func(s Settings) Settings {
						s.Preferences.AddonZipsToKeep = zips
						s.Preferences.KeepUserCatalogueUpdated = keep
						s.Preferences.CheckForUpdate = check
						return s
					})
					return core.ServiceResult{}
				},
			},
		},
	}
}

// returns every service group strongbox offers.
func provider() []core.ServiceGroup {
	required_services := core.ServiceGroup{
		NS: core.NS{Major: "strongbox", Minor: "state", Type: "required"},
		ServiceList: []core.Service{
			core.StartProviderService(func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
				return service_result(Start(app), "failed to start provider")
			}),
			core.StopProviderService(func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
				Stop(app)
				return core.ServiceResult{}
			}),
		},
	}
	return []core.ServiceGroup{
		required_services,
		app_services(),
		catalogue_services(),
		addons_dir_services(),
		addon_services(),
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

func (sp *StrongboxProvider) Menu() []core.Menu {
	return []core.Menu{
		{Name: "File", MenuItemList: []core.MenuItem{
			{Name: "Install addon from file", ServiceID: SERVICE_ID_INSTALL_FROM_FILE},
			{Name: "Import addon", ServiceID: SERVICE_ID_IMPORT_ADDON},
			core.MENU_SEP,
			{Name: "New addons directory", ServiceID: SERVICE_ID_NEW_ADDONS_DIR},
			core.MENU_SEP,
			{Name: "Update all", ServiceID: SERVICE_ID_UPDATE_ALL},
		}},
		{Name: "View", MenuItemList: []core.MenuItem{
			{Name: "Refresh", ServiceID: SERVICE_ID_REFRESH},
		}},
		{Name: "Catalogue", MenuItemList: []core.MenuItem{
			{Name: "Switch catalogue", ServiceID: SERVICE_ID_SWITCH_CATALOGUE},
			{Name: "Refresh user catalogue", ServiceID: SERVICE_ID_REFRESH_USER_CAT},
		}},
		{Name: "Preferences", MenuItemList: []core.MenuItem{
			{Name: "Preferences", ServiceID: SERVICE_ID_PREFERENCES},
		}},
	}
}

// ---

func Provider(app *core.App) *StrongboxProvider {
	return &StrongboxProvider{}
}

// https://vitaneri.com/posts/implementing-map-filter-and-reduce-using-generic-in-go
func Map[T1, T2 any](s []T1, f func(T1) T2) []T2 {
	r := make([]T2, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}
