package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sourcegraph/conc/pool"
)

// checking installed addons for updates at their hosts.
// clj: `core.clj/check-for-updates`, `check-for-updates-in-parallel`

// how many addons are checked at once. requests are also limited globally, see
// `http_utils.HTTPSem`.
const UPDATE_CHECK_CONCURRENCY = 8

// returns the message telling the user no release was found for the addons dir `ad` on
// `source`, naming the game tracks searched.
// "no 'Classic (TBC)', 'Classic (WotLK)' or 'Classic' release found on github."
// clj: `catalogue.clj/expand-summary`
func no_release_message(ad AddonsDir, source Source) string {
	pref_list := []GameTrackID{ad.GameTrackID}
	if !ad.Strict {
		pref_list = GAMETRACK_PREF_MAP[ad.GameTrackID]
	}
	quoted := []string{}
	for _, gt := range pref_list {
		quoted = append(quoted, "'"+GameTrackLabel(gt)+"'")
	}
	names := quoted[0]
	if len(quoted) > 1 {
		names = strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
	}
	return fmt.Sprintf("no %s release found on %s.", names, source)
}

// the outcome of deciding whether to check an addon.
type check_decision int

const (
	CHECK            check_decision = iota
	SKIP_IGNORED                    // the user said to leave it alone
	SKIP_NO_SOURCE                  // nothing says where it came from
	SKIP_UNSUPPORTED                // it came from a host strongbox cannot use
)

// returns whether the addon `a` should be checked for updates.
func should_check(a Addon) check_decision {
	switch {
	case a.IsIgnored:
		return SKIP_IGNORED
	case a.Source == "" || a.SourceID == "":
		return SKIP_NO_SOURCE
	case !SUPPORTED_HOSTS.Contains(a.Source):
		return SKIP_UNSUPPORTED
	}
	return CHECK
}

// sets or clears the busy mark on the result `id`, waiting for it to apply.
func set_busy(app *core.App, id string, busy bool) {
	app.UpdateResult(id, func(r core.Result) core.Result {
		if busy {
			r.Tags.Add(core.TAG_BUSY)
		} else {
			r.Tags.Remove(core.TAG_BUSY)
		}
		return r
	}).Wait()
}

// checks the addon in the result `id` for updates, marking it busy while it does.
// the addon is composed again with the updates found, from its state at the time of
// writing, never a stale copy. a failure is logged at WARN for that addon and leaves it
// as it was. returns an error when the check failed.
func check_addon(app *core.App, id string) error {
	r := app.GetResult(id)
	if r == nil {
		return fmt.Errorf("addon is gone: %s", id)
	}
	a, is_addon := r.Item.(Addon)
	if !is_addon || a.AddonsDir == nil {
		return fmt.Errorf("not an installed addon: %s", id)
	}

	switch should_check(a) {
	case SKIP_IGNORED, SKIP_NO_SOURCE:
		return nil
	case SKIP_UNSUPPORTED:
		slog.Info("addon is from an unsupported source and cannot be checked for updates", "addon", a.Label, "source", a.Source)
		return nil
	}

	set_busy(app, id, true)
	source_update_list, err := ExpandSummary(app, a.Source, expand_request(a))
	if err != nil {
		set_busy(app, id, false)
		slog.Warn("failed to check addon for updates", "addon", a.Label, "source", a.Source, "error", err)
		return err
	}

	var updated Addon
	app.UpdateResult(id, func(x core.Result) core.Result {
		current := x.Item.(Addon)
		updated = MakeAddon(*current.AddonsDir, current.InstalledAddonGroup, current.Primary, current.NFO, current.CatalogueAddon, source_update_list)
		x.Item = updated
		x.Tags.Remove(core.TAG_BUSY)
		return tag_addon_result(x)
	}).Wait()

	if updated.SourceUpdate == nil && len(source_update_list) > 0 {
		slog.Info(no_release_message(*updated.AddonsDir, updated.Source), "addon", updated.Label)
	}
	return nil
}

// checks the addons in the results `id_list` for updates, several at once, as a job.
// every addon is checked even when some fail. returns the errors joined.
func check_addons(app *core.App, id_list []string) error {
	job := app.StartJob("checking for updates", len(id_list))
	defer job.Finish()

	p := pool.NewWithResults[error]().WithMaxGoroutines(UPDATE_CHECK_CONCURRENCY)
	for _, id := range id_list {
		p.Go(func() error {
			defer job.Tick(1)
			return check_addon(app, id)
		})
	}
	return errors.Join(p.Wait()...)
}

// checks every installed addon in the selected addons dir for updates.
// an addon is checked from where it was installed, or where it was matched in the
// catalogue, so match first. blocks until every addon has been checked.
func CheckForUpdates(app *core.App) {
	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		slog.Warn("no addons directory selected, not checking for updates")
		return
	}
	id_list := []string{}
	for _, r := range installed_addons(app, addons_dir) {
		id_list = append(id_list, r.ID)
	}
	slog.Info("checking addons for updates", "num-addons", len(id_list))
	check_addons(app, id_list)
}

// checks the addons in `result_list` for updates.
func CheckAddons(app *core.App, result_list []*core.Result) error {
	id_list := []string{}
	for _, r := range result_list {
		id_list = append(id_list, r.ID)
	}
	return check_addons(app, id_list)
}
