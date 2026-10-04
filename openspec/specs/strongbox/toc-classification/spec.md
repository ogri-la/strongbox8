# strongbox/toc-classification Specification

## Purpose

Derives the game tracks a `.toc` file supports from its interface versions and its file name, so installed addons are matched to the right addons dir and update. An interface version that names no supported game track contributes nothing rather than being assumed retail.

## Requirements

### Requirement: Interface versions map strictly to game tracks

An interface version is `major * 10000 + minor * 100 + patch`, and is valid from `10000` to `999999` inclusive. A valid interface version's game track SHALL be:

| major | minor | game track |
|---|---|---|
| 1 | 0–59 | classic |
| 1 | 60–99 | none (forever, not yet supported) |
| 2 | any | classic-tbc |
| 3 | any | classic-wotlk |
| 4 | any | classic-cata |
| 5 | any | none (mists, not yet supported) |
| 6 or more | any | retail |

An invalid interface version SHALL have no game track and no game version. An interface version with no game track SHALL contribute no game track. It SHALL NOT be assumed retail.

#### Scenario: Cata interface version

- **WHEN** the interface version is `40400`
- **THEN** the game track is classic-cata

#### Scenario: Classic interface version

- **WHEN** the interface version is `11503`
- **THEN** the game track is classic

#### Scenario: Forever interface version

- **WHEN** the interface version is `16000`
- **THEN** there is no game track

#### Scenario: Mists interface version

- **WHEN** the interface version is `50500`
- **THEN** there is no game track

#### Scenario: Interface version out of range

- **WHEN** the interface version is `1234` or `1234567`
- **THEN** there is no game track

#### Scenario: Game version from an interface version

- **WHEN** the interface version is `11507`, `16001` or `110002`
- **THEN** the game version is `1.15.7`, `1.60.1` or `11.0.2`

#### Scenario: Retail interface versions

- **WHEN** the interface version is `70000`, `90207` or `110002`
- **THEN** the game track is retail

### Requirement: A .toc's game tracks combine its interface versions and file name

A `.toc` file's game tracks SHALL be the game tracks of all of its interface versions, plus the game track guessed from its file name, if any. When neither gives a game track, the `.toc` SHALL have no game tracks.

#### Scenario: Multiple interface versions

- **WHEN** a `.toc` declares `## Interface: 110002, 40400, 11503`
- **THEN** its game tracks are retail, classic-cata and classic

#### Scenario: Unsupported interface only

- **WHEN** `EveryAddon.toc` declares only `## Interface: 50500`
- **THEN** it has no game tracks

#### Scenario: File name supplies the game track

- **WHEN** `EveryAddon_Cata.toc` declares only `## Interface: 50500`
- **THEN** its game tracks are classic-cata
