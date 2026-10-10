# strongbox/user-catalogue Specification

## Purpose

Keeps the user's own catalogue of addons: favourites and addons imported by URL, preserved across catalogue switches and refreshed when asked.

## Requirements

### Requirement: User catalogue file

The user catalogue SHALL be stored as `user-catalogue.json` in the config directory, in the catalogue format. It SHALL be written only with valid data, with today's date as its datestamp, atomically. An unreadable user catalogue SHALL be logged at WARN level, treated as empty, and SHALL NOT be overwritten until the user next changes the user catalogue; before that write it SHALL be preserved as `user-catalogue.json.<timestamp>.invalid`.

#### Scenario: Missing file

- **WHEN** no user catalogue file exists
- **THEN** the user catalogue is empty and no file is created until an addon is added

### Requirement: Starring catalogue addons

The user SHALL be able to add a catalogue addon to the user catalogue, and remove one from it. An installed addon matched to the catalogue SHALL be addable the same way. Adding an addon already present SHALL leave one entry. Removing an addon not present SHALL change nothing. Each change SHALL be saved. Addons in the user catalogue SHALL be shown as starred in the search tab.

#### Scenario: Add three times

- **WHEN** the user adds the same addon three times
- **THEN** the user catalogue holds one entry for it

#### Scenario: Unmatched installed addon

- **WHEN** the user tries to star an installed addon that is not matched to the catalogue
- **THEN** the user catalogue is unchanged and the user is told the addon has no catalogue entry

### Requirement: Refreshing the user catalogue

The user SHALL be able to refresh the user catalogue. Each entry SHALL be replaced by the matching entry from the full catalogue, by source and source ID, or failing that by looking the addon up at its host by URL. An entry that cannot be found SHALL be kept unchanged. A failure looking up one entry SHALL NOT stop the others. The refreshed user catalogue SHALL be written once at the end.

#### Scenario: Newer data pulled

- **WHEN** a user catalogue entry has 10 downloads and the full catalogue lists the same addon with 20
- **THEN** after refreshing, the entry has 20 downloads

#### Scenario: Host lookup fails

- **WHEN** one entry's host returns an error
- **THEN** that entry is unchanged and the other entries are refreshed

### Requirement: Scheduled refresh

When the `keep-user-catalogue-updated` preference is on and the user catalogue's datestamp is more than 28 days old, the startup refresh SHALL refresh the user catalogue after checking for updates.

#### Scenario: Stale user catalogue

- **WHEN** the preference is on and the user catalogue is dated 40 days ago
- **THEN** the user catalogue is refreshed during the startup refresh

#### Scenario: Preference off

- **WHEN** the preference is off and the user catalogue is dated 40 days ago
- **THEN** the user catalogue is not refreshed
