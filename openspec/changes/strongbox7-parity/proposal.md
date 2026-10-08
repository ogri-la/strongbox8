## Why

Strongbox 8 cannot yet replace strongbox 7 for its core job of finding, installing, updating and removing addons. The strongbox provider declares most of its services as placeholders. The services that do exist have defects that matter to users:
- update checks are served from an HTTP cache that never expires;
- startup blocks on the network;
- implicit ignoring of version-controlled addons is inverted;
- nfo files are written in a form that strongbox 7 misreads;
- a corrupt settings file is replaced with defaults.

This change brings v8 to parity with v7's core behaviour. It favours v8's goals of speed and strictness, and keeps boardwalk free of strongbox-specific features.

## What Changes

- **Settings.**
  - v8 keeps its config and data in their own directories (`~/.config/strongbox8`, `~/.local/share/strongbox8`, or `$XDG_*_HOME/strongbox8`).
  - On first run it imports strongbox 7's `config.json` and `user-catalogue.json` read-only. v7's files are never written.
  - Every historical v7 config version (0.9 to 7.0) and the v8 alpha format migrate deterministically.
  - A settings file that cannot be read is preserved, never overwritten. A settings file from a newer version makes the session read-only.
- **Startup.** Installed addons are shown immediately from local data. Loading the catalogue, matching and checking for updates happen in the background, with visible progress. Nothing is installed without the user asking.
- **Game tracks.** `classic-mists` and `forever` become supported game tracks, joining retail, classic, classic-tbc, classic-wotlk and classic-cata, so that v7 addons dirs on those tracks migrate.
- **Addons dirs.** Add, select and remove addons dirs, and change a dir's game track and strictness. A dir that is temporarily missing (an unmounted drive) is kept and shown as unavailable rather than silently deleted.
- **Installed addons.** Fixes to loading:
  - implicit ignore from a `.git`/`.hg`/`.svn` directory or an unrendered `@project-version@`;
  - stable result identities;
  - Blizzard addons are skipped;
  - grouping by group ID.
- **nfo files.** v8 writes nfo files v7 can read: a JSON object for a single addon, and an array only for shared directories. An unreadable nfo file is reported and left on disk, never deleted.
- **Catalogue.**
  - The catalogue is refreshed when stale, and a corrupt local copy is downloaded again.
  - The user can switch catalogues.
  - The user catalogue is combined with the selected catalogue by whole entries (no field-level merging), can be added to and removed from (starring), and is refreshed on schedule.
- **Matching.** Installed addons are matched to the catalogue by v7's ordered rules, with one change: the defective "installed source equals catalogue name" rule is removed. Ignored addons are never matched.
- **Hosts.** A GitLab source is added. WoWInterface updates take their game tracks from the catalogue instead of assuming retail. GitHub honours `GITHUB_TOKEN`.
- **Install.**
  - Sources: the catalogue, a URL (GitHub, GitLab or WoWInterface; added to the user catalogue), and a local zip file.
  - Zip validation, the refusal to overwrite ignored or pinned addons, shared-directory (mutual dependency) handling, and uninstalling addons the new zip completely overwrites.
  - Pruning of downloaded zips according to `addon-zips-to-keep`.
- **Update.** Update, update all, re-install, and installing a specific release. Pin and unpin. Ignore and stop ignoring. Switching source between the hosts an addon lists.
- **Uninstall.** Every directory of an addon group is removed, shared directories are respected, and ignored addons are refused.
- **Search.** Case-insensitive substring search over catalogue addon names and descriptions.
- **Self-update check.** If the `check-for-update` preference is on, v8 checks GitHub once per start for a newer strongbox release and reports it.
- **Boardwalk (generic, benefits all providers).**
  - Services declare which item types they accept and when they apply, so context-menu entries are enabled or disabled per selection.
  - Destructive services ask for confirmation.
  - New form widgets: choice list, file picker and checkbox.
  - Background jobs report progress in a status bar, and rows being worked on are marked busy.
  - The HTTP cache honours expiry and lives in the data dir. Downloads are atomic.
  - A headless test harness provides deterministic "wait until idle" and isolated fake HTTP.
- **Testing.**
  - v7's business-as-usual test cases are ported as Go unit tests, within reason.
  - Xvfb integration tests drive the GUI through: add addons dir, search, install, update, uninstall.
  - A read-only developer tool prints what v8 loads from a real addons dir, to compare against v7.
- **BREAKING (relative to strongbox 7, recorded in the changelog):**
  - **Separate settings.** v8 uses its own directories. After the first import, changes made in v7 are not seen by v8, and vice versa.
  - **Removed command-line options.** `--addons-dir`, `--headless`, `--ui`, `--action` and `--[no-]update-check` are gone (the CLI UI was already removed).
  - **Features not carried over:**
    - importing and exporting addon lists;
    - the Cache menu (clearing the cache, zips and catalogues, and deleting `.wowman.json`, `WowMatrix.dat` and `.strongbox.json` files);
    - colour themes (`gui-theme` is preserved but has no effect);
    - column presets;
    - the log pane and the addon detail pane;
    - search filters by host, tag and starred.
  - **nfo handling.** An invalid `.strongbox.json` is no longer deleted. It is reported, and the addon is treated as not installed by strongbox.
  - **Missing addons dirs.** These are kept in the settings rather than removed.

## Capabilities

### New Capabilities

- `strongbox/settings`: paths, one-time v7 import, migration of every historical config version, validation, and safe saving (never overwriting an unreadable or newer file).
- `strongbox/startup`: start order, immediate display of installed addons, background refresh with progress, refusal to run as root.
- `strongbox/addons-dir-management`: adding, selecting and removing addons dirs; game track and strictness; default game track guessed from the path; unavailable dirs.
- `strongbox/installed-addon-loading`: reading `.toc` and nfo data into grouped addons, implicit ignore, Blizzard exclusion, stable identities.
- `strongbox/nfo-files`: reading and writing `.strongbox.json` compatibly with v7, mutual-dependency stacks, ignore and pin flags.
- `strongbox/catalogue`: catalogue locations, selection, download freshness, corrupt-file recovery, loading and combining with the user catalogue by whole entries.
- `strongbox/user-catalogue`: adding and removing addons (starring), import-by-URL entries, scheduled refresh.
- `strongbox/catalogue-matching`: ordered match rules between installed addons and the catalogue.
- `strongbox/update-checking`: expanding sources into updates, choosing an update by game track and strictness, the updateable rules, parallel checks.
- `strongbox/gitlab-source`: finding GitLab addons by URL and turning their releases into updates.
- `strongbox/wowinterface-source`: WoWInterface updates and URL parsing, game tracks from the catalogue.
- `strongbox/addon-installation`: installing from the catalogue, a URL or a zip file; zip validation; overwrite guards; mutual dependencies; complete-overwrite uninstall; zip pruning.
- `strongbox/addon-updating`: update, update all, re-install, install a specific release.
- `strongbox/addon-removal`: uninstalling addon groups safely.
- `strongbox/addon-ignoring`: explicit and implicit ignore, and what ignoring blocks.
- `strongbox/addon-pinning`: pinning, unpinning and pinned update rules.
- `strongbox/source-switching`: switching an addon between the sources in its source map list.
- `strongbox/catalogue-search`: searching catalogue addons.
- `strongbox/self-update-check`: checking for a newer strongbox release.
- `strongbox/gui`: the menus, context menus and forms strongbox presents through boardwalk.
- `bw/service-applicability`: services declare accepted item types and an applicability predicate; context menus enable and disable entries from them.
- `bw/service-confirmation`: services that declare themselves destructive ask before running.
- `bw/form-widgets`: choice-list, file-picker and checkbox widgets.
- `bw/background-jobs`: tracked background jobs, a progress status bar, busy rows.
- `bw/http-cache`: cache expiry, cache location, atomic downloads, isolated fake transport for tests.

### Modified Capabilities

- `strongbox/toc-classification`: interface versions 1.60–1.99 map to `forever` and 5.x maps to `classic-mists`, instead of to no game track.
- `strongbox/toc-selection`: the relaxed-mode game track preference order includes `classic-mists` and `forever`.
- `strongbox/github-classification`: game track guessing recognises `mists` and `forever`/`camelot`.

## Impact

- **Code:** most of `strongbox/src` (`provider.go`, `core.go`, `settings.go`, `addons_dir.go`, `addon.go`, `nfo.go`, `catalogue.go`, `toc.go`, `utils.go`, `models.go`, `wowinterface_api.go`, plus a new `gitlab_api.go`), and `strongbox/main.go`.
- **Boardwalk:** `bw/core` (`service.go`, `core.go`, `http.go`), `bw/http_utils`, `bw/ui` (`gui.go`, `gui_form.go`).
- **Data on disk:**
  - new directories `~/.config/strongbox8` and `~/.local/share/strongbox8`;
  - nfo files written by v8 change shape for single addons (object instead of array);
  - v8 alpha settings files are migrated.
- **Tests:** new fixtures (zips, catalogues, host API responses), ported v7 tests, Xvfb integration tests. `manage.sh test` runs GUI tests under `xvfb-run` when no display is available.
- **Build:** a new developer tool binary, added to `.gitignore`.
- **Dependencies:** none added. Existing `zog`, `golang-set` and `conc` are reused.
- **Docs:** `CHANGELOG.md` records the breaking changes listed above. `ISSUES.md` entries fixed by this change are removed.
