## Purpose

Lets the user find addons in the loaded catalogue to install.

## ADDED Requirements

### Requirement: Text search

The search tab SHALL list the catalogue addons whose label, name or description contains the search text, compared case-insensitively. Empty search text SHALL list every catalogue addon. Results SHALL keep catalogue order. Already installed addons SHALL be distinguishable from others.

#### Scenario: Substring of label

- **WHEN** the catalogue holds Chinchilla and the search text is `chin`
- **THEN** Chinchilla is listed

#### Scenario: Description match

- **WHEN** an addon's description contains `Auction House` and the search text is `auction`
- **THEN** the addon is listed

#### Scenario: Empty catalogue

- **WHEN** no catalogue is loaded and the user searches
- **THEN** no results are listed and no error is shown

### Requirement: Search reflects the loaded catalogue

The search results SHALL be replaced, not appended, when the catalogue is switched or reloaded, and SHALL include user catalogue addons.

#### Scenario: Switch catalogue

- **WHEN** the user switches from the short to the github catalogue
- **THEN** only the github catalogue's addons, plus user catalogue addons, are listed
