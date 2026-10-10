## Purpose

Lets the user tell strongbox to leave an addon entirely alone, and recognises developer checkouts that should be left alone without being told.

## ADDED Requirements

### Requirement: Ignore and stop ignoring

The user SHALL be able to ignore one or several selected addons that are installed and not ignored, and to stop ignoring ignored addons. Each change SHALL be written to nfo data as defined for ignore flag editing, and the addons SHALL be re-evaluated.

#### Scenario: Ignore

- **WHEN** the user ignores EveryAddon
- **THEN** EveryAddon is shown as ignored and has no update

#### Scenario: Stop ignoring a group

- **WHEN** one member of a group is ignored and the user stops ignoring the group
- **THEN** no member of the group is ignored

### Requirement: What ignoring blocks

An ignored addon SHALL NOT be matched to the catalogue, checked for updates, updated, re-installed, overwritten by an install from the catalogue or a URL, uninstalled, pinned or switched to another source.

#### Scenario: Update all skips ignored

- **WHEN** the user chooses update all and an ignored addon has a newer release at its host
- **THEN** the ignored addon is unchanged

#### Scenario: No request for ignored

- **WHEN** a refresh checks for updates
- **THEN** no host request is made for ignored addons
