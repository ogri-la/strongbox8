# strongbox/addon-updating Specification

## Purpose

Applies updates the user has asked for, one addon, a selection or all of them, and lets the user re-install or choose a specific release.

## Requirements

### Requirement: Update selected addons

The user SHALL be able to update one or several selected addons. Only updateable addons in the selection SHALL be updated; others SHALL be skipped without error. Each update SHALL be downloaded and installed as an install from the catalogue, keeping the addon's group ID, and its row SHALL be marked busy until done. After updating, the addon SHALL show its new installed version and SHALL no longer be marked as having an update.

#### Scenario: Update one

- **WHEN** EveryAddon 1.2.3 is installed, 1.2.4 is available, and the user updates it
- **THEN** EveryAddon's installed version is 1.2.4 and it has no update

#### Scenario: Mixed selection

- **WHEN** the user updates two selected addons of which only one is updateable
- **THEN** only the updateable addon is changed

### Requirement: Update all

The user SHALL be able to update every updateable addon in the selected addons dir. Downloads MAY run concurrently. Installs SHALL be serialised per addons dir. While an update all is running, starting another SHALL be refused with a message that updates are in progress. A failure for one addon SHALL be reported and SHALL NOT stop the others.

#### Scenario: Update all

- **WHEN** three addons are updateable and the user chooses update all
- **THEN** all three are updated

#### Scenario: Already running

- **WHEN** the user chooses update all while an update all is running
- **THEN** no second update all starts and the user is told updates are in progress

### Requirement: Re-install

The user SHALL be able to re-install one or several selected addons. A re-install SHALL install the release whose version equals the installed version, when the host still offers it, otherwise the chosen update, logging at WARN level that the installed version was not available. Re-install SHALL be available only for addons that are installed, not ignored, and have a source.

#### Scenario: Same version restored

- **WHEN** EveryAddon 1.2.3 is installed, its host offers 1.2.4 and 1.2.3, and the user re-installs it
- **THEN** version 1.2.3 is installed again

#### Scenario: Version gone

- **WHEN** the host no longer offers the installed version
- **THEN** the chosen update is installed and a WARN says the installed version was unavailable

### Requirement: Install a specific release

The user SHALL be able to choose any release the host offers for an addon, filtered by the addons dir's game track and strictness, and install it. Choosing a release SHALL NOT be available for pinned or ignored addons.

#### Scenario: Downgrade

- **WHEN** EveryAddon 1.2.4 is installed and the user chooses release 1.2.3
- **THEN** version 1.2.3 is installed and the addon is offered 1.2.4 as an update
