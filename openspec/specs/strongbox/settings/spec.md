# strongbox/settings Specification

## Purpose

Locates, imports, migrates, validates and saves strongbox 8's settings so that a strongbox 7 user's configuration carries over without effort and no settings file is ever lost or silently replaced.

## Requirements

### Requirement: Strongbox 8 has its own directories

Strongbox 8 SHALL keep its settings in a config directory and its catalogues, cache and other data in a data directory, separate from strongbox 7's:

| directory | `XDG_*_HOME` unset or empty | `XDG_*_HOME` set |
|---|---|---|
| config | `~/.config/strongbox8` | `$XDG_CONFIG_HOME/strongbox8` |
| data | `~/.local/share/strongbox8` | `$XDG_DATA_HOME/strongbox8` |

A relative `XDG_*_HOME` SHALL be treated as unset. An `XDG_*_HOME` whose last segment is already `strongbox8` SHALL be used as it is, as strongbox 7 did with `strongbox`. The settings file SHALL be `config.json` and the user catalogue `user-catalogue.json`, both in the config directory. Every path SHALL be absolute. Strongbox 8 SHALL NOT write to strongbox 7's directories.

#### Scenario: Default paths

- **WHEN** `XDG_CONFIG_HOME` and `XDG_DATA_HOME` are unset
- **THEN** the settings file is `~/.config/strongbox8/config.json`
- **AND** catalogues are stored under `~/.local/share/strongbox8`

#### Scenario: XDG paths

- **WHEN** `XDG_CONFIG_HOME` is `/tmp/cfg` and `XDG_DATA_HOME` is `/tmp/data`
- **THEN** the settings file is `/tmp/cfg/strongbox8/config.json`
- **AND** catalogues are stored under `/tmp/data/strongbox8`

#### Scenario: Strongbox 7 directory untouched

- **WHEN** `XDG_CONFIG_HOME` is `/tmp/cfg` and `/tmp/cfg/strongbox/config.json` exists
- **THEN** strongbox 8 never modifies, moves or deletes `/tmp/cfg/strongbox/config.json`

#### Scenario: Unwriteable data directory

- **WHEN** the data directory does not exist and cannot be created, or exists and is not writeable
- **THEN** strongbox fails to start and reports which directory is unusable

### Requirement: Strongbox 7 settings are imported once

When strongbox 8's settings file does not exist and strongbox 7's settings file does, strongbox 8 SHALL read strongbox 7's settings file, migrate it and save the result as its own settings file. When strongbox 8's user catalogue does not exist and strongbox 7's does, it SHALL be copied. The import SHALL be logged at INFO level naming the source files. Strongbox 7's files SHALL only be read. When strongbox 8's settings file exists, strongbox 7's files SHALL NOT be read.

Strongbox 7's settings file is `$XDG_CONFIG_HOME/strongbox/config.json`, or `~/.config/strongbox/config.json` when `XDG_CONFIG_HOME` is unset, empty or relative. When `XDG_CONFIG_HOME` already ends in `strongbox`, it is `$XDG_CONFIG_HOME/config.json`, matching strongbox 7.

#### Scenario: First run after strongbox 7

- **WHEN** strongbox 8 starts with no strongbox 8 settings file, and strongbox 7's settings file lists two addons dirs, selects the second and selects the `full` catalogue
- **THEN** strongbox 8's settings file is written with both addons dirs, the second selected and the `full` catalogue selected
- **AND** strongbox 7's settings file is byte-for-byte unchanged

#### Scenario: Later runs ignore strongbox 7

- **WHEN** strongbox 8's settings file exists and strongbox 7's settings file has since gained a third addons dir
- **THEN** strongbox 8 lists only the addons dirs in its own settings file

#### Scenario: Unreadable strongbox 7 settings

- **WHEN** strongbox 8 has no settings file and strongbox 7's settings file is not valid JSON
- **THEN** strongbox 8 starts with default settings
- **AND** logs a WARN naming strongbox 7's settings file
- **AND** strongbox 7's settings file is unchanged

#### Scenario: User catalogue copied

- **WHEN** strongbox 8 has no user catalogue and strongbox 7's user catalogue holds three addons
- **THEN** strongbox 8's user catalogue holds the same three addons

#### Scenario: Fresh install

- **WHEN** neither strongbox 8 nor strongbox 7 settings files exist
- **THEN** strongbox 8 starts with default settings and writes them to its settings file

### Requirement: Every historical settings version migrates

Settings written by any strongbox 7 version from 0.9 to 7.x, or by an earlier strongbox 8 pre-release, SHALL migrate to the current settings shape:

| old form | migrated form |
|---|---|
| `install-dir` | appended to the addons dir list as retail and strict, unless already listed |
| `selected-catalog` | `selected-catalogue`, unless `selected-catalogue` is set |
| top-level `selected-catalogue`, `selected-addon-dir`, `gui-theme` | the matching preference |
| addons dir `strict?` | addons dir `strict` |
| addons dir without `strict?` or `strict` | strict, except for compound game tracks |
| game track `retail-classic` | game track `retail`, not strict |
| game track `classic-retail` | game track `classic`, not strict |
| catalogue list equal to the 0.x–4.x default list (`short`, `full`, `tukui`, `curseforge`, `wowinterface`) | the current default catalogue list |
| catalogue entries named `curseforge` or `tukui` | removed |
| selected columns equal to the v1 default column set | the current default column list |
| `debug?` and any other unknown top-level key | removed, logged at DEBUG |

A missing catalogue list SHALL become the default list. An empty catalogue list SHALL be kept empty. A selected catalogue that is not in the catalogue list SHALL fall back to the first catalogue in the list. A selected addons dir that is not in the addons dir list SHALL fall back to the first available addons dir.

#### Scenario: Each historical fixture

- **WHEN** each of the settings fixtures for versions 0.9, 0.10, 0.11, 0.12, 1.0, 3.1, 3.2, 4.1, 4.7, 4.9, 5.0, 6.0, 7.0 and the 8.0 pre-release is loaded
- **THEN** the result equals that fixture's expected migrated settings

#### Scenario: Compound game track

- **WHEN** an addons dir has game track `retail-classic` and no strictness
- **THEN** it migrates to game track retail, not strict

#### Scenario: Strictness kept

- **WHEN** a strongbox 7 addons dir has game track `retail` and `strict?` false
- **THEN** it migrates to game track retail, not strict

#### Scenario: Dead catalogues removed

- **WHEN** the catalogue list contains `short`, `curseforge` and `tukui` entries
- **THEN** the migrated catalogue list contains only `short`

#### Scenario: Empty catalogue list kept

- **WHEN** the catalogue list is `[]`
- **THEN** the migrated catalogue list is empty and no catalogue is downloaded

### Requirement: Settings are validated, and invalid parts are rejected individually

After migration the settings SHALL be validated against a schema. An invalid individual entry SHALL be discarded and logged at WARN level with the reason, leaving the rest of the settings in effect:

- an addons dir entry with an empty or relative path, or a game track that is not a supported game track;
- a duplicate addons dir entry (same path), after the first;
- a catalogue location without a name, or without an absolute `https` source URL, or with a duplicate name;
- a preference value of the wrong type, which SHALL take its default.

An addons dir whose path does not currently exist SHALL be kept in the settings and SHALL NOT be discarded.

#### Scenario: Unknown game track discarded

- **WHEN** one addons dir has game track `classic-bfa` and another has game track `retail`
- **THEN** only the retail addons dir is loaded
- **AND** a WARN names the discarded addons dir and its game track

#### Scenario: Missing directory kept

- **WHEN** an addons dir's path does not exist at startup
- **THEN** it remains in the settings file after the next save

#### Scenario: Bad preference type

- **WHEN** `addon-zips-to-keep` is the string `"three"`
- **THEN** it takes its default and the other preferences are kept

### Requirement: Supported preferences

The settings SHALL support these preferences, with these defaults when absent:

| preference | type | default | meaning |
|---|---|---|---|
| `selected-addon-dir` | path or empty | first available addons dir | the selected addons dir |
| `selected-catalogue` | catalogue name | first catalogue in the list | the catalogue to search and match against |
| `addon-zips-to-keep` | non-negative integer or null | null | downloaded zips to keep per addon; null keeps all |
| `keep-user-catalogue-updated` | boolean | false | refresh the user catalogue when older than 28 days |
| `check-for-update` | boolean | true | check for a newer strongbox release at startup |
| `ui-selected-columns` | list of column names | the default column list | columns shown in the installed tab |
| `selected-gui-theme` | theme name | `light` | preserved; has no effect in strongbox 8 |

#### Scenario: Defaults

- **WHEN** settings contain no preferences
- **THEN** `addon-zips-to-keep` is null, `keep-user-catalogue-updated` is false and `check-for-update` is true

### Requirement: Settings files are never lost

Strongbox SHALL NOT overwrite a settings file it could not read. When the settings file exists but is not valid JSON, or its top level is not an object, it SHALL be copied to `config.json.<timestamp>.invalid` beside it before anything is written, strongbox SHALL start with default settings, and an ERROR SHALL be logged naming both files. Settings SHALL be written to a temporary file in the same directory and renamed over the settings file, so an interrupted save never leaves a partial file.

#### Scenario: Corrupt settings file preserved

- **WHEN** the settings file contains `{"addon-dir-list": [`
- **THEN** a copy with the same contents exists as `config.json.<timestamp>.invalid`
- **AND** strongbox starts with default settings

#### Scenario: Atomic save

- **WHEN** settings are saved
- **THEN** the settings file contains either the previous settings or the new settings, never a mixture or a truncated file

### Requirement: Settings from a newer strongbox are not downgraded

Strongbox 8 SHALL write a settings schema version in its settings file. When it reads a settings file with a schema version greater than it supports, it SHALL use the settings it understands, SHALL NOT write the settings file during that session, and SHALL log a WARN saying the settings were written by a newer strongbox.

#### Scenario: Newer schema version

- **WHEN** the settings file declares a schema version one greater than supported
- **THEN** the addons dirs it lists are loaded
- **AND** the settings file is unchanged when strongbox stops

### Requirement: Settings changes are saved when made

A change to addons dirs, the selected addons dir, an addons dir's game track or strictness, the selected catalogue or any preference SHALL be saved to the settings file when the change is made, not deferred to a later refresh.

#### Scenario: Game track change saved

- **WHEN** the user changes the selected addons dir's game track to classic
- **THEN** the settings file records game track classic for that addons dir before any refresh completes
