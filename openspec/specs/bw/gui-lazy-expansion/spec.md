# bw/gui-lazy-expansion Specification

## Purpose

Defines how the GUI presents and expands rows whose children are loaded on demand, so a user can browse arbitrarily deep hierarchies without the application loading them upfront.

## Requirements

### Requirement: Unrealised lazy rows are expandable

A row whose result has unrealised lazy children SHALL present the same expansion affordance as a row with loaded children (expand indicator, double-click and expand gestures work). A row whose result declares the do-not-load policy or has no children SHALL NOT present an expansion affordance.

#### Scenario: Lazy row displayed before first expansion

- **WHEN** a result with unrealised lazy children is displayed in the tree
- **THEN** the row can be expanded by the user

#### Scenario: Leaf row displayed

- **WHEN** a result whose item declares the do-not-load policy is displayed
- **THEN** the row presents no expansion affordance

### Requirement: Expanding a lazy row realises its children

When the user expands a row with unrealised lazy children, the GUI SHALL request realisation of that result, and the loaded children SHALL appear as child rows of that row. Any placeholder used to make the row expandable SHALL be removed once real children are shown. The realisation work SHALL NOT block the UI thread.

#### Scenario: First expansion of a lazy row

- **WHEN** the user expands a lazy row whose children are unrealised
- **THEN** the children are loaded and appear under that row
- **AND** the UI remains responsive while loading is in progress

#### Scenario: Expansion exceeds the loading timeout

- **WHEN** the user expands a lazy row and loading its children exceeds the timeout
- **THEN** the placeholder is replaced by a single non-expandable 'failed' row
- **AND** the UI remains responsive throughout

#### Scenario: Expansion yields no children

- **WHEN** the user expands a lazy row and realisation produces zero children
- **THEN** the row shows no child rows and no placeholder remains

### Requirement: Collapse and re-expansion do not reload

Collapsing a previously expanded lazy row and expanding it again SHALL redisplay the already-loaded children without triggering realisation again.

#### Scenario: Collapse then re-expand

- **WHEN** the user collapses an expanded lazy row and expands it again
- **THEN** the same child rows are shown
- **AND** no new load is performed
