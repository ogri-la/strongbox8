# strongbox/toc-selection Specification

## Purpose

Chooses which of an installed addon's `.toc` files describes it for an addons dir's game track, so the addon's displayed details match the game track the addons dir runs.

## Requirements

### Requirement: Strict mode uses only a matching .toc

When the addons dir is strict, the `.toc` used SHALL be one that supports the addons dir's game track. When several support it, the one supporting the fewest game tracks SHALL be used, and among those the one with the lowest file path. When no `.toc` supports it, no `.toc` SHALL be used.

#### Scenario: Matching .toc present

- **WHEN** a strict classic addons dir holds an addon with a retail `.toc` and a classic `.toc`
- **THEN** the classic `.toc` is used

#### Scenario: Most specific .toc wins

- **WHEN** a strict classic-cata addons dir holds an addon with `EveryAddon.toc` supporting retail, classic-cata and classic, and `EveryAddon_Cata.toc` supporting only classic-cata
- **THEN** `EveryAddon_Cata.toc` is used

#### Scenario: Equally specific .toc files

- **WHEN** a strict classic addons dir holds an addon with `EveryAddon_Classic.toc` and `EveryAddon_Vanilla.toc`, each supporting only classic
- **THEN** `EveryAddon_Classic.toc` is used, every time

#### Scenario: No matching .toc

- **WHEN** a strict classic addons dir holds an addon with only a retail `.toc`
- **THEN** no `.toc` is used

### Requirement: Relaxed mode uses the most-preferred available .toc

When the addons dir is not strict, the `.toc` used SHALL support the earliest game track in the addons dir's game track preference order that any of the addon's `.toc` files supports. A less-preferred game track SHALL NOT be chosen when a more-preferred one is available. When several `.toc` files support that game track, the strict-mode tie-break SHALL apply.

#### Scenario: Preferred game track present

- **WHEN** a relaxed classic-wotlk addons dir holds an addon with classic-cata, classic-tbc and classic `.toc` files and no classic-wotlk one
- **THEN** the classic-cata `.toc` is used

#### Scenario: Exact game track present

- **WHEN** a relaxed retail addons dir holds an addon with retail and classic `.toc` files
- **THEN** the retail `.toc` is used

### Requirement: Game track preference order

Each game track SHALL have a fixed preference order, used by relaxed mode to fall back when its own game track is unavailable:

| addons dir game track | preference order |
|---|---|
| retail | retail, classic, classic-tbc, classic-wotlk, classic-cata, classic-mists |
| classic | classic, classic-tbc, classic-wotlk, classic-cata, classic-mists, retail |
| classic-tbc | classic-tbc, classic-wotlk, classic-cata, classic-mists, classic, retail |
| classic-wotlk | classic-wotlk, classic-cata, classic-mists, classic-tbc, classic, retail |
| classic-cata | classic-cata, classic-mists, classic-wotlk, classic-tbc, classic, retail |
| classic-mists | classic-mists, classic-cata, classic-wotlk, classic-tbc, classic, retail |
| forever | forever, retail |

forever SHALL NOT appear in the preference order of any other game track. The same order SHALL be used to choose a `.toc` and to choose an update.

#### Scenario: Mists falls back to cata

- **WHEN** a relaxed classic-mists addons dir holds an addon with classic-cata and classic `.toc` files and no classic-mists one
- **THEN** the classic-cata `.toc` is used

#### Scenario: Forever falls back only to retail

- **WHEN** a relaxed forever addons dir holds an addon with only a classic `.toc`
- **THEN** no `.toc` is used

#### Scenario: Forever is never a fallback

- **WHEN** a relaxed retail addons dir holds an addon with only a forever `.toc`
- **THEN** no `.toc` is used
