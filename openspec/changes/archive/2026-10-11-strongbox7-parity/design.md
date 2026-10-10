## Context

See `proposal.md` for motivation and scope, and the specs under `specs/` for required behaviour. This document covers how to get there from the current code.

**Strongbox provider (current state)**
- `strongbox/src/provider.go` declares about 25 services. About half have no `Fn`.
- The context-menu map (`ItemHandlerMap`) is written by hand.
- Menu items call a `donothing` stub.

**Startup, loading and identity**
- `Start` calls `Refresh` synchronously: catalogue download, matching and every update check. Provider start, and therefore `main_gui`, waits on the network.
- Installed addons are loaded by two paths: `AddonsDir.ItemChildren` (lazy, on expand) and `LoadAllInstalledAddonsToState`. Both give each addon a fresh `core.UniqueID()`, so reloading duplicates rows and updates cannot target a row reliably.

**Settings**
- Settings decode straight into a Go struct, so unknown and legacy keys vanish silently.
- A read failure falls back to defaults, which are then saved over the file.
- Paths are derived twice, in `strongbox/main.go` and in `strongbox/src/core.go`. Both append `strongbox` rather than `strongbox8` when `XDG_*_HOME` is set.

**Boardwalk**
- `bw/http_utils.FileCachingRequest` never expires entries (`if true || ...`) and caches to `/tmp`.
- `http_utils.DownloadFile` writes the destination file before checking the status, and bypasses `app.HTTPClient`, so a test transport cannot intercept it.
- `bw/ui/gui_form.go` renders only text fields and directory pickers. `InputWidgetSelection`, which strongbox already uses, logs an error.
- There is no status bar and no job concept. `GUIUI.WaitForServices` waits for services only.

**Tests**
- `go test -timeout=5s` in `manage.sh` limits each package's whole test binary to 5 s, not each test.
- `strongbox/main_test.go` hits the real network ("the envvars above are not preventing the catalogue from loading").

## Goals / Non-Goals

**Goals:**
- Every service in `provider()` has an implementation, or is deleted.
- Every v7 business-as-usual behaviour in scope has a Go test, as a unit test where the logic is pure and an integration test where it crosses the GUI.
- Strongbox logic is restructured so decisions are pure functions over data (plan) and side effects are thin (execute). Most v7 test cases then become table tests without a filesystem or network.
- Boardwalk gains only generic mechanisms: service declarations, confirmation, widgets, jobs, HTTP. Nothing in `bw/` mentions addons, catalogues or game tracks.
- The design leaves room for later speed work: bounded worker pools, an incremental reconcile, a different store for installed addon state.

**Non-Goals:**
- Performance tuning beyond not blocking on the network.
- A properties pane or addon detail pane. The boardwalk "Properties" context entry stays as it is.
- Theme switching, column presets, a log pane, Cache-menu housekeeping, and importing or exporting addon lists.
- Changing the catalogue format or the strongbox-catalogue repository.
- Concurrent installs within a single addons dir.

## Decisions

### D1. One path derivation, `strongbox8` everywhere

**Decision**
- A pure `GeneratePathMap(getenv, home)` takes the environment lookup and home directory as input. It is exported for `main.go`, which needs the data dir before the GUI installs its Tcl scripts.
- The duplicate `xdg_path` in `main.go` is deleted.
- The application directory name becomes a single constant, `strongbox8`, applied to the defaults and to `XDG_*_HOME`.
- A sibling function, `GenerateV7Paths`, derives strongbox 7's paths for the one-time import.

**Why:** the user chose separate directories. Today an `XDG_CONFIG_HOME` user's v8 writes into v7's directory, which is the exact collision to avoid.

**Alternatives:** reading `app.config-dir` set by `main.go` (the current split). Rejected because the two copies have already diverged once.

### D2. Settings: strict structs per format, one explicit conversion

This follows the same principle as `MakeAddon`:
- each input is a strict struct;
- a single function builds the result, choosing every field explicitly from named inputs;
- there is no generic deep merge, and no step can leave fields half-moved.

```
bytes ──decode──▶ settings_file ──probe──▶ format ──settings_from_v7 | settings_from_v8──▶ Settings ──validate──▶ (Settings, []Issue)
```

**The input struct and the conversions**
- **`settings_file`** is one struct holding the superset of every key any v7 or v8 version wrote:
  - top level: `install-dir`, `selected-catalog`, `selected-catalogue`, `selected-addon-dir`, `gui-theme`, `debug?`, `spec`, `addon-dir-list` (with `strict?` and `strict`), `catalogue-location-list`, `preferences`;
  - each value is wrapped in `lenient[T]`, which records whether the key was present and keeps the raw JSON of a wrongly typed value, so "absent" differs from "false" or "empty", and one bad value never fails the whole decode.

  Fourteen historical v7 shapes and the v8 pre-release shape are all subsets of this one struct, so no per-version types are needed.
  Implementation note: separate `SettingsV7` and `Settings` input decodes were planned. One lenient superset turned out simpler, because the format probe needs the same fields the conversions do.
- **`Settings`** (v8) is the current struct with `spec.version`. Its `Deprecated*` fields are removed.
- **Format probe:** `probe_settings_format` reads the decoded `settings_file`. A file with `spec.version` is v8. A v8 pre-release file has `strict` and `preferences.selected-addon-dir` but no version, and is treated as v8 version 0.
- **`settings_from_v7(settings_file) (Settings, []Issue)`** is the single place v7 values map to v8 fields. It is written as one function in the order of the migrations table in `specs/strongbox/settings`, each target field assigned once from named sources. Examples: `selected-catalogue` falls back to `selected-catalog`; `strict` comes from `strict?`, or from compound-game-track detection, or defaults to true.
- **`settings_from_v8(settings_file) (Settings, []Issue)`** handles v8 versions (currently 0 to 1). Further format versions add a case here.
- An `Issue` carries a level (DEBUG or WARN) and a message, and is logged by the caller. The conversion functions never log, which keeps them pure and testable.

**Unknown keys**
- They are reported, not carried. After decoding, the top-level key names of the file are compared against the input struct's known JSON names using one shallow `map[string]json.RawMessage` decode used only for names. Unknown names become DEBUG issues.
- Unknown keys are not preserved on save.

**Validation and write rules**
- Validation is per entry. A bad addons dir or catalogue location is removed with a WARN issue. A bad preference reverts to its default. One bad value never discards the whole file, unlike v7.
  - Wrong-typed preference values must not fail the whole decode. Preference fields that v7 typed loosely (`addon-zips-to-keep`) are decoded through a small tolerant type that records an issue instead of returning a decode error.
- An addons dir that does not exist is **not** a validation failure. Availability is computed at runtime (D4) and is not stored.
- v8 writes `"spec": {"version": 1}`. A higher version sets `settings_read_only` in state, and `SaveSettings` then becomes a no-op that logs once.
- Saving goes through `write_atomic(path, bytes)`: a temp file in the same directory, `fsync`, then `rename`. The settings file, the user catalogue and nfo files all share this helper.
- An unparseable settings file is copied to `config.json.<UTC timestamp>.invalid` before defaults are used.

**One-time import**
- If `config.json` is missing in the v8 config dir, `load_settings` reads the v7 file (if present) as the input bytes.
- If `user-catalogue.json` is missing, the v7 user catalogue is copied byte for byte.
- Both reads are logged at INFO. The v7 directory is opened read-only.

**Alternatives considered**
- **A pipeline of steps over `map[string]any`.** Rejected because it reproduces the ambiguity of v7's deep-merging: intermediate states are untyped, and a key can be half-migrated between steps.
- **The current single struct with `Deprecated*` fields.** Rejected because it mixes input and output shapes in one type, so a deprecated value can be written back.

**Tests**
- Table tests over the existing fixtures in `strongbox/src/testdata/config/`.
- Property tests:
  - `settings_from_v7` output, saved and read back through `settings_from_v8`, is unchanged;
  - save-then-load is the identity;
  - loading any fixture twice (load, save, load) gives the same `Settings`.

### D3. Game tracks as data

**Decision**
- Add `classic-mists` and `forever` to `SUPPORTED_GAME_TRACKS`, the alias map, the labels and `GAMETRACK_PREF_MAP`.
- Map the interface ranges `{16000,20000}` and `{50000,60000}` to them.
- Add `forever|camelot` and `mists` patterns at the head of `GAME_TRACK_PATTERN_LIST`.
- Add `Mists` and `Camelot` `.toc` file-name suffixes.

**Why:** the mappings are already data (ranges, ordered pattern list, preference map), so adding a game track means adding rows. The modified specs list the exact rows.

**Agreement with the catalogue producers:** `github-wow-addon-catalogue` and `strongbox-catalogue-builder-go` are the source of truth for what v8 reads:
- game track names: `forever`, `classic-mists`, with `camelot` as Forever's codename;
- `.toc` file-name suffixes: `Mainline`, `Vanilla`, `Classic`, `Forever`, `Camelot`, `TBC`, `BCC`, `Wrath`, `WOTLK`, `WOTLKC`, `Cata`, `Mists`;
- flavour aliases: `mop`, `cataclysm`, `wotlkc`;
- catalogue fields that may be omitted: `download-count`, `tag-list`, `description`, `created-date`.

v8's tables are checked against theirs in a unit test that lists the producers' values verbatim. A future producer change then fails loudly rather than silently discarding data.

**Guessing from paths:** `guess_game_track_from_path` applies the same guesser to the WoW client directories in the path (`_retail_`, `_classic_era_` and the like), innermost first, and returns retail when none names a game track. Ordinary directory names are not considered: guessing from every segment found game tracks in names like `/home/abc`.

### D4. Installed addon state: one loader, deterministic IDs

**Decision**
- `addons_dir_results(ad AddonsDir) ([]core.Result, error)` is the single place addon results are built. Both `AddonsDir.ItemChildren` and an explicit reload (`ReloadAddonsDir`) use it.
- Result IDs are `"addon:<addons-dir-path>#group:<group-id>"`, or `"addon:<addons-dir-path>#dir:<dirname>"` for an addon without a group ID. A child result's ID is scoped by its parent's ID, its directory name and its `.toc` file name.
- Reloading an addons dir replaces its children in **one** `UpdateState`: remove the children of the addons dir result, then add the new list. This is atomic for observers and avoids flicker and duplicates.
- Availability (`DirExists`) is evaluated when settings are loaded into state and on refresh. It is stored on the `AddonsDir` item as a derived field and never persisted.

**Why:** stable IDs let update checks, busy marks and installs target rows with `UpdateResult`. Today `CheckForUpdates` captures a result by value and writes a stale copy back.

**Data structure:** an addon group is a **map** from group ID to the list of member `InstalledAddon`s, built in `group_installed_addons`. The primary is chosen by a pure function, `pick_primary`, with an explicit tie-break (lowest directory name). This replaces "first in map iteration order", which is non-deterministic.

**Single composition point:** `MakeAddon` remains the one function that builds an `Addon` from its strict parts (installed addons, nfo, `.toc`, catalogue addon, updates, addons dir). Every derived field this change adds is assigned there, from named inputs, and nowhere else:
- implicit ignore;
- pinned version;
- the source map list;
- matched details.

The combined version is display text, not addon data. `combined_version(a)` computes it from a finished `Addon` for `Addon.ItemMap`.

Other code reads an `Addon`; it never patches one. Re-evaluation after a game track change, a match or an update check means calling `MakeAddon` again with the new parts.

**Implicit ignore:** it is computed in `MakeAddon` from three inputs: the nfo `Ignored` pointer, `version_controlled(dir)` for any member, and `TOC.Ignored` from any `.toc` of any member. The nfo value overrides only when it is non-nil. The VCS check is fixed: today it applies only when `Ignored` is already set, which inverts it.

### D5. Plan/execute for filesystem operations

Install splits into:
- **`plan_*`**: a pure function from loaded state (the `[]Addon` in the addons dir, a `ZipReport`, options) to a plan value, or a refusal error;
- **`execute_*`**: applies the plan to disk.

Uninstall, ignore, pin and source switching need no separate plan. Each is a pure predicate (`check_removable`, `ignorable`, `pinnable`, `unpinnable`, `source_switchable`) followed by one execution step (`remove_addon`, `edit_addon_nfo`).

```
plan_install(installed []Addon, target Addon, report ZipReport, opts InstallOpts) (InstallPlan, error)

InstallPlan {
  Uninstall  []Addon            // previous version + completely overwritten addons
  NFOWrites  map[string]NFOFile // final nfo data per top-level dir
  Primary    string
  Messages   []string           // "x replaced directory y of addon z", suspicious bundle
}
```

- **Plan contents:** a plan is a value. The zip to extract is passed to `execute_install` beside the plan. Zips to prune are computed after the install by `zips_to_prune`. The refusal rules (invalid zip, overwriting ignored or pinned, path traversal) and the stacking rules for mutual dependencies are all decided in `plan_install`, so the v7 install and mutual-dependency tests become table tests over in-memory `Addon` and `ZipReport` values.
- **Execution order:** `execute_install` aborts on the first failed step. The previous behaviour (log and continue after a failed unzip, then write nfo data anyway) is removed. That fixes the ISSUES.md entry.
- **Locking:** a per-addons-dir `sync.Mutex`, held in a `map[path]*sync.Mutex` guarded by its own mutex, serialises execution within an addons dir. Downloads happen outside the lock. Finer-grained (per-directory) locks can replace this later without changing the plan/execute split.
- **Removal safety:** removal uses `os.Lstat` to refuse symbolic links. It compares paths with `filepath.Rel` (a result must not start with `..` and must not be `.`) rather than `strings.HasPrefix`, which wrongly accepts `/AddOnsX` as inside `/AddOns`.
- **Nil nfo:** `remove_addon` no longer dereferences a nil `NFO`. An unmanaged addon removes its own directory only.

### D6. Startup and refresh as a background, single-flight job

**Startup**

```
Start(app)
  paths → init_dirs → load/import/migrate settings → save (unless read-only)
  load selected addons dir from disk  (synchronous, local only)
  go refresh(app)                      (returns immediately)
```

**`refresh(app)`**
- It is guarded by an `atomic.Bool`, so a second call while one runs returns at once.
- It runs as a tracked job (D9), in this order:
  1. load the local catalogue if present;
  2. load the user catalogue, combine it with the selected catalogue by whole entries, reconcile;
  3. download the catalogue if stale; if it changed, reload and reconcile;
  4. check for updates. Each addon is marked busy and checked in a `conc` pool bounded at 8. The global `HTTPSem` of 50 still applies;
  5. scheduled user-catalogue refresh;
  6. self-update check, once per process.

**Why a goroutine inside `Start` rather than a boardwalk "after start" hook:** boardwalk already treats provider start as opaque. The job mechanism (D9) gives visibility, so no new lifecycle hook is needed.

**Matching**
- The ordered rules live in a slice of matchers, which is data, as today. The defective `source == catalogue name` matcher is removed.
- `core.Index` is last-wins, but the spec requires first-in-catalogue-order. A first-wins `index_first` is used instead, and `core.Index` is left unchanged for its other callers.
- Ignored addons skip matching.

### D7. Hosts behind one interface, keyed by source

```go
type AddonSource interface {
    ParseURL(url string) (source_id string, ok bool)
    FindAddon(app *core.App, source_id string) (CatalogueAddon, error)
    ExpandSummary(app *core.App, req ExpandRequest) ([]SourceUpdate, error)
}
type ExpandRequest struct {
    SourceID        string
    KnownGameTracks mapset.Set[GameTrackID] // catalogue game-track-list, else nfo installed game track
}
var SOURCE_MAP = map[Source]AddonSource{ github: …, gitlab: …, wowinterface: … }  // a map: source → host behaviour
```

- **Why `KnownGameTracks`:** the WoWInterface API carries no game track. The catalogue does. Passing it in keeps the host pure with respect to state, and fixes the "every WoWInterface update is retail" issue.
- **GitLab:** ported from `gitlab_api.clj` using v7's fixtures. v7's two `juxt` filters always passed. They are implemented as written in the spec (external links excluded, link types `package`/`other`).
- **GitHub:** gains `FindAddon` (releases, then `release.json`, then root `.toc` via the contents API) and `GITHUB_TOKEN`. If no game track can be determined, the addon is not found (strictness), instead of v7's assumed retail.
- **WoWInterface:** `FindAddon` looks up the loaded catalogue only, as in v7.

### D8. Boardwalk service declarations replace `ItemHandlerMap`

`core.Service` gains:

```go
Accepts    *Accepts                                  // nil: not offered in context menus
Applicable func(app *App, selected []Result) bool    // nil: always
Confirm    func(app *App, args ServiceFnArgs) string // nil: no confirmation; "" from the fn also skips
type Accepts struct { Types []reflect.Type; Many bool }
```

- **Derived map:** `App.TypeMap` is derived from the services when providers start. `Provider.ItemHandlerMap` is removed from the interface. The bw filesystem provider and strongbox migrate to `Accepts`.
- **Context menus:** entries are built from the derived map. Each is enabled when `Applicable(selected)` is true. A selection of mixed types keeps today's grouping by type.
- **Menu items:** a menu item bound to a `ServiceID` is re-evaluated with `Applicable(app, nil)` in the Tk menu's `-postcommand`, so menus need no rebuild on state change.
- **Selection argument:** the argument boardwalk fills from the selection is marked `ArgDef.FromSelection`. A service whose remaining args are all `FromSelection` runs immediately. The current check, `len(ArgDefList) == 1`, cannot express that.
- **No `Fn`:** services without `Fn` are filtered out of menus and context menus. Strongbox placeholders are either implemented or deleted.
- **Panics:** `CallServiceFnWithArgs` turns a panicking service into a failed result. Before, the recovered value was assigned to a local variable that was never returned, so a panic reported success.
- **Confirmation:** confirmation goes through `GUIUI.Confirm(title, message) bool`, a replaceable function field. It defaults to `tk_messageBox -type yesno`. Integration tests replace it to answer.

**Alternatives:** keep `ItemHandlerMap` and add a predicate map beside it. Rejected because it would be two hand-maintained maps keyed by service ID, the drift `provider.go` already notes as a TODO.

### D9. Background jobs and busy rows in boardwalk

**Jobs**
- Jobs live in a mutex-guarded store on `App` as a `map[job_id]JobInfo{Name, Done, Total, Started}`, a map keyed by job ID.
- They are kept out of `State` because `UpdateState` deep-clones the whole state on every call, which is too expensive per progress step.
- Every change queues an `ACTION_JOBS_CHANGED` action through the update channel, so it is ordered with state updates. Observers read the current jobs with `App.Jobs()`.
- `app.StartJob(name, total) *Job` returns a handle with `Tick(n)` and `Finish()`. `Finish` is always deferred by callers.

**Status bar and busy rows**
- The GUI adds a status bar at the bottom of the main window, driven by `OnResultsChanged`/`OnJobsChanged`.
- It shows "idle", or each job as `name (done/total)`.
- `core.TAG_BUSY` marks rows. The tablelist styling gives busy rows a distinct foreground.

**State access**
- `App.State` is an `atomic.Pointer[State]` read through `App.State()`. `process_update` builds the new state and its index, then stores the pointer, so a reader never sees a half-built state. Previously every reader dereferenced the field unlocked while the update loop replaced it.
- `State.KeyVals` is a `*KeyValStore` with its own lock. Every state copy shares one store, as they always shared one map, and `SetKeyAnyVal` from a goroutine could previously crash the process with a concurrent map write.
- Both were found running the GUI workflow under `-race`.

**`WaitForIdle`**
- `GUIUI.WaitForIdle()` replaces `WaitForServices` in tests.
- It loops until services are finished, jobs are empty, no children are loading, and an `ACTION_FLUSH` action has passed through the update channel (`App.Flush`, proving the queue is drained). It finishes with a `TkSync` so the GUI has applied the changes.

### D10. HTTP: one transport seam, expiring cache, atomic downloads

**Changes to the HTTP layer**
- The cache directory is set from the app's data dir: `app.SetDataDir` reconfigures `FileCachingRequest.CWD` to `<data>/cache`.
- The `if true ||` short-circuit is removed. The default expiry is 1 hour, and per-request-type overrides stay possible.
- An entry's age is its file's modification time. Entries are stamped with the transport's own clock (`FileCachingRequest.Now`) when written, so an injected clock and the files agree.
- `PruneCache(dir, max_age, now)` deletes expired entries when `app.SetDataDir` sets the cache directory.
- `DownloadFile` is reimplemented on `app.HTTPClient` (not `http.Get`). It streams to `<dest>.part` in the same directory, checks the status before writing, and renames into place. The downloaded files themselves are not cached.
- The User-Agent is `strongbox/<version> (https://github.com/ogri-la/strongbox)`. The version comes from strongbox's own `VERSION` variable, set by `-ldflags` and defaulting to `8.0.0-unreleased`, which the self-update check also compares against. `core.VERSION` stays boardwalk's. The `8.0.0-unreleased` hard-coded in `Start` is replaced.

**Test seam**
- `FixtureTransport` is an `http.RoundTripper` with a URL-to-response map and an `Unrouted []string` record.
- Integration tests install it as `app.HTTPClient.Transport` before providers start, via a `main_gui(opts)` parameter. This covers API calls and file downloads through one seam.
- The existing `MapDownloader` stays for unit tests.

### D11. nfo compatibility

- **Write:** `write_nfo_file` marshals a single `NFO` when the list has one element, and an array otherwise. It validates each element with the existing zog schema (extended to the game-track enum) before writing atomically.
- **Read:** `read_nfo_file` no longer treats a one-element array specially except to normalise it. `is_mutual_dependency` stays `len > 1`, which is v7's meaning. Invalid files are logged and treated as absent, never deleted. This is a recorded behaviour change from v7.
- **Ignore editing:** ignore-only nfo objects are a distinct shape. Today `NFO.GroupID` is required by `write_nfo` and `IsEmpty`. A small sum type is introduced:
  - `NFO` (full or grouping-only, has a group ID);
  - `IgnoreFlag` (only `ignore?`).

  It is read and written through one `NFOFile` value, holding either `Stack []NFO` or `IgnoreFlag *bool`. This keeps the strict "no empty nfo" rule while supporting ignoring unmanaged addons.

### D12. Strongbox GUI wiring

**Services and forms**
- Each row of the context-menu table in `specs/strongbox/gui` becomes a service with `Accepts` and `Applicable`. The predicates reuse the pure rules (`Updateable`, `pinnable`, `ignorable`, `re_installable`, `len(SourceMapList) >= 2`).
- "Set game track", "Switch catalogue", "Releases" and "Switch source" use choice-list arguments whose `ChoiceFn` reads state.
- "Preferences" is one service with a checkbox per boolean preference and a validated integer-or-blank field for `addon-zips-to-keep`.
- "Browse" runs `xdg-open <dir>` detached. Failure is reported at WARN.

**Columns**
- Column titles map from `ui-selected-columns` names through `COL_KEY_MAP`: v7 names such as `tag-list` and `dirsize` map to `Addon.ItemMap` keys (`tags`, `size`). `InstalledColumnKeys` gives every offered column in display order, and `SelectedColumnKeys` the ones to show.
- `combined-version` and `source-id` are added to `Addon.ItemMap`. The combined version is the available version when there is an update, otherwise the installed version, prefixed with `(ignored)` or `(pinned)`.
- Unknown names are ignored. The debugging `ns` column is no longer forced on.

**Search**
- The search filter in `main.go` becomes case-insensitive substring over label, name and description. The current check is `name == needle || desc contains`.
- Catalogue addons matched to an addon in the selected addons dir are marked installed, shown in an `installed` column. `mark_installed` is a pure pass over the result list, run after every match (`Reconcile`) and every reload of the addons dir, so an uninstall clears the mark.

### D13. Testing approach

**Unit tests (pure, table-driven)**
- They live beside the code, using `given`/`expected`/`actual` naming.
- Each v7 deftest in scope maps to a Go test or a table row. The mapping is listed in `tasks.md` by v7 namespace.
- v7 fixtures (zips, catalogues, GitHub/GitLab/WoWInterface responses, configs) are copied into `strongbox/src/testdata/` as needed. Tests that used curseforge as a placeholder source use wowinterface.

**Property and fuzz tests (built-in `testing` only)**
- settings migration idempotence;
- nfo write-then-read round trip;
- `.toc` parser fuzzing (`FuzzParseTOC`);
- zip entry path validation fuzzing (`FuzzZipEntryPath`, no extraction outside the root);
- `determine_primary_subdir` invariants (the result is a prefix of every input, or empty).

**Integration tests**
- `strongbox/main_test.go` keeps its single-GUI-instance structure, because one Tk interpreter is allowed per process.
- It gains a `FixtureTransport` in isolation and a temporary `XDG_*_HOME`.
- Subtests:
  - first-run import of a v7 config;
  - add addons dir;
  - search;
  - install;
  - publish a newer release in the fixture map, then refresh and check that the update is marked;
  - update;
  - pin, which blocks update;
  - ignore, which blocks uninstall;
  - install from zip;
  - import by URL;
  - switch source;
  - uninstall (confirmed by replacing `GUIUI.Confirm`);
  - remove addons dir.
- Each subtest ends with `WaitForIdle` and asserts on both the filesystem and the GUI rows (via `tab` row data).
- It fails if `FixtureTransport.Unrouted` is non-empty.

**Seams for driving the GUI**
- `main_gui(gui_opts)` takes an HTTP transport, a confirm function and an error reporter.
- `GUIUI.InvokeSelectionService` is what the context menu calls, so the tests choose services exactly as a user does. `GUITab.RowOption`, `GUITab.RowCell` and `GUIFormField.ID` let them read rows and fill forms.
- The workflow test found that deleting a row left it, and every row below it, in the tab's key indexes. `delete_row_in_tree_sync` now forgets them.

**Running**
- `manage.sh test` runs `xvfb-run -a` when `DISPLAY` is unset. It gives the `strongbox` main package a 120 s timeout and leaves others at their current timeout.
- Every package runs with `-race -gcflags=all=-d=checkptr=0`. `checkptr`, which `-race` turns on, is disabled because the atk fork passes integer handles to C as pointers (recorded in `ISSUES.md`).
- The comment claiming `-timeout` is per test is corrected.

### D14. Developer inspection tool

**Decision**
- `strongbox/cmd/sbinspect` is a read-only CLI that reuses the provider's pure functions.
- `sbinspect addons <dir> --game-track retail --strict=true` prints JSON per addon: label, name, directories, group ID, primary, ignored (and why), pinned, the `.toc` chosen, nfo source, and installed version.
- `sbinspect settings <file>` prints the migrated settings and the issues found, writing nothing.
- The binary path is added to `.gitignore`.

**Why:** comparing v8's view of a real, long-lived addons dir against v7's view is the fastest way to find loading and grouping differences that fixtures miss. The tool must never write, so it does not start boardwalk.

## Risks / Trade-offs

- **[Large change across both modules]**
  - The tasks are ordered so each group leaves the tree building with its tests passing. Boardwalk mechanisms come first, then strongbox data rules, then services, then GUI wiring, then integration.
- **[Boardwalk API churn: `ItemHandlerMap` removed, `Service` extended, `App.State` became `App.State()`]**
  - The only providers are strongbox and bw's filesystem provider, both in this repository. Both migrate in the same task group.
- **[Tk thread deadlocks with new dialogs]**
  - Every dialog and menu `-postcommand` follows the existing rule: compute on the Tk thread, run services off it (`go`).
  - Integration tests run under `-race`.
- **[Integration test flakiness from asynchrony]**
  - Assertions happen only after `WaitForIdle`. There are no sleeps.
- **[A v8-written nfo still confuses v7]**
  - An nfo round-trip test asserts the bytes v8 writes for single and shared directories match v7's shapes.
  - The changelog notes that v7 and v8 should not manage the same addons dir at the same time.
- **[Keeping missing addons dirs could surprise users who meant to delete them]**
  - They are shown as unavailable and can be removed explicitly.
- **[WoWInterface addons without catalogue game tracks get no updates]**
  - This is the strict outcome. Before, the update was wrongly assumed retail.
  - It is logged at INFO naming the addon, so it is discoverable.
- **[Per-addons-dir lock serialises installs]**
  - Acceptable for correctness. The plan/execute split lets finer locks replace it later.
- **[Self-update check is an outbound request]**
  - It is off when the preference is off, it runs at most once per start, and no identifying data beyond the User-Agent is sent.

## Migration Plan

1. **First v8 run, v7 user:** v8's directories are created. `config.json` is imported from v7 and migrated, and `user-catalogue.json` is copied. Catalogues are downloaded into v8's data dir. v7 is untouched.
2. **v8 pre-release users** (`~/.config/strongbox8` already exists): the existing file migrates in place through the same pipeline. The file has no `spec.version`, which marks it as a pre-release. It is saved with `spec.version: 1`.
3. **Rollback:** delete `~/.config/strongbox8` and `~/.local/share/strongbox8`. The next start imports from v7 again. nfo files written by v8 remain readable by v7.
4. **Changelog:** the breaking changes from `proposal.md` go under 8.0.0.
5. **ISSUES.md:** entries resolved here are removed:
   - `install_addon` swallowing failures;
   - `download_catalogue_addon` indexing;
   - `read_settings_file` extension check;
   - every WoWInterface update assumed retail;
   - `NFO.IsEmpty`.

## Open Questions

- **Update-check concurrency limit (8).** It can be tuned without changing behaviour.
- **Catalogue staleness threshold (1 hour, matching v7's HTTP expiry).** It could become a preference later.
- **Busy-row styling.** Exact colours are left to the parade theme. Any visibly distinct style meets the spec.
