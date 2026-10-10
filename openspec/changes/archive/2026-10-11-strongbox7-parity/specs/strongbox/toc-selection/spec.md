## ADDED Requirements

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
