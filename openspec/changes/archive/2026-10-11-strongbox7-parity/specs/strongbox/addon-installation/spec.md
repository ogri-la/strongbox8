## Purpose

Installs addons into the selected addons dir from the catalogue, from a host URL or from a local zip file, without damaging addons the user has protected or other addons sharing directories.

## ADDED Requirements

### Requirement: Install from the catalogue

The user SHALL be able to install one or several catalogue addons into the selected available addons dir. Each SHALL be expanded into its updates, the update for the addons dir's game track and strictness chosen, downloaded and installed. When no update qualifies, nothing SHALL be installed for that addon and the user SHALL be told which game tracks were searched. A failure for one addon SHALL NOT stop the others. Installing SHALL switch to the installed tab, where installed addons appear as they complete. An addon already installed and matched to the same catalogue addon SHALL be updated rather than installed alongside.

#### Scenario: Install one

- **WHEN** the user installs EveryAddon from the search tab into a retail addons dir
- **THEN** `EveryAddon/` exists in the addons dir with a full nfo file
- **AND** the installed tab shows EveryAddon

#### Scenario: No addons dir

- **WHEN** the user installs a catalogue addon and no available addons dir exists
- **THEN** nothing is downloaded and the user is told to add an addons dir

#### Scenario: Unsupported source

- **WHEN** a catalogue addon's source is `gitplex`
- **THEN** nothing is installed and the user is told the source is unsupported

### Requirement: Install from a URL

The user SHALL be able to install an addon by giving a GitHub, GitLab or WoWInterface URL. The addon SHALL be found at its host, its update chosen relaxed for the selected addons dir's game track, and verified to be a valid addon zip before anything is changed. On success the addon SHALL be added to the user catalogue and installed. A URL for curseforge or tukui, an unrecognised URL, or an addon that cannot be found SHALL install nothing, add nothing to the user catalogue, and tell the user what URL forms are accepted.

#### Scenario: GitHub URL

- **WHEN** the user gives `https://github.com/Aviana/HealComm`
- **THEN** HealComm is installed and added to the user catalogue

#### Scenario: Case-insensitive repository

- **WHEN** the user gives `https://github.com/aviana/healcomm`
- **THEN** the user catalogue entry's source ID is `Aviana/HealComm`

#### Scenario: Curseforge URL

- **WHEN** the user gives a curseforge URL
- **THEN** nothing is installed or added to the user catalogue

#### Scenario: GitHub repository without releases

- **WHEN** the repository has no published releases with zip assets
- **THEN** nothing is installed and the user is told no release was found

### Requirement: Install from a zip file

The user SHALL be able to install one or several local zip files into the selected available addons dir. Each SHALL get a group ID of the zip's file name, without its extension and without anything from the first `--` onwards, followed by `-` and eight random hexadecimal characters. Its primary directory SHALL be chosen as for any install, and each of its directories SHALL get a grouping-only nfo object. Installing a zip over an ignored or pinned addon SHALL be allowed, keeping it ignored and removing its pin. An addon installed from a zip SHALL later be matched and updated like any other.

#### Scenario: Group ID from file name

- **WHEN** the user installs `/downloads/everyaddon--1-2-3.zip`
- **THEN** the group ID is `everyaddon-` followed by eight hexadecimal characters

#### Scenario: Then updated from the catalogue

- **WHEN** an addon installed from a zip matches a catalogue addon with a newer update
- **THEN** it is offered the update, and updating writes a full nfo object

#### Scenario: Over an ignored addon

- **WHEN** the user installs a zip whose directory belongs to an ignored addon
- **THEN** the zip is installed and the addon remains ignored

### Requirement: Zip validation

Before installing, a zip SHALL be refused, with the reason given to the user, when it cannot be opened, when it contains any top-level file, when it contains no top-level directory, or when a top-level directory has no `.toc` file directly inside it. At most three offending names SHALL be listed. A downloaded zip that is refused SHALL be deleted. A local zip chosen by the user SHALL NOT be deleted. Entries that would extract outside the addons dir SHALL cause the zip to be refused.

#### Scenario: Top-level file

- **WHEN** a zip contains only `empty.txt`
- **THEN** it is refused because it contains top-level files

#### Scenario: Non-addon directory

- **WHEN** a zip contains `EveryAddon/EveryAddon.toc` and `__MACOS/`
- **THEN** it is refused, naming `__MACOS`

#### Scenario: Truncated download

- **WHEN** a downloaded zip is truncated
- **THEN** nothing is extracted and the downloaded file is deleted

#### Scenario: Path traversal

- **WHEN** a zip contains `EveryAddon/../../evil.lua`
- **THEN** it is refused and nothing is extracted

### Requirement: Protected addons are not overwritten

An install from the catalogue or a URL SHALL be refused when the zip's top-level directories include a directory of an ignored addon or of a pinned addon. The refusal SHALL name the reason. Nothing SHALL be changed.

#### Scenario: Overwriting an ignored bundled addon

- **WHEN** a zip contains `EveryAddon-BundledAddon/`, which belongs to an ignored addon
- **THEN** the install is refused and both directories are unchanged

#### Scenario: Overwriting a pinned addon

- **WHEN** a zip's directory belongs to a pinned addon
- **THEN** the install is refused

### Requirement: Installation order

An install SHALL, in order: uninstall the previous version of the addon being installed; uninstall every other non-ignored addon whose directories are all among the zip's top-level directories; extract the zip into the addons dir; write nfo data into every top-level directory. The directory whose name is the shortest and a prefix of every other top-level directory SHALL be marked primary; when there is none, no directory SHALL be primary. Installs into one addons dir SHALL NOT run concurrently with other installs or removals in that addons dir.

#### Scenario: Obsolete bundled directory removed on upgrade

- **WHEN** version 0.1.2 installed `EveryAddon/` and `EveryAddon-BundledAddon/`, and version 1.2.3 contains only `EveryAddon/`
- **THEN** after updating, `EveryAddon-BundledAddon/` no longer exists

#### Scenario: Completely overwritten addons uninstalled

- **WHEN** addon A owns `EveryAddonOne/`, addon B owns `EveryAddonTwo/`, and addon C's zip contains `EveryAddonOne/`, `EveryAddonTwo/` and `EveryAddonThree/`
- **THEN** after installing C, A and B are no longer installed and each directory's nfo data holds only C

#### Scenario: Partial overwrite becomes shared

- **WHEN** EveryOtherAddon's zip contains `EveryOtherAddon/` and `EveryAddon-BundledAddon/`, and EveryAddon owns `EveryAddon/` and `EveryAddon-BundledAddon/`
- **THEN** `EveryAddon-BundledAddon/` is shared by both addons

#### Scenario: Primary directory

- **WHEN** a zip contains `HealBot/`, `HealBot_de/` and `HealBot_Tips/`
- **THEN** `HealBot/` is marked primary

#### Scenario: Unzip failure stops installation

- **WHEN** extraction fails part way
- **THEN** no nfo data is written and the user is told the install failed

### Requirement: Suspicious bundles are reported

When a zip's top-level directories fall into two or more prefix groups by their first three characters, the smallest group has three or fewer directories, and the groups are not all the same size, the user SHALL be told the addon will also install every directory outside the largest group. Several groups of more than three directories each, such as Altoholic's `Altoholic*` and `DataStore*`, SHALL NOT be reported.

#### Scenario: Auctioneer

- **WHEN** a zip contains Auc-Advanced, Auc-Stat-Histogram, BeanCounter, Enchantrix and SlideBar
- **THEN** the user is told BeanCounter, Enchantrix and SlideBar will also be installed

### Requirement: Downloaded zips

Downloaded zips SHALL be saved in the addons dir as `<name>--<version>.zip`, with name and version made safe for file names. After a successful install, when `addon-zips-to-keep` is a number N, only the N most recently modified zips for that addon SHALL be kept and the rest deleted; when it is null, all SHALL be kept. Only files inside the addons dir matching that addon's zip name pattern SHALL be deleted.

#### Scenario: Keep none

- **WHEN** `addon-zips-to-keep` is 0 and an addon is installed
- **THEN** its downloaded zip is deleted after installing

#### Scenario: Keep three

- **WHEN** `addon-zips-to-keep` is 3 and five zips for EveryAddon exist after installing
- **THEN** the three newest remain
- **AND** zips for other addons are untouched
