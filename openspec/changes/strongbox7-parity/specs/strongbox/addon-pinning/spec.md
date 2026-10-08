## Purpose

Lets the user hold an addon at a particular version so updates leave it alone until they choose otherwise.

## ADDED Requirements

### Requirement: Pin and unpin

The user SHALL be able to pin one or several selected addons that strongbox installed (they have nfo data), have an installed version and are not ignored. An addon without nfo data SHALL NOT be pinnable, because the pin is kept in nfo data. Pinning SHALL record the installed version as the pinned version in the nfo data of every directory of the addon. The user SHALL be able to unpin pinned, non-ignored addons, removing the pinned version.

#### Scenario: Pin

- **WHEN** the user pins EveryAddon at installed version 1.2.3
- **THEN** each of its directories' nfo data has pinned version 1.2.3
- **AND** EveryAddon is shown as pinned to 1.2.3

#### Scenario: Unpin

- **WHEN** the user unpins a pinned addon
- **THEN** its nfo data has no pinned version

### Requirement: Pinned addons are not updated

A pinned addon whose installed version equals its pinned version SHALL NOT be updateable. A pinned addon whose installed version differs from its pinned version SHALL be updateable only to its pinned version.

#### Scenario: Newer release ignored

- **WHEN** EveryAddon is pinned at 1.2.3, installed at 1.2.3, and 1.2.4 is available
- **THEN** EveryAddon is not updateable

#### Scenario: Restore the pinned version

- **WHEN** EveryAddon is pinned at 1.2.3, installed at 1.2.4, and the host still offers 1.2.3
- **THEN** EveryAddon is updateable to 1.2.3

### Requirement: Installing over a pinned addon removes the pin

An install from a zip file over a pinned addon SHALL remove the pin. An install from the catalogue or a URL over a pinned addon SHALL be refused.

#### Scenario: Zip over pinned

- **WHEN** the user installs a zip over a pinned addon
- **THEN** the installed addon is not pinned
