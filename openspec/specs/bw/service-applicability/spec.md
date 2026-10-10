# bw/service-applicability Specification

## Purpose

Lets a provider declare which items each of its services accepts and when it applies, so boardwalk builds context menus and menu states from data instead of hand-written mappings.

## Requirements

### Requirement: Services declare accepted item types

A service SHALL be able to declare the item types it accepts, and whether it accepts one item or many. Boardwalk SHALL derive the services offered for a selection from these declarations. A service that declares no item types SHALL NOT appear in any context menu.

#### Scenario: Single-item service

- **WHEN** a service accepts exactly one item of type T and the user selects two rows of type T
- **THEN** the service is not offered for that selection

#### Scenario: Many-item service

- **WHEN** a service accepts one or many items of type T and the user selects three rows of type T
- **THEN** the service is offered and called with all three items

### Requirement: Services declare when they apply

A service SHALL be able to declare an applicability predicate over the selected items. When the predicate is false for a selection, the service's context menu entry SHALL be shown disabled. A service without a predicate SHALL always apply to items it accepts. A menu item bound to a service SHALL be disabled when that service's predicate, given no items, is false.

#### Scenario: Disabled entry

- **WHEN** a service's predicate is false for the selected row
- **THEN** the service is listed in the context menu but cannot be chosen

### Requirement: Services without an implementation are not offered

A service without a callable SHALL NOT be offered in menus or context menus.

#### Scenario: Placeholder hidden

- **WHEN** a provider declares a service with no callable
- **THEN** no menu or context menu lists it

### Requirement: Calling a service from a selection

Choosing a context menu entry SHALL call the service with the selected items as its first argument. When the service needs further input, a form SHALL be opened with the selected items already filled in. When it needs none, it SHALL be called immediately, off the GUI thread.

#### Scenario: No further input

- **WHEN** a service takes only the selected items and the user chooses it
- **THEN** it runs without a form being opened
