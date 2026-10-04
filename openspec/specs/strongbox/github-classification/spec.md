# strongbox/github-classification Specification

## Purpose

Turns the list of releases a Github repository publishes into updates, each labelled with the game tracks it supports, so the right update can be offered to an addons dir. An asset that cannot be classified is left out rather than guessed.

## Requirements

### Requirement: Only published releases produce updates

Drafts and prereleases SHALL produce no updates. Every other release SHALL be considered.

#### Scenario: Draft and prerelease skipped

- **WHEN** the release list contains a draft, a prerelease and a published release
- **THEN** only the published release's assets become updates

### Requirement: Only installable assets produce updates

An asset SHALL become a candidate update only if its content type is `application/zip` or `application/x-zip-compressed` and its state is `uploaded`. A `release.json` asset SHALL never become an update.

#### Scenario: Non-zip and incomplete assets skipped

- **WHEN** a release has a fully uploaded zip, a zip still uploading, a non-zip file and a `release.json`
- **THEN** only the fully uploaded zip becomes a candidate update

### Requirement: Update fields come from the release

Each update SHALL take the release's publication date. Its version SHALL be the release name, or the tag name when the release name is empty, or the asset name when both are empty.

#### Scenario: Version falls back to the tag

- **WHEN** a release has an empty name and the tag `v1.2.3`
- **THEN** each of its updates has the version `v1.2.3`

### Requirement: Updates keep the release order

Updates SHALL be returned in the order of the release list, which Github gives newest first. Updates from the same release SHALL keep the order of its assets.

#### Scenario: Newest release first

- **WHEN** the release list is `1.2.4`, then `1.2.3`
- **THEN** every `1.2.4` update comes before every `1.2.3` update

### Requirement: Game tracks are guessed from names

A game track SHALL be guessed from a string by checking, in order:

1. an exact match against a known alias: `retail`, `mainline`, `classic`, `vanilla`, `classic-tbc`, `tbc`, `bcc`, `classic-wotlk`, `wrath`, `wotlk`, `classic-cata`, `cata`
2. `cata` as a delimited word, giving classic-cata
3. `wrath` or `wotlk`, giving classic-wotlk
4. a `tbc`/`bc`/`bcc` form, giving classic-tbc
5. `classic` or `vanilla`, giving classic
6. `retail` or `mainline`, or `standard` as a delimited word, giving retail

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

#### Scenario: Unknown game track

- **WHEN** the string is `Addon-1.2.3-mists.zip`
- **THEN** there is no guessed game track

### Requirement: First pass classifies each asset on its own

Each candidate update SHALL be given, in order of preference:

1. the game track guessed from its asset name
2. retail, when the release was published before WoW Classic was released (2019-08-26T00:00:00Z)
3. the game track guessed from the release name

An update matching none of these SHALL stay unclassified.

#### Scenario: Asset name beats release name

- **WHEN** the release `1.2.3-classic` has the asset `Addon-1.2.3-wrath.zip`
- **THEN** the update is classic-wotlk

#### Scenario: Published before Classic

- **WHEN** an asset name carries no game track and its release was published in 2018
- **THEN** the update is retail

### Requirement: Second pass infers a lone unclassified asset from its siblings

When exactly one update in a release is unclassified and exactly one supported game track is covered by no sibling, the unclassified update SHALL be given that game track. In every other case the release's updates SHALL stay as they are. In particular, retail SHALL NOT be assumed because retail is one of several game tracks left.

#### Scenario: One game track left

- **WHEN** a release's siblings cover every supported game track except classic-tbc, and one update is unclassified
- **THEN** that update is classic-tbc

#### Scenario: Several game tracks left

- **WHEN** a release has a classic update and one unclassified update
- **THEN** the unclassified update stays unclassified after this pass

#### Scenario: Two unclassified

- **WHEN** a release has two unclassified updates
- **THEN** both stay unclassified after this pass

### Requirement: The newest release's release.json is authoritative

The `release.json` SHALL be downloaded for the newest release that is neither a draft nor a prerelease, if that release has one. It SHALL NOT be downloaded for any other release, so classifying a release list costs at most one extra request. Its position in the Github response SHALL NOT matter.

For each update in that release with a matching `release.json` entry (by file name), the game tracks of the entry's recognised flavours SHALL replace the update's game tracks. An unrecognised flavour SHALL contribute nothing. An entry with no recognised flavour, an update with no matching entry, a failed download or a `release.json` that cannot be parsed SHALL leave the update's game tracks as the earlier passes set them. A `release.json` that cannot be parsed SHALL NOT stop classification of the release list.

#### Scenario: Draft at the head of the list

- **WHEN** the release list is a draft, then a published release with a `release.json` declaring the flavour `wrath` for its zip
- **THEN** the `release.json` is downloaded once
- **AND** the zip's update is classic-wotlk

#### Scenario: Older releases are not consulted

- **WHEN** the two newest published releases both have a `release.json`
- **THEN** only the newest one's `release.json` is downloaded

#### Scenario: Newest release has no release.json

- **WHEN** the newest published release has no `release.json` and an older one does
- **THEN** no `release.json` is downloaded

#### Scenario: Unrecognised flavour keeps the earlier guess

- **WHEN** an asset guessed as classic from its name has a `release.json` entry whose only flavour is unrecognised
- **THEN** the update stays classic

#### Scenario: Partly recognised flavours

- **WHEN** a `release.json` entry declares `mainline` and an unrecognised flavour
- **THEN** the update is retail only

#### Scenario: Download fails

- **WHEN** the `release.json` download fails
- **THEN** the release's updates keep the game tracks from the earlier passes

#### Scenario: Malformed release.json

- **WHEN** the downloaded `release.json` is not valid JSON
- **THEN** the release's updates keep the game tracks from the earlier passes
- **AND** the updates of every other release are still returned

### Requirement: Unclassifiable assets are excluded

An update with no game track after every pass SHALL be left out of the result, and the exclusion SHALL be logged at DEBUG level. No game track SHALL be assumed for it.

#### Scenario: Single unhinted zip

- **WHEN** a release published after WoW Classic has only `Addon-1.2.3.zip`, with no game track in the release name and no `release.json`
- **THEN** the release produces no updates

#### Scenario: Unknown game track beside known ones

- **WHEN** a release has `Addon-classic.zip` and `Addon-mists.zip`, and no `release.json`
- **THEN** only the classic update is returned
