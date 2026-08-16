# bw/lazy-child-realisation Specification

## Purpose

Defines the framework contract for results whose children are loaded on demand: a provider can declare an item's children as lazy and rely on the framework to defer loading until explicitly requested, and to load them exactly once.

## Requirements

### Requirement: Lazy items are not realised on insert

When a result whose item declares the lazy child-loading policy is added to application state, the framework SHALL NOT load its children. The result SHALL be recorded as having unrealised children. This MUST hold both for results added at the top level and for results that appear as children of another result.

#### Scenario: Top-level lazy result added to state

- **WHEN** a result whose item declares lazy children is added to state at the top level
- **THEN** no children are loaded for it
- **AND** the result is marked as having unrealised children

#### Scenario: Nested lazy result realised as part of a parent

- **WHEN** a parent's children are loaded and one of those children itself declares lazy children
- **THEN** the child is added to state without its own children being loaded

### Requirement: Realisation on demand loads exactly one level

When realisation is requested for a result with unrealised children, the framework SHALL load that item's immediate children, add them to state with their parent reference set to the requesting result, mark the result as realised, and return the children. Grandchildren SHALL NOT be loaded unless their own policy is eager.

#### Scenario: First realisation of a lazy result

- **WHEN** realisation is requested for a lazy result whose children are unrealised
- **THEN** its immediate children are loaded and added to state as children of that result
- **AND** the result is marked as realised

#### Scenario: Lazy grandchildren stay unrealised

- **WHEN** realisation of a lazy result produces children that are themselves lazy
- **THEN** those children are added unrealised and no grandchildren are loaded

### Requirement: Realisation happens at most once

Requesting realisation for a result already marked as realised SHALL NOT reload its children. The framework SHALL return the children already present in state.

#### Scenario: Repeated realisation request

- **WHEN** realisation is requested twice for the same result
- **THEN** the item's child-loading logic runs only on the first request
- **AND** both requests return the same set of children

### Requirement: Realisation is bounded by a timeout

On-demand realisation SHALL be subject to a timeout (a few seconds). If loading an item's children exceeds the timeout, the load SHALL be abandoned: the result is given exactly one child — a terminal 'failed' result that cannot be expanded — the result is marked realised, and a warning is logged. Results produced by the abandoned load after the timeout SHALL be discarded and MUST NOT enter application state. Retrying a timed-out result is not supported; a fresh browse of the same path is the way to try again.

#### Scenario: Realisation exceeds the timeout

- **WHEN** realisation of a lazy result takes longer than the timeout
- **THEN** the result gains a single non-expandable 'failed' child
- **AND** the result is marked realised
- **AND** a warning is logged

#### Scenario: Late results are discarded

- **WHEN** an abandoned load completes after the timeout has fired
- **THEN** none of its results are added to application state

### Requirement: Eager and opt-out policies are unchanged

Items declaring the eager child-loading policy SHALL continue to have their children loaded when the result is added to state, and items declaring the do-not-load policy SHALL never have children loaded by the framework. Existing providers relying on eager loading MUST be unaffected by this change.

#### Scenario: Eager item added to state

- **WHEN** a result whose item declares eager children is added to state
- **THEN** its children (and their eager descendants) are loaded immediately, as before

#### Scenario: Do-not-load item added to state

- **WHEN** a result whose item declares the do-not-load policy is added to state
- **THEN** the framework never loads children for it
