## Purpose

Uninstalls addons completely and safely, never touching anything outside the addons dir, protected addons, or directories other addons still use.

## ADDED Requirements

### Requirement: Uninstall removes every directory of the addon

Uninstalling an addon SHALL remove every directory of its group, after the user confirms. A directory shared with another addon SHALL be kept, with only this addon's entry removed from its nfo data. An addon without nfo data SHALL have only its own directory removed. Uninstalling SHALL remove the addon from the GUI.

#### Scenario: Grouped addon

- **WHEN** the user uninstalls EveryAddon, which owns `EveryAddon/` and `EveryAddon-BundledAddon/`
- **THEN** both directories are removed

#### Scenario: Unmanaged addon

- **WHEN** the user uninstalls an addon without nfo data in `EveryAddon/`, next to `EveryAddon-BundledAddon/`
- **THEN** only `EveryAddon/` is removed

#### Scenario: Shared directory, newest owner removed

- **WHEN** EveryOtherAddon replaced `EveryAddon-BundledAddon/` of EveryAddon, and the user uninstalls EveryOtherAddon
- **THEN** `EveryAddon-BundledAddon/` remains and its nfo data describes EveryAddon

#### Scenario: Shared directory, older owner removed

- **WHEN** EveryOtherAddon replaced `EveryAddon-BundledAddon/` of EveryAddon, and the user uninstalls EveryAddon
- **THEN** `EveryAddon/` is removed, `EveryAddon-BundledAddon/` remains and its nfo data describes only EveryOtherAddon

### Requirement: Ignored addons are not uninstalled

Uninstalling an ignored addon, or a group with any ignored member, SHALL be refused with a message, and nothing SHALL be removed. When a selection of several addons includes ignored ones, the ignored ones SHALL be skipped and reported, and the rest uninstalled.

#### Scenario: Ignored addon

- **WHEN** the user uninstalls an addon whose directory contains `.git/`
- **THEN** nothing is removed and the user is told the addon is ignored

### Requirement: Removal stays inside the addons dir

A directory SHALL be removed only when it is a real directory, not a symbolic link, strictly inside the addons dir. Any other path, including `./`, `../`, the addons dir itself, an absolute path elsewhere or `~/Desktop`, SHALL be refused with an error naming the path, and nothing SHALL be removed. When removing a group stops at a refused or failed directory, the user SHALL be told the uninstall was partial.

#### Scenario: Malicious directory name

- **WHEN** an addon's directory name is `../`
- **THEN** nothing is removed and an error names the path

#### Scenario: Symbolic link

- **WHEN** an addon's directory is a symbolic link to a directory outside the addons dir
- **THEN** the link target is not removed
