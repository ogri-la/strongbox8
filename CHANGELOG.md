# Change Log
All notable changes to this project will be documented in this file. This change log follows the conventions of [keepachangelog.com](http://keepachangelog.com/).

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## 8.0.0-alpha.4 - 2026-11-14

### Added

* strongbox, parity with strongbox 7 for core addon management
    - addons dirs, installing from the catalogue, a URL or a zip file, updates and re-installs
    - pinning, ignoring, switching source, uninstalling, catalogue switching and the user catalogue
* strongbox, search tab matches name (exact) or description (substring), case-insensitive
* strongbox, `sbinspect` prints how an addons dir or settings file is read, without writing to it
* gui, per-tab search filter callback `(input, row) -> bool` (bw)
    - search entry is disabled on tabs without a filter
* gui, context menus offer only the services that apply to the selected rows (bw)
* gui, services can ask for confirmation before they run, such as uninstall (bw)
* gui, background jobs report progress in a new status bar (bw)
* gui, 'files' tab and File > 'Browse Directory' browse the filesystem one directory at a time (bw)
* gui, lazy rows such as directories load their children when expanded (bw)
* release building now happens concurrently

### Changed

* settings and data are kept in `strongbox8` directories, separate from strongbox 7
    - strongbox 7 settings and user catalogue are imported once, read-only
* gui, expanding, collapsing and searching rows appear on screen in one step, without flicker (bw)
    - Linux only, the binaries now need `libXcomposite`, `libXdamage` and `libXfixes`, see README
    - `BW_OFFSCREEN=0` disables it
* gui, searching no longer blocks typing on tables with thousands of rows (bw)
* gui, a column resize moves a guide line and resizes once on release (bw)
* gui, expanding a row or searching no longer changes column widths (bw)
* `./manage.sh release` refuses to run unless `./` and `./atk` are on `master` and match `origin/master`
* cleaned up release process and consolidated the disparate scripts
  - process: ./manage.sh build.image && ./manage.sh release
* tests run with the race detector and drive the GUI through whole workflows under Xvfb
* parade theme, disabled TEntry now visually distinct (grey field + foreground)
* TODO.md consolidates outstanding TODOs throughout code
  - converted it to a structured format
* ISSUES.md records known defects
* screenshots follow the strongbox 7 naming scheme

### Fixed

* game track classification no longer assumes retail
    - interface versions such as `110002` and `40400` map to the right game track
    - a Github release asset with no recognisable game track is excluded
* installing over an ignored or pinned addon is refused
* catalogue addons installed in the selected addons dir are marked installed in the search tab
* installing from a URL adds the addon to the user catalogue only after the install succeeds
* colour escape codes are removed from `.toc` labels and names
* a service that panics is reported as failed instead of successful (bw)
* a Tcl error at idle time is logged instead of opening a modal error dialog (bw)

### Removed

* `Provider.ItemHandlerMap`, replaced by services declaring the items they accept (bw)

## 8.0.0-alpha.3 - 2026-04-19

### Added

* gui, double-click to expand/collapse tree rows
* gui, right-click context menu with Properties item on all rows
    - right-click preserves multi-row selection
* gui, standard desktop keybindings for text inputs (Ctrl+A, Ctrl+Backspace, Ctrl+Delete)
* gui, app-level 'action' dispatch using an observer pattern
    - so services that don't have access to the UI can affect it (like switching tabs)
* strongbox, installing from search tab now switches to 'installed' tab first
	- so you can watch the addon appear immediately and be installed :)

### Changed

* replaced listener/event/channel system with observer pattern
* removed CLI and UI abstraction interfaces
* menus now styled by theme (background, active highlight, padding)
* service functions now execute off the Tk thread
* upgraded to Go 1.26
* `go fix` and `go mod tidy` added to lint command
* removed custom `Contains` in favour of `slices.Contains`
* observers now receive `Snapshot` type with built-in lookup instead of raw result slices

### Fixed

* false expand arrows on leaf nodes in tree view
* GUI startup visual jumping and slow display
* race condition in observer pattern state lookups
* right-click now selects the clicked row
* right-click menu no longer disappears on quick clicks

## 8.0.0-alpha.2 - 2025-09-07

### Added

* tighter integration with file menu 
    - and menu separators!
* gui error handler now prints a helpful stacktrace
* addons dir rows now have a 'selected' column with 'true' if the addons dir is selected
    - it's a bit rough but Strongbox 7 only displayed a single selected addons dir at once so didn't have this problem
* combined 'installed' and 'available' version column

### Changed

* abstract UI logic moved from 'ui' module to 'core' module
    - partly to resolve cyclic dependencies
* default tab on starting is now the addons dirs tab and not the search tab
* new addons dirs are immediately selected
* 'created' and 'updated' columns are now formatted in a human friendly relative offset

### Fixed

* fixed bug where settings weren't being saved if intermediate dirs didn't exist
* fixed bug where forms would accumulate in the side panel
* fixed bug where updates to child items with no direct descendent of the root item were ignored
* fixed issue where settings may be altered because of migrations during their load, but not saved afterwards
* fixed missing values in 'tags' columns

### Removed

## 8.0.0-alpha.1 - 2025-08-24

### Added

* initial release of Strongbox 8.0.0

### Changed

* application implementation changed from Clojure to Go, GUI changed from JavaFX to Tcl/Tk
