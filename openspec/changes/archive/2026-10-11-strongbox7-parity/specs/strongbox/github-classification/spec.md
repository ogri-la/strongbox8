## ADDED Requirements

### Requirement: A lone unhinted zip takes the addon's known game tracks

After the earlier passes, when a release has exactly one installable asset and it is still unclassified, it SHALL take the game tracks the addon is known to support: its catalogue entry's game tracks, otherwise the game track recorded in its nfo data when it was installed. When the addon has no known game tracks, or the release has more than one installable asset, this pass SHALL change nothing.

#### Scenario: Single zip with catalogue game tracks

- **WHEN** a release published after WoW Classic has only `Addon-1.2.3.zip`, with no game track in its name, the release name or a `release.json`, and the catalogue lists the addon for retail and classic
- **THEN** the release produces one update supporting retail and classic

#### Scenario: Single zip with an installed game track

- **WHEN** the release has only an unhinted zip, the addon is not in the catalogue, and its nfo data records it was installed for classic-tbc
- **THEN** the release produces one update supporting classic-tbc

#### Scenario: Several unhinted zips

- **WHEN** a release has two unhinted zips and the catalogue lists the addon for retail
- **THEN** neither zip takes the known game tracks

## MODIFIED Requirements

### Requirement: Game tracks are guessed from names

A game track SHALL be guessed from a string by checking, in order:

1. an exact match against a known alias: `retail`, `mainline`, `classic`, `vanilla`, `classic-tbc`, `tbc`, `bcc`, `classic-wotlk`, `wrath`, `wotlk`, `classic-cata`, `cata`, `cataclysm`, `classic-mists`, `mists`, `mop`, `forever`, `camelot`, `wotlkc`
2. `forever` or `camelot` as a delimited word, giving forever
3. `mists` as a delimited word, giving classic-mists
4. `cata` as a delimited word, giving classic-cata
5. `wrath` or `wotlk`, giving classic-wotlk
6. a `tbc`/`bc`/`bcc` form, giving classic-tbc
7. `classic` or `vanilla`, giving classic
8. `retail` or `mainline`, or `standard` as a delimited word, giving retail

A word is delimited when the start of the string or a non-alphanumeric character comes before it, and the end of the string or a non-alphanumeric character comes after it. A string matching none of these SHALL have no game track.

#### Scenario: Cata in an asset file name

- **WHEN** the string is `WeakAuras-5.0.1-cata.zip`
- **THEN** the guessed game track is classic-cata

#### Scenario: Cata wins over wotlk

- **WHEN** the string is `retail-classic-tbc-classic-wotlk-cata`
- **THEN** the guessed game track is classic-cata

#### Scenario: Cata inside a longer word is not cata

- **WHEN** the string is `Catalyst-1.0.zip`
- **THEN** the guessed game track is not classic-cata

#### Scenario: Standard is retail

- **WHEN** the string is `Addon-1.2.3-standard.zip`
- **THEN** the guessed game track is retail

#### Scenario: Mists in an asset file name

- **WHEN** the string is `Addon-1.2.3-mists.zip`
- **THEN** the guessed game track is classic-mists

#### Scenario: Mists wins over cata

- **WHEN** the string is `Addon-cata-mists.zip`
- **THEN** the guessed game track is classic-mists

#### Scenario: Catalogue builder aliases

- **WHEN** the string is exactly `mop`, `cataclysm` or `wotlkc`
- **THEN** the guessed game track is classic-mists, classic-cata or classic-wotlk respectively

#### Scenario: Forever and Camelot

- **WHEN** the string is `Addon-1.2.3-forever.zip` or `Addon_Camelot.zip`
- **THEN** the guessed game track is forever

#### Scenario: Forever inside a longer word is not forever

- **WHEN** the string is `foreverything.zip` or `camelots.zip`
- **THEN** the guessed game track is not forever

#### Scenario: Unknown game track

- **WHEN** the string is `Addon-1.2.3-ptr.zip`
- **THEN** there is no guessed game track

### Requirement: Unclassifiable assets are excluded

An update with no game track after every pass, including the known game tracks pass, SHALL be left out of the result, and the exclusion SHALL be logged at DEBUG level. No game track SHALL be assumed for it.

#### Scenario: Single unhinted zip

- **WHEN** a release published after WoW Classic has only `Addon-1.2.3.zip`, with no game track in the release name and no `release.json`, and the addon has no known game tracks
- **THEN** the release produces no updates

#### Scenario: Unknown game track beside known ones

- **WHEN** a release has `Addon-classic.zip`, `Addon-retail.zip` and `Addon-ptr.zip`, and no `release.json`
- **THEN** only the classic and retail updates are returned

### Requirement: Second pass infers a lone unclassified asset from its siblings

When exactly one update in a release is unclassified and exactly one supported game track is covered by no sibling, the unclassified update SHALL be given that game track. Forever SHALL NOT be counted among the uncovered game tracks: an unlabelled asset far more likely belongs to an established game track. In every other case the release's updates SHALL stay as they are. In particular, retail SHALL NOT be assumed because retail is one of several game tracks left.

#### Scenario: One game track left

- **WHEN** a release's siblings cover every supported game track except classic-tbc, and one update is unclassified
- **THEN** that update is classic-tbc

#### Scenario: Forever is not inferred

- **WHEN** a release's siblings cover every supported game track except forever, and one update is unclassified
- **THEN** the unclassified update stays unclassified after this pass

#### Scenario: Several game tracks left

- **WHEN** a release has a classic update and one unclassified update
- **THEN** the unclassified update stays unclassified after this pass

#### Scenario: Two unclassified

- **WHEN** a release has two unclassified updates
- **THEN** both stay unclassified after this pass
