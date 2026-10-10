# strongbox/gui Specification

## Purpose

Defines how strongbox's features are reached through boardwalk's GUI: tabs, menus, context menus and forms, so every core workflow can be done with the mouse and keyboard and driven by integration tests.

## Requirements

### Requirement: Tabs

Strongbox SHALL present an "installed" tab listing addons dirs with the selected addons dir's addons as its children, and a "search" tab listing catalogue addons. The installed tab SHALL be shown first.

#### Scenario: Start

- **WHEN** strongbox starts with a selected addons dir holding two addons
- **THEN** the installed tab is shown with the addons dir expanded to its two addons

### Requirement: Menus

Strongbox SHALL contribute these menu items:

| menu | item | effect |
|---|---|---|
| File | Install addon from file | choose zip files and install them |
| File | Import addon | enter a URL and install it |
| File | New addons directory | choose a directory and add it |
| File | Update all | update all updateable addons |
| View | Refresh | refresh the selected addons dir and catalogue |
| Catalogue | Switch catalogue | choose a catalogue location from a list |
| Catalogue | Refresh user catalogue | refresh the user catalogue |
| Preferences | Preferences | edit `addon-zips-to-keep`, `keep-user-catalogue-updated` and `check-for-update` |

A menu item SHALL NOT exist without an implementation. Items that need an available addons dir SHALL be disabled when there is none.

#### Scenario: No placeholders

- **WHEN** any strongbox menu item is chosen
- **THEN** it performs its effect and does not log "not implemented"

#### Scenario: No addons dir

- **WHEN** no available addons dir exists
- **THEN** "Install addon from file", "Import addon" and "Update all" are disabled

### Requirement: Context menus

Right-clicking selected rows SHALL offer the services applicable to the selection, disabled when not applicable:

| selection | services |
|---|---|
| one addons dir | Select, Set game track, Set strictness, Browse, Remove |
| one or more addons | Check for updates, Update, Re-install, Pin, Unpin, Ignore, Stop ignoring, Star, Uninstall |
| exactly one addon | additionally Releases, Switch source |
| one or more catalogue addons | Install, Star, Unstar |

Update SHALL be enabled only when a selected addon is updateable. Pin, Releases and Switch source SHALL be disabled for ignored addons. Uninstall SHALL be disabled when every selected addon is ignored. Switch source SHALL be disabled when the addon has fewer than two sources. Unpin SHALL be enabled only when a selected addon is pinned, and Stop ignoring only when a selected addon is ignored. Browse SHALL open the addons dir in the desktop's file manager.

#### Scenario: Update disabled

- **WHEN** the user right-clicks an addon with no update
- **THEN** "Update" is shown disabled

#### Scenario: Ignored addon

- **WHEN** the user right-clicks an ignored addon
- **THEN** "Stop ignoring" is enabled and "Pin", "Uninstall" and "Update" are disabled

### Requirement: Destructive actions are confirmed

Uninstall and Remove addons dir SHALL ask the user to confirm, naming what will be affected, before acting. Declining SHALL change nothing.

#### Scenario: Decline uninstall

- **WHEN** the user chooses Uninstall and then declines
- **THEN** no files are removed

### Requirement: Installed tab columns

The installed tab SHALL offer columns for source, name, description, tags, created date, updated date, size, installed version, available version, combined version and game version. The columns shown SHALL follow the `ui-selected-columns` preference; unknown column names in it SHALL be ignored. The combined version SHALL show the available version when an update exists, otherwise the installed version, prefixed with `(ignored)` or `(pinned)` where applicable.

#### Scenario: Pinned display

- **WHEN** an addon is pinned at 1.2.3
- **THEN** its combined version shows `(pinned) 1.2.3`

### Requirement: Rows reflect addon state

An addon row SHALL be marked when it has an update, when it is ignored, and while it is busy. Marks SHALL update as the addon's state changes, without reloading the tab.

#### Scenario: Update mark cleared

- **WHEN** an addon with an update is updated
- **THEN** its row is no longer marked as having an update

### Requirement: Headless operation for tests

Every workflow in this specification SHALL be completable under a virtual X display without a desktop, with all network requests answered by test fixtures.

#### Scenario: Full workflow

- **WHEN** the integration test adds an addons dir, searches for EveryAddon, installs it, makes a newer release available, updates it, and uninstalls it
- **THEN** each step's files and GUI rows are as specified, and no real network request is made
