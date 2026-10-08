## Purpose

Finds the updates available for installed addons at their hosts and decides which addons can be updated, honouring each addons dir's game track and strictness.

## ADDED Requirements

### Requirement: Supported hosts

Updates SHALL be found at github, gitlab and wowinterface. An addon from curseforge, tukui or any other source SHALL NOT be checked; the user SHALL be told the source is unsupported, once per addon per check, at INFO level.

#### Scenario: Unsupported source

- **WHEN** an installed addon's source is `curseforge`
- **THEN** no request is made for it and it is not marked as having an update

### Requirement: Choosing an update

For each checked addon, strongbox SHALL choose one update from the host's updates, which are ordered newest first. In a strict addons dir it SHALL choose the newest update supporting the addons dir's game track. In a relaxed addons dir it SHALL choose the newest update supporting the first game track in the preference order that any update supports. When the addon is pinned and an update with the pinned version exists, that update SHALL be chosen instead. When no update qualifies, the addon SHALL have no update and the user SHALL be told, at INFO level, which game tracks were searched, for example `no 'Classic (TBC)', 'Classic (WotLK)' or 'Classic' release found on github.`

#### Scenario: Strict and missing

- **WHEN** a strict retail addons dir holds an addon whose host offers only classic updates
- **THEN** the addon has no update

#### Scenario: Relaxed fallback

- **WHEN** a relaxed retail addons dir holds an addon whose host offers only classic updates
- **THEN** the newest classic update is chosen

#### Scenario: Strict with both

- **WHEN** a strict classic addons dir holds an addon whose host offers retail and classic updates
- **THEN** the newest classic update is chosen

### Requirement: Updateable rules

An addon SHALL be updateable when it has a chosen update and none of these hold:

- it is ignored;
- it is pinned and its installed version equals its pinned version, or its pinned version differs from the chosen update's version;
- the chosen update's version equals the installed version, and the installed `.toc` files support a game track the update supports;
- the chosen update's version equals the installed version, and the nfo data's installed game track is one the update supports.

When the versions are equal but neither the `.toc` files nor the installed game track cover the update's game tracks, the addon SHALL be updateable.

#### Scenario: Newer version

- **WHEN** installed version is `1.2.3` and the chosen update is `1.2.4`
- **THEN** the addon is updateable

#### Scenario: Same version

- **WHEN** installed version and chosen update are both `1.2.3`, and the `.toc` supports retail, which the update supports
- **THEN** the addon is not updateable

#### Scenario: Same version, wrong game track

- **WHEN** installed version and chosen update are both `1.2.3`, the installed addon supports only classic, and the update supports only retail
- **THEN** the addon is updateable

#### Scenario: Ignored

- **WHEN** an ignored addon has a newer update
- **THEN** it is not updateable

### Requirement: Checks run in parallel and show progress

Checking all addons SHALL check addons concurrently, with a bounded number of simultaneous requests. Each addon SHALL be marked busy while it is checked. A failure checking one addon SHALL be logged at WARN level for that addon and SHALL NOT stop the others. The user SHALL be able to check one or several selected addons on demand.

#### Scenario: One host fails

- **WHEN** github returns HTTP 500 for one addon and the other addons' hosts respond
- **THEN** the other addons are checked and marked
- **AND** a WARN names the failed addon

#### Scenario: Busy marker

- **WHEN** an addon is being checked
- **THEN** its row is shown as busy until its check completes

### Requirement: Checks see new releases

An update check SHALL NOT be answered from an HTTP response cached more than one hour earlier.

#### Scenario: Release published after the first check

- **WHEN** an addon was checked two hours ago, a new release has been published since, and the user checks again
- **THEN** the new release is offered as the update

### Requirement: GitHub token

When the `GITHUB_TOKEN` environment variable is set, requests to the GitHub API SHALL be authenticated with it. A GitHub rate-limit response SHALL be reported to the user as the request quota being exceeded.

#### Scenario: Rate limited

- **WHEN** the GitHub API responds with HTTP 403 and no requests remaining
- **THEN** the user is told GitHub's request quota has been exceeded
