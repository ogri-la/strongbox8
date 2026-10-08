## Purpose

Turns WoWInterface addons into updates, using the catalogue's game tracks for them because the WoWInterface API does not report game tracks.

## ADDED Requirements

### Requirement: WoWInterface updates

The update for a WoWInterface addon SHALL come from `https://api.mmoui.com/v3/game/WOW/filedetails/<id>.json`. Its version SHALL be `UIVersion` and its download URL `https://cdn.wowinterface.com/downloads/getfile.php?id=<id>`. Its game tracks SHALL be the catalogue addon's game tracks. When the addon has no catalogue game tracks, its game tracks SHALL be the installed game track from its nfo data, if any. When neither is known, it SHALL have no updates. A 404 response SHALL give no updates.

#### Scenario: Classic-only addon in a retail addons dir

- **WHEN** a catalogue lists a WoWInterface addon for classic only, and a strict retail addons dir is checked
- **THEN** the addon has no update

#### Scenario: Retail and classic

- **WHEN** a catalogue lists a WoWInterface addon for retail and classic, and a strict classic addons dir is checked
- **THEN** the addon's update supports classic

#### Scenario: Not found

- **WHEN** the API responds with HTTP 404
- **THEN** the addon has no updates

### Requirement: WoWInterface URLs

A WoWInterface URL SHALL give the addon ID from a path of the form `info<id>`, `info<id>-<name>.html` or `download<id>-<name>`. Other WoWInterface URLs SHALL be rejected. A WoWInterface addon found by URL SHALL be looked up in the loaded catalogue by ID; one not in the catalogue SHALL NOT be found.

#### Scenario: Info page

- **WHEN** the URL is `https://www.wowinterface.com/downloads/info8882-Name.html`
- **THEN** the addon ID is 8882

#### Scenario: Alternate download page

- **WHEN** the URL is a `dlfile` alternate download page
- **THEN** it is rejected

### Requirement: Downloads that are not zips

A WoWInterface download that responds successfully with a non-zip body, such as an HTML "not yet approved" page, SHALL be rejected as an invalid zip and SHALL NOT be installed.

#### Scenario: Unapproved file

- **WHEN** the download URL responds with HTTP 200 and an HTML page
- **THEN** nothing is installed and the user is told the download was not a valid zip
