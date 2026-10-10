# strongbox/catalogue-matching Specification

## Purpose

Identifies which catalogue addon each installed addon is, so strongbox knows where to look for its updates.

## Requirements

### Requirement: Ordered match rules

Each installed addon that is not ignored SHALL be matched against the loaded catalogue by trying these rules in order, the first hit winning:

1. installed source and source ID equal a catalogue addon's source and source ID;
2. installed name equals a catalogue addon's name;
3. installed label equals a catalogue addon's label;
4. installed directory name equals a catalogue addon's label.

A rule whose installed value is empty SHALL be skipped. When a rule matches several catalogue addons, the first in catalogue order SHALL win.

#### Scenario: Source and ID beats name

- **WHEN** an installed addon has source github and source ID `a/b`, and the catalogue has a github `a/b` addon and a differently-sourced addon of the same name
- **THEN** it matches the github `a/b` addon

#### Scenario: Directory name fallback

- **WHEN** an installed addon without nfo data is in `AdiBags/` and its `.toc` title does not match, and the catalogue has an addon labelled `AdiBags`
- **THEN** it matches that addon

#### Scenario: Source compared to name is not a rule

- **WHEN** an installed addon has source `github` and no source ID, and the catalogue has an addon named `github`
- **THEN** it is not matched by that addon

### Requirement: Ignored addons are not matched

An ignored addon SHALL NOT be matched to the catalogue and SHALL keep its installed details.

#### Scenario: Ignored addon

- **WHEN** an ignored addon's name equals a catalogue addon's name
- **THEN** the addon is shown unmatched

### Requirement: Matched details

A matched addon SHALL show the catalogue addon's label, URL, tags and created date, and the catalogue description when it is not empty, otherwise the `.toc` description. When the installed nfo data names a source different from the matched catalogue addon's source, the addon SHALL keep the nfo source for updates.

#### Scenario: Empty catalogue description

- **WHEN** a matched catalogue addon has no description
- **THEN** the `.toc` description is shown

### Requirement: Unmatched addons with nfo data are still checkable

An unmatched, non-ignored addon whose nfo data has a supported source and source ID SHALL still be checked for updates from that source.

#### Scenario: Addon removed from the catalogue

- **WHEN** an addon installed from github by strongbox is no longer in the catalogue
- **THEN** it is still checked for updates on github

### Requirement: Unmatched addons are reported

Each unmatched, non-ignored addon SHALL be logged at INFO level once per match, and a summary of how many installed addons were matched SHALL be logged at INFO level.

#### Scenario: Summary

- **WHEN** 10 addons are installed and 8 are matched
- **THEN** an INFO message reports 8 of 10 addons matched
