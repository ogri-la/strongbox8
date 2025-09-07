# Change Log
All notable changes to this project will be documented in this file. This change log follows the conventions of [keepachangelog.com](http://keepachangelog.com/).

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 8.0.0-alpha.2 - 2025-09-07

### Added

* tighter integration with file menu 
    - and menu separators!
* gui error handler now prints a helpful stacktrace
* addons dir rows now have a 'selected' column with 'true' if the addons dir is selected
    - it's a bit rough but Strongbox 7 only displayed a single selected addons dir at once so didn't have this problem

### Changed

* abstract UI logic moved from 'ui' module to 'core' module
    - partly to resolve cyclic dependencies
* default tab on starting is now the addons dirs tab and not the search tab
* new addons dirs are immediately selected 

### Fixed

* fixed bug where settings weren't being saved if intermediate dirs didn't exist
* fixed bug where forms would accumulate in the side panel
* fixed bug where updates to child items with no direct descendent of the root item were ignored
* fixed issue where settings may be altered because of migrations during their load, but not saved afterwards

### Removed

## 8.0.0-alpha.1 - 2025-08-24

### Added

* initial release of Strongbox 8.0.0

### Changed

* application implementation changed from Clojure to Go, GUI changed from JavaFX to Tcl/Tk

## [Unreleased]

### Added

### Changed

### Fixed

### Removed
