## Purpose

Defines the `.strongbox.json` file strongbox keeps in each addon directory it installs, so strongbox 7 and strongbox 8 can both read what the other wrote.

## ADDED Requirements

### Requirement: nfo file shape

The nfo file SHALL be `.strongbox.json` in an addon directory. It SHALL hold either one nfo object, or a JSON array of nfo objects when several addons share the directory, ordered oldest first. A full nfo object SHALL have these keys:

| key | type |
|---|---|
| `installed-version` | string |
| `name` | string |
| `group-id` | string |
| `primary?` | boolean |
| `source` | string |
| `source-id` | string or integer |
| `installed-game-track` | one of the supported game tracks |
| `source-map-list` | list of `{source, source-id}` |
| `ignore?` | optional boolean |
| `pinned-version` | optional string |

A grouping-only nfo object (for an addon installed from a zip file) SHALL have only `group-id` and `primary?`, plus optional `ignore?` and `pinned-version`. An ignore-only nfo object SHALL have only `ignore?`.

#### Scenario: Read an integer source ID

- **WHEN** an nfo file has `"source-id": 321`
- **THEN** the addon's source ID is `321`

#### Scenario: Read a v1 nfo

- **WHEN** an nfo object has `source` and `source-id` but no `source-map-list`
- **THEN** its source map list is that source and source ID

### Requirement: nfo files are written so strongbox 7 can read them

Strongbox 8 SHALL write a single nfo object, not an array, when one addon owns the directory, and an array only when more than one addon shares it. It SHALL write only the keys listed in the nfo file shape, SHALL write `installed-game-track` only with a supported game track, and SHALL validate nfo data before writing. Invalid nfo data SHALL NOT be written, and SHALL be reported as a program error at ERROR level.

#### Scenario: Single addon writes an object

- **WHEN** EveryAddon is installed into an empty addons dir
- **THEN** `EveryAddon/.strongbox.json` contains a JSON object

#### Scenario: Shared directory writes an array

- **WHEN** EveryOtherAddon is installed and replaces `EveryAddon-BundledAddon/` belonging to EveryAddon
- **THEN** `EveryAddon-BundledAddon/.strongbox.json` contains an array of two objects, EveryOtherAddon's last

#### Scenario: Strongbox 8 pre-release array of one

- **WHEN** an nfo file holds an array containing one nfo object
- **THEN** it is read as one addon owning the directory, not as a shared directory
- **AND** the next write stores a single object

### Requirement: Invalid nfo files are reported, not deleted

An nfo file that is not valid JSON, or that matches none of the nfo shapes, SHALL be logged at WARN level naming the file and SHALL be treated as absent. Strongbox SHALL NOT delete it while reading.

#### Scenario: Corrupt nfo

- **WHEN** `EveryAddon/.strongbox.json` contains `{}`
- **THEN** EveryAddon loads from its `.toc` data alone
- **AND** the file still exists

### Requirement: Shared directories keep a stack of owners

When an installed addon claims a directory that another addon's nfo data already claims, its nfo object SHALL be appended to that directory's nfo data, replacing any earlier entry with the same group ID, and the user SHALL be told which addon replaced which directory, in the form `"<new name>" (<new version>) replaced directory "<dir>" of addon "<old name>" (<old version>)`. The last entry SHALL describe the directory.

#### Scenario: Replacement message

- **WHEN** everyotheraddon 5.6.7 replaces `EveryAddon-BundledAddon` of everyaddon 0.1.2
- **THEN** the user is told `"everyotheraddon" (5.6.7) replaced directory "EveryAddon-BundledAddon" of addon "everyaddon" (0.1.2)`

#### Scenario: Reinstall does not stack

- **WHEN** an addon is installed twice into the same directory
- **THEN** that directory's nfo data holds one entry for it

### Requirement: Ignore flag editing

Setting ignore SHALL write `ignore?: true` into the nfo data of every directory of the addon, creating ignore-only nfo files where none exist. Clearing ignore SHALL remove the `ignore?` key; when the addon is also implicitly ignored it SHALL instead write `ignore?: false`. An nfo file left with no keys SHALL be deleted. When an implicitly ignored directory's nfo data is only `ignore?: false`, ignoring it again SHALL delete the nfo file so it reverts to implicitly ignored.

#### Scenario: Ignore an unmanaged addon

- **WHEN** the user ignores an addon with no nfo file
- **THEN** its directory gets an nfo file containing only `{"ignore?": true}`

#### Scenario: Stop ignoring a git checkout

- **WHEN** the user stops ignoring an addon whose directory contains `.git/`
- **THEN** its nfo data has `ignore?: false` and the addon is no longer ignored

#### Scenario: Stop ignoring removes empty nfo

- **WHEN** the user stops ignoring an addon whose nfo file is only `{"ignore?": true}`
- **THEN** the nfo file is deleted
