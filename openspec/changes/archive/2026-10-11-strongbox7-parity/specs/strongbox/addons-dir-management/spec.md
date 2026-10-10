## Purpose

Lets the user keep a list of WoW addons directories, choose which one strongbox works on, and set the game track and strictness each one installs for.

## ADDED Requirements

### Requirement: Supported game tracks

The supported game tracks SHALL be retail, classic, classic-tbc, classic-wotlk, classic-cata, classic-mists and forever, labelled "Retail", "Classic", "Classic (TBC)", "Classic (WotLK)", "Classic (Cata)", "Classic (Mists)" and "Forever".

#### Scenario: Labels

- **WHEN** the game track choices are listed
- **THEN** they are the seven supported game tracks with the labels above

### Requirement: Adding an addons dir

The user SHALL be able to add an existing directory as an addons dir. A new addons dir SHALL be strict. Its game track SHALL be guessed from the WoW client directories in its path, such as `_retail_` or `_classic_era_`, innermost first, using the same name rules as game track guessing for release names. Other directory names SHALL NOT be considered, so `/home/abc` does not suggest classic-tbc. A path with no client directory naming a game track SHALL be retail. Adding a path that is already an addons dir SHALL add nothing and SHALL select that addons dir. A newly added addons dir SHALL become the selected addons dir, and the settings SHALL be saved.

#### Scenario: Retail by default

- **WHEN** the user adds `/games/wow/_retail_/Interface/AddOns`
- **THEN** a strict retail addons dir is added and selected

#### Scenario: Classic guessed from path

- **WHEN** the user adds `/games/wow/_classic_era_/Interface/AddOns`
- **THEN** the new addons dir's game track is classic

#### Scenario: Ordinary directory names are not guessed from

- **WHEN** the user adds `/home/abc/wow-addons`
- **THEN** the new addons dir's game track is retail

#### Scenario: Duplicate path

- **WHEN** the user adds a path that is already an addons dir
- **THEN** the addons dir list is unchanged and that addons dir is selected

#### Scenario: Not a directory

- **WHEN** the user tries to add a path that does not exist or is a file
- **THEN** the addons dir is not added and the form reports the path is not a directory

### Requirement: Selecting an addons dir

The user SHALL be able to select any addons dir in the list. Selecting an addons dir SHALL load its installed addons, match them against the catalogue and check them for updates, and SHALL save the settings. Exactly one addons dir SHALL be selected whenever at least one available addons dir exists.

#### Scenario: Switch addons dir

- **WHEN** the user selects a second addons dir
- **THEN** the second addons dir is marked selected and the first is not
- **AND** the second addons dir's addons are shown and checked for updates

### Requirement: Removing an addons dir

The user SHALL be able to remove an addons dir from the list after confirming. Removing an addons dir SHALL NOT delete or modify any file on disk. When the selected addons dir is removed, the first remaining available addons dir SHALL be selected, or none when the list is empty. The removed addons dir and its addons SHALL disappear from the GUI, and the settings SHALL be saved.

#### Scenario: Remove selected addons dir

- **WHEN** the user removes the selected addons dir of two
- **THEN** the other addons dir becomes selected
- **AND** every file in the removed directory is unchanged

#### Scenario: Remove last addons dir

- **WHEN** the user removes the only addons dir
- **THEN** no addons dir is selected and no installed addons are shown

### Requirement: Changing game track and strictness

The user SHALL be able to set an addons dir's game track to any supported game track, and its strictness on or off. Either change SHALL be saved and SHALL cause the addons dir's addons to be re-evaluated: the `.toc` file and update chosen for each addon SHALL follow the new game track and strictness.

#### Scenario: Strict to relaxed

- **WHEN** a strict retail addons dir holds an addon whose only update is for classic, and the user turns strictness off
- **THEN** the addon is offered the classic update

#### Scenario: Change game track

- **WHEN** the user changes an addons dir from retail to classic
- **THEN** addons with a classic `.toc` file show that `.toc` file's details

### Requirement: Unavailable addons dirs

An addons dir whose directory does not exist SHALL be shown as unavailable, SHALL NOT be selected automatically, and SHALL NOT be offered for installing into. Its entry SHALL be kept in the settings. When its directory exists again at a later start, it SHALL be available.

#### Scenario: Unmounted drive

- **WHEN** an addons dir on an unmounted drive is listed at startup
- **THEN** it is shown as unavailable and another available addons dir is selected
- **AND** its entry remains in the settings file
