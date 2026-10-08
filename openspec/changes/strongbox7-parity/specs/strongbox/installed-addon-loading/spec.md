## Purpose

Reads the addons in an addons dir from their `.toc` files and nfo files into the addons strongbox shows and manages, grouping directories that belong to one addon.

## ADDED Requirements

### Requirement: Every addon directory is read

Each subdirectory of an addons dir SHALL be read as an installed addon when it contains at least one `.toc` file directly inside it. A subdirectory whose name starts with `Blizzard_` SHALL be skipped. A subdirectory without a `.toc` file, or whose `.toc` files cannot be read, SHALL be skipped and logged at WARN level; it SHALL NOT prevent the other addons from loading.

#### Scenario: Blizzard addon skipped

- **WHEN** an addons dir holds `Blizzard_AuctionUI/` and `EveryAddon/`
- **THEN** only EveryAddon is loaded

#### Scenario: Directory without a .toc

- **WHEN** an addons dir holds `NotAnAddon/` with no `.toc` file and `EveryAddon/`
- **THEN** EveryAddon is loaded and a WARN names `NotAnAddon`

### Requirement: .toc parsing

A `.toc` file SHALL be read as `## Key: Value` lines:

- keys SHALL be case-insensitive;
- a value MAY contain colons;
- the space after `##` and after the colon SHALL be optional;
- extra leading `#` characters SHALL be accepted;
- the last of duplicate keys SHALL win;
- lines with leading whitespace, lines without a colon, and single-`#` comments SHALL be ignored;
- a commented `# ## Interface:` line SHALL be read as an additional interface version;
- a leading byte order mark SHALL be ignored.

The label SHALL be the `Title`, with trailing version numbers removed and colour escape codes kept for display only. When `Title` is missing, the label SHALL be the directory name followed by ` *`. The description SHALL be `Notes`, falling back to `Description`. The installed version SHALL be `Version`. Interface versions SHALL be read from a comma-separated `Interface` value, ignoring whitespace, empty items, duplicates and non-numeric items. Sources SHALL be read from `X-WoWI-ID` (numeric only, giving wowinterface), `X-Github` (giving github) and `X-Website` when it is a GitHub URL (giving github). When several are present, wowinterface SHALL be the source and the others SHALL be listed as alternative sources.

#### Scenario: Colons in values

- **WHEN** a `.toc` has `## Notes: Multi: Colon: Madness:`
- **THEN** the description is `Multi: Colon: Madness:`

#### Scenario: Trailing version removed from label

- **WHEN** the titles are `Grid 2`, `Carbonite Maps v8.2.0` and `Bartender4`
- **THEN** the labels are `Grid`, `Carbonite Maps` and `Bartender4`

#### Scenario: Missing title

- **WHEN** `EveryAddon/EveryAddon.toc` has no `Title`
- **THEN** the label is `EveryAddon *`

#### Scenario: Non-numeric WoWInterface ID

- **WHEN** a `.toc` has `## X-WoWI-ID: abc`
- **THEN** the addon has no wowinterface source

### Requirement: Directories are grouped by group ID

Addon directories whose nfo data share a group ID SHALL be shown as one addon. The group's details SHALL come from the directory marked primary. When no directory is marked primary, the directory with the lowest name SHALL represent the group and its label SHALL be `<group ID> (group)`. When several are marked primary, the one with the lowest directory name SHALL be used. A directory without valid nfo data SHALL be an addon of its own.

#### Scenario: Bundled addon grouped

- **WHEN** `EveryAddon/` (primary) and `EveryAddon-BundledAddon/` share a group ID
- **THEN** one addon labelled from `EveryAddon` is shown, with both directories as its members

#### Scenario: Group without a primary

- **WHEN** three directories share group ID `everyaddonthree` and none is primary
- **THEN** one addon labelled `everyaddonthree (group)` is shown

#### Scenario: Deterministic primary

- **WHEN** the same addons dir is loaded twice
- **THEN** the same directory represents each group both times

### Requirement: Addons are ignored implicitly

An addon SHALL be implicitly ignored when any of its directories contains a `.git`, `.hg` or `.svn` directory, or when any of its `.toc` files has a `Version` containing `@project-version@`, unless its nfo data explicitly sets ignore to false. An ignored member SHALL make its whole group ignored.

#### Scenario: Git checkout without nfo

- **WHEN** `EveryAddon/` contains `.git/` and has no nfo file
- **THEN** EveryAddon is ignored

#### Scenario: Unrendered version template

- **WHEN** a `.toc` has `## Version: @project-version@`
- **THEN** the addon is ignored

#### Scenario: Explicit un-ignore wins

- **WHEN** `EveryAddon/` contains `.git/` and its nfo data sets ignore to false
- **THEN** EveryAddon is not ignored

#### Scenario: Ignored member ignores the group

- **WHEN** one of two directories in a group is ignored
- **THEN** the group's addon is ignored

### Requirement: Installed addons have stable identities

Loading the same addons dir again SHALL give each addon the same identity as before, derived from the addons dir path and the addon's group ID, or its directory name when it has no group ID. Reloading an addons dir SHALL replace its addons rather than add duplicates, and addons no longer on disk SHALL be removed from the GUI.

#### Scenario: Reload without duplicates

- **WHEN** an addons dir with two addons is loaded twice
- **THEN** exactly two addons are shown

#### Scenario: Deleted outside strongbox

- **WHEN** an addon's directory is deleted by another program and the addons dir is reloaded
- **THEN** the addon is no longer shown

### Requirement: Addons are sorted by label

Installed addons SHALL be presented in case-insensitive order of label, then directory name.

#### Scenario: Sort order

- **WHEN** the addons are labelled `zeta`, `Alpha` and `beta`
- **THEN** they are listed as `Alpha`, `beta`, `zeta`
