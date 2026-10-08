## 1. Test infrastructure

- [x] 1.1 `manage.sh test`: run under `xvfb-run -a` when `DISPLAY` is unset. Give the `strongbox` main package a 120 s `-timeout`. Correct the comment claiming `-timeout` is per test.
- [x] 1.2 Copy the v7 fixtures needed by later groups into `strongbox/src/testdata/`:
  - zips (bundled, mutual dependency, multi-toc, non-addon tld, empty, truncated, Auctioneer);
  - catalogues;
  - GitHub, GitLab and WoWInterface API responses;
  - `user-config-*.json`;
  - user catalogues.
- [x] 1.3 Add a shared test helper for strongbox unit tests: a temp addons dir, an app with a `MapDownloader`, and a helper that writes `.toc`/nfo/zip fixtures from data, porting v7's `gen-addon!`.

## 2. Boardwalk: HTTP layer (`bw/http-cache`)

- [x] 2.1 Tests first: cache hit when fresh, miss when expired, non-2xx not cached, prune deletes expired entries.
- [x] 2.2 Remove the `if true ||` short-circuit in `cache_expired`. Default expiry is 1 hour. Add `prune_cache(dir, now)`.
- [x] 2.3 Move the cache directory to `<app data dir>/cache` (`App.SetDataDir` reconfigures the transport). Call the prune at provider start.
- [x] 2.4 Tests first, then reimplement `DownloadFile` on `app.HTTPClient`:
  - check the status before writing;
  - write to `<dest>.part` and rename into place;
  - remove the part file on failure;
  - leave an existing destination untouched on failure.
- [x] 2.5 Set the User-Agent from `core.VERSION` (default `8.0.0-unreleased`, overridable by `-ldflags`) and add request timeouts.
- [x] 2.6 Add `FixtureTransport` (URL → status/body/headers) with isolation mode recording `Unrouted` requests, plus a test.

## 3. Boardwalk: service declarations (`bw/service-applicability`, `bw/service-confirmation`)

- [x] 3.1 Tests first: derive `TypeMap` from `Service.Accepts`; single vs many arity; services without `Fn` excluded; `Applicable` evaluated per selection.
- [x] 3.2 Add `Accepts`, `Applicable`, `Confirm` and `ArgDef.FromSelection` to `core.Service`/`core.ArgDef`. Derive `App.TypeMap` in `StartProviders`. Remove `ItemHandlerMap` from `core.Provider`.
- [x] 3.3 Migrate the bw filesystem provider to `Accepts`.
- [x] 3.4 GUI context menu:
  - build entries from the derived map, disabled when `Applicable` is false;
  - run immediately when every remaining arg is `FromSelection`;
  - otherwise open a form prefilled with the selection.
- [x] 3.5 Re-evaluate menu items bound to a `ServiceID` with `Applicable(app, nil)` via the Tk menu `-postcommand`. Hide services without `Fn`.
- [x] 3.6 Add `GUIUI.Confirm(title, message) bool` (a replaceable field, defaulting to `tk_messageBox -type yesno`). Call it before any service that declares `Confirm`, from menus, context menus and forms. Add a test that replaces it.

## 4. Boardwalk: form widgets (`bw/form-widgets`)

- [x] 4.1 Choice list: a combobox for exclusive choices and a multi-select list for non-exclusive ones. The default is preselected. Validate that submitted values are among the choices.
- [x] 4.2 File picker (`tk_getOpenFile`, optional `-multiple`, extension filter, initial dir), and a directory picker using the same field layout.
- [x] 4.3 Checkbox for boolean arguments.
- [x] 4.4 An unsupported widget logs ERROR naming the service and argument, and the other fields still render.
- [x] 4.5 GUI tests: open forms with each widget, fill them programmatically and submit, and assert the parsed args.

## 5. Boardwalk: background jobs and busy rows (`bw/background-jobs`)

- [x] 5.1 Tests first: the job lifecycle in state (`StartJob`, `Tick`, `Finish`), concurrent jobs, `Finish` when deferred after a panic-free error path.
- [x] 5.2 Add a job store to `App` (map of job ID to `JobInfo`, mutex-guarded) and notify observers of each change with an `ACTION_JOBS_CHANGED` action queued through the update channel.
- [x] 5.3 Add a status bar to the main window showing "idle" or each job as `name (done/total)`.
- [x] 5.4 Add `core.TAG_BUSY` and distinct row styling. Update and clear it on `OnResultsChanged`.
- [x] 5.5 `GUIUI.WaitForIdle()`: services finished, no jobs, update queue drained (no-op `UpdateState`), then `TkSync`. Switch existing GUI tests to it.

## 6. Strongbox: paths and settings (`strongbox/settings`)

- [x] 6.1 Tests first: `generate_path_map(env)` for defaults, XDG overrides and empty values. Every directory ends in `strongbox8`. The v7 path derivation is a separate function.
- [x] 6.2 Implement the pure `generate_path_map(env)`. Delete the duplicate `xdg_path` in `main.go` and use the exported function. Refuse an unwriteable data directory.
- [x] 6.3 Tests first: one table row per fixture (0.9 to 7.0, plus 8.0 pre-release) asserting the migrated settings and issues. Port v7 `config_test` cases:
  - `handle-install-dir`;
  - `catalogue-location-list` (missing, empty and invalid entries);
  - `invalid-addon-dirs-in-cfg` (now kept, not dropped);
  - `handle-selected-addon-dir`;
  - `convert-compound-game-track`.
- [x] 6.4 Implement the format probe, the strict `SettingsV7` struct (superset of every v7 key, with pointers for presence-sensitive values), `settings_from_v7` as one explicit conversion function, `upgrade` for v8 versions, unknown-key reporting, and per-entry zog validation (unknown game track, relative path, duplicate path, bad catalogue location, wrong preference type). Remove the `Deprecated*` fields from `Settings`.
- [x] 6.5 Property tests:
  - `upgrade(settings_from_v7(x))` equals `settings_from_v7(x)`;
  - save-then-load is the identity;
  - load-save-load gives the same `Settings` for every fixture.
- [x] 6.6 Add the `write_atomic` helper (temp, fsync, rename) and test it. Use it for settings.
- [x] 6.7 Unparseable settings: copy to `config.json.<timestamp>.invalid`, use defaults, log ERROR. Tests for invalid JSON and a non-object top level.
- [x] 6.8 Write `spec.version`. A newer version makes the session read-only (`SaveSettings` no-op, one WARN). Test included.
- [x] 6.9 One-time import of v7 `config.json` and `user-catalogue.json` when v8's are missing. Tests: v7 file unchanged byte for byte, later runs ignore v7, unreadable v7 file falls back to defaults with WARN.
- [x] 6.10 Save settings immediately on every settings mutation (addons dirs, selection, game track, strictness, catalogue, preferences). Test that each mutation is persisted.

## 7. Strongbox: game tracks (`toc-classification`, `toc-selection`, `github-classification`, `addons-dir-management`)

- [x] 7.1 Update the existing tests to the modified spec scenarios (forever, mists, preference order). Port v7 tests:
  - `utils_test` `interface-version-to-game-track`, `game-version-to-game-track` and `guess-game-track`;
  - `toc_test` `find-toc-files`;
  - `specs_test` `game-tracks-label-map`.
- [x] 7.2 Add `classic-mists` and `forever` to the constants, labels, aliases (including `mop`, `cataclysm` and `wotlkc`), `GAMETRACK_PREF_MAP`, interface ranges and the guess pattern list (forever/camelot and mists first). Add the `.toc` file-name suffixes (`Forever`, `Camelot`, `Mists`, `WOTLKC`, alongside the existing ones).
- [x] 7.3 Add `guess_game_track_from_path` with tests (`_retail_`, `_classic_`, `_classic_era_`, no match gives retail).
- [x] 7.4 Add a test asserting v8's game track names, `.toc` suffixes and flavour aliases agree with the values used by `github-wow-addon-catalogue` (`FLAVOR_LIST`, `FLAVOR_ALIAS_MAP`, `INTERFACE_RANGES`) and `strongbox-catalogue-builder-go` (`AllGameTracks`, `guessGameTrack`), copied verbatim into the test.

## 8. Strongbox: nfo files (`strongbox/nfo-files`)

- [x] 8.1 Tests first:
  - single owner writes an object;
  - a shared directory writes an array;
  - a one-element array is read as a single owner and rewritten as an object;
  - integer and string source IDs;
  - a v1 nfo gains a source map list;
  - an invalid nfo is reported and not deleted.
- [x] 8.2 Introduce the nfo file sum type (full/grouping `NFO` or `IgnoreFlag`). Validate on write (game-track enum). Write atomically.
- [x] 8.3 Ignore flag editing per spec (ignore-only files, clearing ignore, an empty file deleted, the implicit-ignore revert case). Port v7 `nfo_test` `update-nfo-data-with-ignore-flags`, `rm-nfo*`, `pin!`/`unpin!` and `mutual-dependencies`.
- [x] 8.4 Property test: write-then-read round trip for generated nfo stacks.

## 9. Strongbox: installed addon loading (`strongbox/installed-addon-loading`)

- [x] 9.1 Port v7 `toc_test`: `parse-toc-file`, `parse-addon-toc`, `--x-source`, `rm-trailing-version`, `parse-interface-value`, `--invalid-toc-questie`, and BOM handling. Fix any parser divergences.
- [x] 9.2 Add a fuzz test `FuzzParseTOC` (no panics, keys lower-cased).
- [x] 9.3 Tests first:
  - Blizzard skip;
  - no-`.toc` directory WARN;
  - grouping with a primary;
  - grouping without a primary (`<group-id> (group)`);
  - several primaries give a deterministic choice;
  - port v7 `addon_test` `group-addons` and `load-installed-addons-*`.
- [x] 9.4 Implement deterministic grouping and primary selection, and implicit ignore from VCS directories (fixing the inverted check) and `@project-version@` across all members. An explicit `ignore?: false` wins.
- [x] 9.5 Implement `load_addons_dir_results` with deterministic IDs (`<addons-dir-path>\x00<group-id or dirname>`). Use it from `AddonsDir.ItemChildren` and from reload. Reload replaces children in one `UpdateState`. Delete `LoadAllInstalledAddonsToState`'s duplicating path.
- [x] 9.6 Fill `Addon.PinnedVersion` from nfo data. Sort by case-insensitive label, then directory name.

## 10. Strongbox: addons dir management (`strongbox/addons-dir-management`)

- [x] 10.1 Port v7 `core_test` `addon-dir-handling` and `game-strictness` as tests over state.
- [x] 10.2 Add an addons dir (guessed game track, strict, selected, saved; duplicates are a no-op; not-a-directory refused). Select an addons dir (load, match, check, save).
- [x] 10.3 Remove an addons dir: no disk changes, fall back to the next available selection, remove its rows from state (fixing "the removed item remains visible").
- [x] 10.4 Set game track and set strictness: save, then re-evaluate addons (re-run `MakeAddon` over existing results, then check updates).
- [x] 10.5 Unavailable addons dirs: a derived availability field, never auto-selected, not installable into, kept in settings.

## 11. Strongbox: catalogue and user catalogue (`strongbox/catalogue`, `strongbox/user-catalogue`)

- [x] 11.1 Port v7 `catalogue_test`: `format-catalogue-data`, `read-catalogue`, `read-bad-catalogue`. Add description truncation, the curseforge/tukui filter, and entries without `download-count`/`tag-list`, using a catalogue produced by `strongbox-catalogue-builder-go` as a fixture.
- [x] 11.2 Freshness: re-download when the local copy is missing or older than 1 hour. Failures keep the old copy and log WARN. Tests via `MapDownloader` and file mtimes.
- [x] 11.3 Corrupt local catalogue: delete and re-download once, otherwise ERROR with no catalogue loaded. Port v7 `re-download-catalogue-on-bad-data` and `-2`, and `http-500-downloading-catalogue`.
- [x] 11.4 Combine the user catalogue and the selected catalogue by whole entries keyed by source and source ID; the selected catalogue's entry replaces the user's, with no field-level merge. Re-enable `DBLoadUserCatalogue`. v7's `merge-catalogues` test (field kept from the older entry) is ported with the inverted expectation.
- [x] 11.5 Switch catalogue: save, load, re-match, replace search results.
- [x] 11.6 User catalogue add, remove and star, with atomic writes and invalid-file preservation. Port v7 `add-user-addon!`, `--idempotence` and `remove-user-addon!`.
- [x] 11.7 Refresh user catalogue: full catalogue lookup, then host `FindAddon`, keeping entries that are not found, one write at the end. Port v7 `refresh-user-catalogue-item*` and `refresh-user-catalogue--not-in-catalogue`.
- [x] 11.8 Scheduled refresh when `keep-user-catalogue-updated` is on and the datestamp is older than 28 days. Port v7 `scheduled-user-catalogue-refresh`.

## 12. Strongbox: matching (`strongbox/catalogue-matching`)

- [x] 12.1 Tests first:
  - rule order;
  - first-in-catalogue-order wins (add `index_first`);
  - the source-versus-name rule is gone;
  - ignored addons are unmatched;
  - an unmatched addon with nfo source is still checkable;
  - summary INFO;
  - port v7 `db-match-installed-addon-list-with-catalogue*`, `moosh-addons` and `db-addon-by-source-and-source-id`.
- [x] 12.2 Implement the matcher changes and matched-detail rules (catalogue description fallback, nfo source kept when it differs).

## 13. Strongbox: hosts (`strongbox/gitlab-source`, `strongbox/wowinterface-source`, GitHub)

- [x] 13.1 Define the `AddonSource` interface (`ParseURL`, `FindAddon`, `ExpandSummary(ExpandRequest)`) and `SOURCE_MAP`. Migrate GitHub and WoWInterface.
- [x] 13.2 GitHub: `ParseURL`, `FindAddon` (releases, then `release.json`, then root `.toc` via contents; not found when no game track), `GITHUB_TOKEN`, and the rate-limit message. Port v7 `github_api_test`:
  - `parse-user-string*`;
  - `find-addon--*`;
  - `rate-limit-exceeded`;
  - `auctioneer`.
- [x] 13.3 GitLab: new `gitlab_api.go` covering URL parsing, releases to updates (upcoming excluded, link types, direct asset URL), classification and `FindAddon`. Port every v7 `gitlab_api_test` case.
- [x] 13.4 WoWInterface: game tracks from `ExpandRequest.KnownGameTracks` (catalogue list, else nfo installed game track, else none), URL parsing, 404 handling. Port v7 `wowinterface_api_test` and `catalogue_test` `expand-summary--*` (wowi strict/relaxed cases).
- [x] 13.5 A non-zip body with HTTP 200 is rejected as an invalid zip. Port v7 `install-update-these-in-parallel--bad-download`.

## 14. Strongbox: update checking (`strongbox/update-checking`)

- [x] 14.1 Port v7 `addon_test` `test-updateable?` (full truth table) and `catalogue_test` `expand-summary--pinned--*` and `expand-summary--not-found-message`.
- [x] 14.2 Update choice: strict, relaxed with the preference order, and the pinned release preferred. Include the "no 'X' or 'Y' release found on <host>" message.
- [x] 14.3 Parallel checks:
  - a bounded pool;
  - each row marked busy and cleared;
  - per-addon WARN on failure;
  - results written by result ID with `UpdateResult` (fixing the stale-copy write in `CheckForUpdates`);
  - one job with a step per addon.
- [x] 14.4 Unsupported sources are not requested (INFO once per addon per check). Ignored addons are not requested.
- [x] 14.5 "Check for updates" service for one or more selected addons.

## 15. Strongbox: installation (`strongbox/addon-installation`)

- [x] 15.1 Port v7 `zip_test` (`valid-zip-file?`, `valid-addon-zip-file?`, `unzip-file`, `suspicious-subdirs`). Add a path-traversal check and `FuzzZipEntryPath`.
- [x] 15.2 Tests first for the pure `plan_install`. Port v7 cases:
  - `install-addon-guard--*` (bundled, overwriting ignored/pinned, invalid zip, trial installation);
  - `install-addon--uninstall-fully-replaced-mutual-dependencies`;
  - `install-addons-with-mutual-dependencies` and `-user-warning`;
  - `uninstall-installed-addon` (the obsolete bundled dir is removed on upgrade);
  - `determine-primary-subdir`.
- [x] 15.3 Implement `plan_install` and `execute_install` (abort on the first failure, no nfo writes after a failed extract). Add the per-addons-dir lock.
- [x] 15.4 Install from the catalogue: one or many, an already-installed match is updated instead, switch to the installed tab, rows appear as they complete. Port v7 `read-strange-catalogue--unknown-source` and `--unknown-game-track`.
- [x] 15.5 Install from a URL: find, choose relaxed, dry-run validate, add to the user catalogue, install. Port v7 `cli_test` `import-addon--github`, `--wowinterface` and `--tukui`.
- [x] 15.6 Install from a zip file: group ID from the file name, grouping-only nfo, allowed over ignored (stays ignored) and pinned (unpinned). Port v7 `install-addon-from-file-in-parallel`, `install-addon-from-file--then-update`, `install-addon--ignore-then-update-from-file` and `unique-group-id-from-zip-file`.
- [x] 15.7 Zip pruning by `addon-zips-to-keep` (by mtime, own pattern only, inside the addons dir only), reading the preference from settings. Port v7 `install-addon-guard--remove-zip` and `--remove-multiple-zips`.

## 16. Strongbox: updating (`strongbox/addon-updating`)

- [x] 16.1 Update selected addons (updateable only, busy marks, group ID kept). Update all (concurrent downloads, serialised installs, refused while running).
- [x] 16.2 Re-install (the installed version's release, else the chosen update with WARN). Port v7 `test-re-installable?` and `test-find-release`.
- [x] 16.3 Install a specific release (choice of releases filtered by game track and strictness; unavailable for pinned or ignored addons).

## 17. Strongbox: removal, ignoring, pinning, source switching

- [x] 17.1 Tests first for the pure removal planning. Port v7:
  - `uninstall-addon`;
  - `uninstall-ignored-addon`;
  - `uninstall-ignored-bundled-addon`;
  - `uninstall-addons-with-mutual-dependencies--overwrote` and `--overwritten`;
  - `remove-addon--malign-addon-data`.

  Add a symbolic-link case.
- [x] 17.2 Implement removal with `Lstat` and `filepath.Rel` containment, a nil-nfo-safe path, partial-uninstall reporting, and skipping and reporting ignored addons in multi-selection.
- [x] 17.3 Ignore and stop ignoring services. Port v7 `ignore-addon`, `clear-addon-ignore-flag`, `--group-addons` and `--implicit-ignore`.
- [x] 17.4 Pin and unpin services. Port v7 `pin-addon`, `unpin-addon`, `test-pinned-dir-list`, `test-pinnable?`, `test-unpinnable?` and `test-find-pinned-release`.
- [x] 17.5 Switch source: compute the source map list in `MakeAddon` from the nfo and `.toc` inputs (ordered, deduplicated, dead hosts filtered), rewrite nfo, re-match and check. Port v7 `switch-source*`, `merge-toc-nfo--source-map-list*`, `merge-lists` and `extract-source-map-list`.

## 18. Strongbox: startup, refresh, search, self-update (`strongbox/startup`, `strongbox/catalogue-search`, `strongbox/self-update-check`)

- [x] 18.1 Restructure `Start`: paths, dirs, settings, then a synchronous local load of the selected addons dir, then the background `refresh`. Start must not wait on the network. Test with a transport that blocks forever.
- [x] 18.2 `refresh` as a single-flight tracked job in the D6 order. Test that a second concurrent call returns immediately and that refreshing twice produces no duplicates.
- [x] 18.3 Refuse to run as root in `main` (testable helper over the effective UID).
- [x] 18.4 Search filter: case-insensitive substring over label, name and description. Results are replaced on catalogue change. Port v7 `search-db`, `search-db--empty-db` and `empty-search-state` (the text parts).
- [x] 18.5 Self-update check: background, once per start, preference-gated, pre-releases ignored, a local semver comparator, results shown in the About dialog and an INFO message. Port v7 `-download-strongbox-release-list*` and `latest-strongbox-release!*`. Add semver-ordering tests.

## 19. Strongbox provider services and GUI wiring (`strongbox/gui`)

- [x] 19.1 Rewrite `provider()`: every service in the spec tables has `Fn`, `Accepts`, `Applicable` and `Confirm` where needed. Delete any remaining placeholder and the hand-written `ItemHandlerMap`.
- [x] 19.2 Addons dir services:
  - Select;
  - Set game track (choice list of seven labelled tracks);
  - Set strictness (checkbox);
  - Browse (`xdg-open`, detached, WARN on failure);
  - Remove (confirmation).
- [x] 19.3 Addon services:
  - Check for updates;
  - Update;
  - Re-install;
  - Releases;
  - Switch source;
  - Pin, Unpin;
  - Ignore, Stop ignoring;
  - Star;
  - Uninstall (confirmation naming the addons).

  Each has its applicability predicate.
- [x] 19.4 Catalogue addon services: Install, Star, Unstar.
- [x] 19.5 Menus:
  - File → Install addon from file (file picker, `.zip`, opens in the addons dir), Import addon (URL), New addons directory (directory picker), Update all;
  - View → Refresh;
  - Catalogue → Switch catalogue, Refresh user catalogue;
  - Preferences → Preferences.

  Items needing an addons dir are disabled without one. No `donothing` remains.
- [x] 19.6 Installed tab columns from `ui-selected-columns` via a name-mapping table (unknown names ignored, no forced `ns` column). Add the combined version with `(ignored)`/`(pinned)` prefixes. Show update, ignored and busy row marks.

## 20. Integration tests (Xvfb)

- [x] 20.1 Make `main_gui` accept options (HTTP transport, confirm function) so tests inject a `FixtureTransport` in isolation and a scripted `Confirm`. Build fixtures: a catalogue with EveryAddon on GitHub, its release JSON (1.2.3, later 1.2.4), zips, a WoWInterface addon, and a v7 `config.json`.
- [x] 20.2 Subtests in `strongbox/main_test.go`, each ending in `WaitForIdle` with filesystem and row assertions:
  - first-run import of v7 settings (v7 file unchanged);
  - add addons dir;
  - search for EveryAddon;
  - install from the search tab.
- [x] 20.3 Subtests: publish 1.2.4 in the fixture map, refresh, update marked, update applied, mark cleared; pin blocks update; unpin.
- [x] 20.4 Subtests: ignore blocks uninstall and update; stop ignoring; install from a zip file; import by URL (added to the user catalogue); switch source.
- [x] 20.5 Subtests: uninstall declined (nothing removed), uninstall confirmed (all group directories removed), remove addons dir (files untouched). Fail the test if `FixtureTransport.Unrouted` is non-empty.
- [x] 20.6 Run the full suite under `xvfb-run` and with `-race`. Fix any races or Tk-thread deadlocks found.

## 21. Developer inspection tool

- [x] 21.1 Add `strongbox/cmd/sbinspect` with `addons <dir> --game-track --strict` (JSON per addon: dirs, group, primary, ignored and why, pinned, chosen `.toc`, nfo source, installed version) and `settings <file>` (migrated settings plus issues, no writes). Add a test that it never writes to the inspected dir.
- [x] 21.2 Add the `sbinspect` binary to `.gitignore`. Compare v8's output against v7 on a real long-lived addons dir. Record each divergence as a fixed bug, a test, or an `ISSUES.md` entry.

## 22. Documentation and cleanup

- [x] 22.1 Remove the resolved entries from `ISSUES.md`. Add any new defects found during this work.
- [x] 22.2 Remove stale CLI mentions and dead code left by the rewrite (commented-out services, `load_addons_dir`, `update_all_addons` stubs, `InstalledAddonToAddon`).
- [x] 22.3 Run `./manage.sh test` and `./manage.sh coverage` under Xvfb. Raise the coverage threshold to the new baseline.
