package strongbox

import (
	"bw/core"
	"errors"
	"fmt"
	"log/slog"
)

// uninstalling addons.
// clj: `core.clj/remove-many-addons`

var ErrIgnored = errors.New("addon is ignored")

// uninstalls the addon in the result `r`, every directory of it, then reloads its addons
// dir. an ignored addon is refused.
func RemoveAddon(app *core.App, r *core.Result) error {
	a, ok := r.Item.(Addon)
	if !ok || a.AddonsDir == nil {
		return fmt.Errorf("not an installed addon: %s", r.ID)
	}
	if a.IsIgnored {
		return fmt.Errorf("refusing to remove %s: %w", a.Label, ErrIgnored)
	}
	err := func() error {
		unlock := lock_addons_dir(a.AddonsDir.Path)
		defer unlock()
		return remove_addon(a, *a.AddonsDir)
	}()
	if rerr := ReloadAddonsDir(app, *a.AddonsDir); rerr != nil {
		slog.Warn("failed to reload addons dir after removing addon", "error", rerr)
	}
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w", a.Label, err)
	}
	slog.Info("removed addon", "addon", a.Label)
	return nil
}

// uninstalls each of the addons in `result_list`. ignored addons are skipped and
// reported, the rest are removed. returns the failures and refusals joined.
func RemoveAddons(app *core.App, result_list []*core.Result) error {
	err_list := []error{}
	for _, r := range result_list {
		if err := RemoveAddon(app, r); err != nil {
			slog.Warn("addon not removed", "error", err)
			err_list = append(err_list, err)
		}
	}
	return errors.Join(err_list...)
}
