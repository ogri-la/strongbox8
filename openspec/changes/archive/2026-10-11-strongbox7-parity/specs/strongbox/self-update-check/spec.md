## Purpose

Tells the user, when they allow it, that a newer strongbox release exists, without updating strongbox itself.

## ADDED Requirements

### Requirement: Check for a newer strongbox

When the `check-for-update` preference is on, strongbox SHALL request the list of releases from `https://api.github.com/repos/ogri-la/strongbox/releases` once per start, in the background, after installed addons are shown. Pre-releases and drafts SHALL be ignored. When the newest release's version is greater than the running version by semantic version ordering, the user SHALL be told a newer version is available, with a link to the releases page, in the About dialog and by an INFO message. When the preference is off, no request SHALL be made.

#### Scenario: Newer release

- **WHEN** strongbox 8.0.0 is running and the newest release is 8.1.0
- **THEN** the user is told version 8.1.0 is available

#### Scenario: Preference off

- **WHEN** `check-for-update` is false
- **THEN** no request is made to the releases URL

#### Scenario: Throttled

- **WHEN** the releases request responds with HTTP 403
- **THEN** no newer version is reported and a WARN reports the failure, once per start

### Requirement: Version ordering

Release versions SHALL be ordered by semantic version: numeric parts compare numerically, and a pre-release suffix such as `-alpha.3` orders before the same version without one.

#### Scenario: Pre-release orders first

- **WHEN** the versions are `8.0.0`, `8.0.0-alpha.3` and `7.8.0`
- **THEN** they order `7.8.0`, `8.0.0-alpha.3`, `8.0.0`
