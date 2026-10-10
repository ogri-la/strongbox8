## Purpose

Downloads, keeps fresh and loads the catalogue of known addons that installed addons are matched against and that the user searches and installs from.

## ADDED Requirements

### Requirement: Catalogue locations

A catalogue location SHALL have a name, a label and an absolute `https` source URL. The default catalogue locations SHALL be, in order:

| name | label |
|---|---|
| short | Short (default) |
| full | Full |
| wowinterface | WoWInterface |
| github | GitHub |

each with source `https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/<name>-catalogue.json`. The first location SHALL be the default.

#### Scenario: Default list

- **WHEN** strongbox starts with default settings
- **THEN** the catalogue locations are short, full, wowinterface and github, and short is selected

### Requirement: Switching catalogue

The user SHALL be able to select any catalogue location. Selecting one SHALL save the settings, load that catalogue (downloading it when needed), and re-match installed addons against it.

#### Scenario: Switch to full

- **WHEN** the user selects the full catalogue
- **THEN** the search tab lists the full catalogue's addons
- **AND** the settings file records `full` as the selected catalogue

### Requirement: Catalogue freshness

The selected catalogue SHALL be stored locally as `<name>-catalogue.json` in the data directory. It SHALL be downloaded when no local copy exists, or when the local copy is older than one hour at refresh time. A failed download SHALL leave any existing local copy unchanged and SHALL NOT leave a partial or empty file behind. A failure SHALL be logged at WARN level with the host's response, and the existing local copy SHALL still be used.

#### Scenario: Fresh copy reused

- **WHEN** the local catalogue was downloaded ten minutes ago and the user refreshes
- **THEN** the catalogue is not downloaded again

#### Scenario: Stale copy refreshed

- **WHEN** the local catalogue is two hours old and the user refreshes
- **THEN** the catalogue is downloaded again

#### Scenario: Server error keeps the old copy

- **WHEN** the catalogue host responds with HTTP 500
- **THEN** the previous local catalogue is unchanged and used
- **AND** a WARN reports the failure

#### Scenario: Server error with no copy

- **WHEN** no local catalogue exists and the host responds with HTTP 500
- **THEN** no catalogue file is created
- **AND** installed addons are still shown, unmatched

### Requirement: Corrupt catalogues are downloaded again

When the local catalogue cannot be parsed or fails validation, it SHALL be deleted and downloaded once more. When the new copy is also unusable, the selected catalogue SHALL NOT be loaded, an ERROR SHALL be logged, and strongbox SHALL keep running. The user catalogue's addons SHALL still be listed and matched.

#### Scenario: Empty local file

- **WHEN** the local catalogue file is empty and the host serves a valid catalogue
- **THEN** the valid catalogue is loaded

#### Scenario: Bad twice

- **WHEN** the local catalogue is corrupt and the host also serves corrupt data
- **THEN** none of the selected catalogue's addons are loaded and strongbox does not crash

### Requirement: Catalogue validation

A catalogue SHALL be a JSON object with `spec.version` 2, a `datestamp`, a `total`, and an `addon-summary-list`. `total` SHALL equal the number of entries. Each entry SHALL have `url`, `name`, `label`, `updated-date`, `source`, `source-id` and `game-track-list`, and MAY have `description`, `created-date`, `download-count` and `tag-list`, matching what the strongbox catalogue builder writes. An absent `download-count` SHALL be read as 0 and an absent `tag-list` as empty. Game track values that are not supported game tracks SHALL be removed from `game-track-list` with a DEBUG log. An entry that fails validation SHALL be discarded with a DEBUG log, leaving the rest. Entries from curseforge and tukui SHALL be discarded. Descriptions longer than 255 characters SHALL be truncated to 255.

#### Scenario: Total mismatch

- **WHEN** a catalogue's `total` is 5 and it lists 4 addons
- **THEN** the catalogue fails validation

#### Scenario: Optional fields absent

- **WHEN** an entry has no `tag-list` and no `download-count`
- **THEN** the entry is loaded with no tags and 0 downloads

#### Scenario: Forever and mists game tracks

- **WHEN** an entry's `game-track-list` is `["forever", "classic-mists"]`
- **THEN** the entry supports forever and classic-mists

#### Scenario: Long description

- **WHEN** an entry's description is 300 characters
- **THEN** the loaded description is its first 255 characters

### Requirement: The user catalogue and the selected catalogue are combined by whole entries

The loaded catalogue SHALL contain every entry of the selected catalogue and every entry of the user catalogue, keyed by source and source ID. Where both have an entry for the same key, the selected catalogue's entry SHALL be used whole, and the user catalogue's entry SHALL NOT contribute any field to it. An entry's fields SHALL always come from a single catalogue.

#### Scenario: User-only addon searchable

- **WHEN** the user catalogue holds an addon absent from the selected catalogue
- **THEN** that addon can be found in search and matched to installed addons

#### Scenario: Selected catalogue entry wins whole

- **WHEN** the user catalogue's entry for github `a/b` has a description and the selected catalogue's entry for github `a/b` has none
- **THEN** the loaded entry for github `a/b` has no description
