## Purpose

Lets the user browse the local filesystem as a tree through the UI, listing one directory level at a time as directories are expanded rather than scanning trees upfront.

## ADDED Requirements

### Requirement: Browse service creates a lazy directory root

The filesystem service group SHALL offer a `browse` service that accepts a directory path. Invoking it with a valid, readable directory SHALL add a single result representing that directory, with its contents unloaded until the user expands it. A path that does not exist or is not a directory SHALL be rejected by input validation before the service runs.

#### Scenario: Browsing a valid directory

- **WHEN** the user invokes `browse` with an existing directory
- **THEN** one result representing that directory is added to state
- **AND** none of the directory's contents are read yet

#### Scenario: Browsing an invalid path

- **WHEN** the user invokes `browse` with a path that does not exist or is a file
- **THEN** validation rejects the input and no result is added

### Requirement: Directory listing is lazy, one level at a time

Expanding a directory SHALL list only that directory's immediate entries. Each subdirectory entry SHALL itself be lazy; each file entry SHALL be a leaf. Hidden entries (dot-files) SHALL be included.

#### Scenario: Expanding a directory

- **WHEN** a directory result is expanded
- **THEN** its immediate files and subdirectories appear as children
- **AND** no subdirectory contents are read

#### Scenario: File entry

- **WHEN** a file entry is displayed
- **THEN** it has no expansion affordance and no children

### Requirement: Entries are displayed with name and ordered deterministically

Each entry SHALL display at least its name. Entries of a directory SHALL be ordered deterministically: directories before files, each group sorted by name. Repeated listings of the same directory SHALL produce the same order.

#### Scenario: Mixed directory contents

- **WHEN** a directory containing files and subdirectories is expanded
- **THEN** subdirectories are listed first, sorted by name, followed by files sorted by name

### Requirement: Results are identified by absolute path

Every filesystem result SHALL use the entry's absolute path as its stable identifier, so the same path always maps to the same result and repeat operations do not create duplicates.

#### Scenario: Browsing the same directory twice

- **WHEN** the user invokes `browse` twice with the same directory
- **THEN** state contains one result for that directory, not two

### Requirement: Browsing is reachable from the application

The application SHALL offer a menu entry that invokes the `browse` service, and SHALL show filesystem results (including load failures) in a dedicated files view. On start the files view SHALL contain the user's home directory as a single unrealised browsable root.

#### Scenario: Browsing from the menu

- **WHEN** the user chooses File → "Browse Directory"
- **THEN** the `browse` service form opens and a submitted directory appears in the files view

#### Scenario: Default root on start

- **WHEN** the application starts
- **THEN** the files view shows the user's home directory as a browsable root
- **AND** none of its contents are read until it is expanded

### Requirement: Unreadable directories fail without crashing

If a directory cannot be read when expanded (for example, permission denied or removed since listing), the failure SHALL be reported as a warning through the application log and the row SHALL show no children. The application SHALL NOT crash or leave a stale loading placeholder.

#### Scenario: Expanding an unreadable directory

- **WHEN** the user expands a directory that cannot be read
- **THEN** a warning is logged
- **AND** the row shows no children and the UI remains usable
