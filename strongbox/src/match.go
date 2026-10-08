package strongbox

import (
	"bw/core"
	"errors"
	"log/slog"
)

// matching installed addons to the catalogue.
// clj: `core.clj/db-match-installed-addon-list-with-catalogue`

// a rule for finding an installed addon in the catalogue: a key from the installed addon
// looked up in an index of the catalogue by the same kind of key.
type catalogue_matcher struct {
	name        string
	addon_key   func(Addon) string
	catalogue_k func(CatalogueAddon) string
}

// the rules, most reliable first. the first to find a match wins.
// strongbox 7 also matched an installed addon's source against a catalogue addon's name,
// which could only match by accident, and is gone.
var CATALOGUE_MATCHER_LIST = []catalogue_matcher{
	{"source and source ID",
		func(a Addon) string {
			if a.Source == "" || a.SourceID == "" {
				return ""
			}
			return a.Source + "/" + a.SourceID
		},
		func(ca CatalogueAddon) string { return ca.Key() }},
	{"name", func(a Addon) string { return installed_name(a) }, func(ca CatalogueAddon) string { return ca.Name }},
	{"label", func(a Addon) string { return installed_label(a) }, func(ca CatalogueAddon) string { return ca.Label }},
	{"directory name", func(a Addon) string { return a.DirName }, func(ca CatalogueAddon) string { return ca.Label }},
}

// returns the installed addon's own name: the nfo's, else its .toc's. matching must not
// use a name that came from an earlier match.
func installed_name(a Addon) string {
	if a.NFO != nil && a.NFO.Name != "" {
		return a.NFO.Name
	}
	if a.TOC != nil {
		return a.TOC.Name
	}
	if toc, err := a.Primary.SomeTOC(); err == nil {
		return toc.Name
	}
	return ""
}

// returns the installed addon's own label, from its .toc.
func installed_label(a Addon) string {
	if a.TOC != nil {
		return a.TOC.Label
	}
	if toc, err := a.Primary.SomeTOC(); err == nil {
		return toc.Label
	}
	return ""
}

// returns `list` indexed by `keyfn`. when several share a key, the first wins: catalogue
// order decides between equally good matches.
func index_first[T any](list []T, keyfn func(T) string) map[string]T {
	idx := map[string]T{}
	for _, item := range list {
		key := keyfn(item)
		if key == "" {
			continue
		}
		if _, present := idx[key]; !present {
			idx[key] = item
		}
	}
	return idx
}

// returns the catalogue addon matching `a`, the name of the rule that matched, and `true`,
// or `false` when nothing matches. ignored addons are never matched.
func match_addon(a Addon, idx_list []map[string]CatalogueAddon) (CatalogueAddon, string, bool) {
	if a.IsIgnored {
		return CatalogueAddon{}, "", false
	}
	for i, matcher := range CATALOGUE_MATCHER_LIST {
		key := matcher.addon_key(a)
		if key == "" {
			continue
		}
		if ca, present := idx_list[i][key]; present {
			return ca, matcher.name, true
		}
	}
	return CatalogueAddon{}, "", false
}

// returns the catalogue indexed once per matcher.
func matcher_indexes(db []CatalogueAddon) []map[string]CatalogueAddon {
	idx_list := []map[string]CatalogueAddon{}
	for _, matcher := range CATALOGUE_MATCHER_LIST {
		idx_list = append(idx_list, index_first(db, matcher.catalogue_k))
	}
	return idx_list
}

// returns each addon in `addon_list` composed again with its catalogue match, or with no
// match when none is found, keeping the updates already found. also returns how many
// were matched.
func match_addon_list(db []CatalogueAddon, addon_list []Addon) ([]Addon, int) {
	idx_list := matcher_indexes(db)
	matched := 0
	out := make([]Addon, 0, len(addon_list))
	for _, a := range addon_list {
		var ca_ptr *CatalogueAddon
		if ca, rule, ok := match_addon(a, idx_list); ok {
			ca_ptr = &ca
			matched++
			slog.Debug("matched addon to catalogue", "addon", a.Label, "rule", rule, "catalogue-addon", ca.Label)
		} else if !a.IsIgnored {
			slog.Info("addon not found in the catalogue", "addon", a.Label, "dir", a.DirName)
		}
		out = append(out, MakeAddon(*a.AddonsDir, a.InstalledAddonGroup, a.Primary, a.NFO, ca_ptr, a.SourceUpdateList))
	}
	return out, matched
}

// matches the installed addons in the selected addons dir against the loaded catalogue,
// updating them in place in app state.
// returns an error when no addons dir is selected or no catalogue is loaded.
// clj: `core.clj/match-all-installed-addons-with-catalogue`
func Reconcile(app *core.App) error {
	addons_dir, err := selected_addon_dir(app)
	if err != nil {
		return errors.New("failed to match addons with the catalogue: no addons directory selected")
	}
	cat, ok := loaded_catalogue(app)
	if !ok {
		return errors.New("failed to match addons with the catalogue: no catalogue loaded")
	}

	result_list := installed_addons(app, addons_dir)
	addon_list := core.ItemList[Addon](result_list...)
	matched_list, num_matched := match_addon_list(cat.AddonSummaryList, addon_list)
	slog.Info("matched installed addons with the catalogue", "matched", num_matched, "installed", len(addon_list))

	update_idx := map[string]Addon{} // result ID => matched addon
	for i, r := range result_list {
		update_idx[r.ID] = matched_list[i]
	}
	replace_addons_in_state(app, update_idx)
	return nil
}

// replaces the addons in the results named in `update_idx`, result ID => addon, in place,
// retagging each, in one state update.
func replace_addons_in_state(app *core.App, update_idx map[string]Addon) {
	if len(update_idx) == 0 {
		return
	}
	app.UpdateState(func(old_state core.State) core.State {
		result_list := old_state.GetResults()
		for i, r := range result_list {
			if a, present := update_idx[r.ID]; present {
				r.Item = a
				result_list[i] = tag_addon_result(r)
			}
		}
		return old_state
	}).Wait()
}
