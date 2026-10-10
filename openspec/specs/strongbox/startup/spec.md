# strongbox/startup Specification

## Purpose

Defines what strongbox does between launch and being ready, so the user sees their installed addons at once while slower network work happens visibly in the background.

## Requirements

### Requirement: Installed addons are shown before any network access

At startup strongbox SHALL load its settings and the installed addons of the selected addons dir from local disk, and present them, before making any network request. Starting the strongbox provider SHALL NOT wait on the network.

#### Scenario: Offline start

- **WHEN** strongbox starts with no network access and the selected addons dir holds three addons
- **THEN** the installed tab shows the three addons
- **AND** the provider reports itself started

#### Scenario: Local catalogue used first

- **WHEN** a catalogue file from a previous run exists locally
- **THEN** installed addons are matched against it before any newer catalogue is downloaded

### Requirement: Startup refresh runs in the background

After the installed addons are shown, strongbox SHALL run a background refresh: download the selected catalogue when stale, load the catalogue and user catalogue, match installed addons, then check each matched addon for updates. Progress SHALL be visible while it runs. The refresh SHALL NOT install, update or remove any addon.

#### Scenario: Updates are offered, not applied

- **WHEN** the startup refresh finds an update for an installed addon
- **THEN** the addon is marked as having an update
- **AND** the installed files of the addon are unchanged

#### Scenario: Progress shown

- **WHEN** the startup refresh is checking addons for updates
- **THEN** a status indicator shows that checking is in progress and how many addons remain

### Requirement: Refresh is idempotent

Running a refresh while one is running SHALL NOT start a second concurrent refresh. Running a refresh again after it completes SHALL NOT duplicate any addon, catalogue or addons dir in state.

#### Scenario: Refresh twice

- **WHEN** the user refreshes twice in succession
- **THEN** each installed addon appears exactly once in the installed tab

### Requirement: Strongbox refuses to run as root

Strongbox SHALL refuse to start when run by the root user, and SHALL report why.

#### Scenario: Run as root

- **WHEN** strongbox is launched with an effective user ID of 0
- **THEN** it exits with a non-zero status and a message saying it must not be run as root

### Requirement: Logging verbosity

Strongbox SHALL log to standard error at INFO level by default, and SHALL accept a verbosity option of `debug`, `info`, `warn` or `error`. An unknown verbosity SHALL be refused with a non-zero exit status.

#### Scenario: Debug opt-in

- **WHEN** strongbox is started with verbosity `debug`
- **THEN** DEBUG messages are logged
