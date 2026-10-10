## Purpose

Lets the user move an addon between the hosts it is published on, such as from WoWInterface to GitHub, when the addon lists more than one.

## ADDED Requirements

### Requirement: Source map list

An addon's source map list SHALL be its current source first (its nfo source, otherwise its catalogue match's source), followed by the sources from its nfo data and its `.toc` files, without duplicates and without curseforge or tukui sources.

#### Scenario: Merged list

- **WHEN** the nfo source is wowinterface 123, and the `.toc` lists wowinterface 123 and github `a/b`
- **THEN** the source map list is wowinterface 123, then github `a/b`

### Requirement: Switching source

The user SHALL be able to switch an addon to another source in its source map list when the addon is not ignored and not pinned. Switching SHALL rewrite `source` and `source-id` in every directory's nfo data, keeping the full source map list, and the addon SHALL then be matched and checked against the new source. Switching SHALL be unavailable when the source map list has fewer than two entries.

#### Scenario: Switch to GitHub

- **WHEN** the user switches EveryAddon from wowinterface 123 to github `a/b`
- **THEN** its nfo data's source is github and source ID `a/b`, and its source map list still lists both

#### Scenario: Pinned addon

- **WHEN** the user tries to switch the source of a pinned addon
- **THEN** its nfo data is unchanged
