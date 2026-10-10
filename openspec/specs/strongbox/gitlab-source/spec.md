# strongbox/gitlab-source Specification

## Purpose

Finds addons hosted on gitlab.com by URL and turns their releases into updates, so GitLab-hosted addons install and update like GitHub ones.

## Requirements

### Requirement: GitLab URLs

A GitLab URL SHALL identify a project by its path of two or three segments (group, optional subgroup, project). The scheme, a `www.` prefix, user info, a query, an anchor, a trailing slash and a trailing `/-/releases` SHALL be tolerated. A URL with fewer than two path segments SHALL be rejected. The project path SHALL be lowercased and URL-encoded as the project ID in API requests to `https://gitlab.com/api/v4/projects/<id>`. GitLab paths are case-insensitive, so this gives one cache entry per project.

#### Scenario: Release page URL

- **WHEN** the URL is `https://gitlab.com/woblight/nitro/-/releases`
- **THEN** the project is `woblight/nitro` and the API URL is `https://gitlab.com/api/v4/projects/woblight%2Fnitro`

#### Scenario: Subgroup

- **WHEN** the URL is `gitlab.com/group/subgroup/project`
- **THEN** the project is `group/subgroup/project`

#### Scenario: Too short

- **WHEN** the URL is `https://gitlab.com/woblight`
- **THEN** it is rejected

### Requirement: GitLab releases become updates

The project's releases SHALL be read newest first. A release marked as upcoming SHALL be excluded. Each release link of type `package` or `other` that is not marked external SHALL become an update, downloading from its direct asset URL when present, otherwise its URL, with the release's tag name as its version. Game tracks SHALL be classified as for GitHub: from the link name, then the release name, then elimination among siblings, then the newest release's `release.json`, then, for a release with a single link, the addon's known game tracks. An update without a game track SHALL be excluded.

#### Scenario: Upcoming release excluded

- **WHEN** the only release is an upcoming release
- **THEN** there are no updates

#### Scenario: Classic link

- **WHEN** a release has links `Nitro-1.2.zip` and `Nitro-1.2-classic-bcc.zip`
- **THEN** the second link is a classic-tbc update

#### Scenario: Single all-in-one link

- **WHEN** the only release has a single link `Nitro` and the addon is known to support classic, classic-tbc and retail
- **THEN** that link is one update supporting classic, classic-tbc and retail

#### Scenario: Host error

- **WHEN** the GitLab API responds with HTTP 504
- **THEN** there are no updates and a WARN reports the failure

### Requirement: Finding a GitLab addon

Finding a GitLab addon by URL SHALL produce a catalogue entry with the project path as its source ID, the project name as its label, the project description, the project's URL, zero downloads, and game tracks read from the repository's `.toc` file names, or failing that from the interface versions in its root `.toc` file. A project with no releases, or whose game tracks cannot be determined, SHALL NOT be found. Its newest release SHALL be classified using the game tracks found in its `.toc` files as its known game tracks.

#### Scenario: Several .toc files

- **WHEN** a project's root holds `Nitro.toc`, `Nitro-Classic.toc` and `Nitro-BCC.toc`, the first declaring a retail interface version
- **THEN** the found addon's game tracks are retail, classic and classic-tbc
