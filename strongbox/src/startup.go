package strongbox

import (
	"bw/core"
	"bw/http_utils"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// starting strongbox and refreshing it.
// installed addons are shown from local data straight away, everything that needs the
// network happens afterwards in the background.

// flags kept per app, app => name => flag. per app rather than per process, so apps
// started one after another, as in tests, never see each other's state.
var app_flags sync.Map

// returns the flag `name` of `app`, created unset.
func app_flag(app *core.App, name string) *atomic.Bool {
	flags, _ := app_flags.LoadOrStore(app, &sync.Map{})
	flag, _ := flags.(*sync.Map).LoadOrStore(name, &atomic.Bool{})
	return flag.(*atomic.Bool)
}

// set while a refresh runs, so a second is not started alongside it.
func refresh_running(app *core.App) *atomic.Bool { return app_flag(app, "refresh-running") }

// set once the self-update check has run for an app.
func self_update_checked(app *core.App) *atomic.Bool { return app_flag(app, "self-update-checked") }

// returns an error when strongbox is being run by the root user, `euid` 0.
func RefuseRoot(euid int) error {
	if euid == 0 {
		return errors.New("strongbox must not be run as the root user")
	}
	return nil
}

// matches the installed addons against the catalogue, downloading the catalogue first
// when it is stale, then checks them for updates, refreshes the user catalogue when due
// and, once per process, checks for a newer strongbox.
// a local catalogue is matched against before any download, so addons are identified
// even without the network.
// does nothing and returns `false` when a refresh is already running. never installs,
// updates or removes anything.
// clj: `core.clj/refresh`
func Refresh(app *core.App) bool {
	if !refresh_running(app).CompareAndSwap(false, true) {
		slog.Debug("a refresh is already running")
		return false
	}
	defer refresh_running(app).Store(false)

	job := app.StartJob("refreshing", 0)
	defer job.Finish()

	slog.Info("refreshing")

	// match against whatever catalogue is on disk first, without the network
	if cat_loc, err := current_catalogue_location(app); err == nil {
		local_path := CataloguePath(app, cat_loc.Name)
		if cat, err := read_catalogue_file(cat_loc, local_path); err == nil {
			catalogue_to_state(app, cat, read_user_catalogue(app.State().GetKeyVal("strongbox.paths.user-catalogue-file")))
			if err := Reconcile(app); err != nil {
				slog.Debug("not matched against the local catalogue", "error", err)
			}
		}
	}

	if err := LoadCatalogue(app); err != nil {
		slog.Warn("failed to load catalogue", "error", err)
	}
	if err := Reconcile(app); err != nil {
		slog.Info("installed addons not matched with the catalogue", "reason", err)
	}

	CheckForUpdates(app)

	settings := FindSettings(app)
	user := read_user_catalogue(app.State().GetKeyVal("strongbox.paths.user-catalogue-file"))
	if user_catalogue_refresh_due(user, settings.Preferences.KeepUserCatalogueUpdated, time.Now()) {
		slog.Info("the user catalogue is out of date, refreshing it")
		if err := RefreshUserCatalogue(app); err != nil {
			slog.Warn("failed to refresh the user catalogue", "error", err)
		}
	}

	if self_update_checked(app).CompareAndSwap(false, true) {
		CheckForStrongboxUpdate(app)
	}

	return true
}

// starts strongbox: fixes the paths in app state, creates the directories, loads the
// settings and the selected addons dir's addons from disk, then refreshes in the
// background, so starting never waits on the network.
// returns an error when strongbox is already running in this app, or when the
// directories cannot be created.
func Start(app *core.App) error {
	slog.Debug("starting strongbox")

	if app.State().GetKeyVal("app.name") == "strongbox" {
		return errors.New("only one instance of strongbox can be running at a time")
	}

	home := core.HomePath("")
	paths := GeneratePathMap(os.Getenv, home)
	set_paths(app, paths, GenerateV7Paths(os.Getenv, home))

	http_utils.SetUserAgent("strongbox", VERSION, PROJECT_URL)
	config := map[string]string{
		"app.name":    "strongbox",
		"app.version": VERSION,
		"app.about":   fmt.Sprintf("version: %s\n%s\nAGPL v3", VERSION, PROJECT_URL),
	}
	for key, val := range config {
		app.State().SetKeyAnyVal(key, val)
	}

	if err := init_dirs(app); err != nil {
		return err
	}

	// the selected addons dir's addons are read from disk as part of this
	LoadSettings(app)
	SaveSettings(app)

	// started here rather than in the goroutine, so the app is never idle in between
	job := app.StartJob("starting", 0)
	go func() {
		defer job.Finish()
		Refresh(app)
	}()

	slog.Debug("strongbox started", "config", config, "paths", paths)
	return nil
}

// stops strongbox. a refresh in progress is left to finish on its own.
func Stop(app *core.App) {
	slog.Debug("stopping strongbox")
}
