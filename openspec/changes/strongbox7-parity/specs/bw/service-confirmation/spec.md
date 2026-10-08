## Purpose

Makes boardwalk ask before running services that destroy or remove data, so every provider gets the same safeguard without building its own dialogs.

## ADDED Requirements

### Requirement: Destructive services ask first

A service SHALL be able to declare a confirmation message built from its arguments. Before calling such a service from a menu, context menu or form, boardwalk SHALL show the message with confirm and cancel choices. Cancelling SHALL NOT call the service. Calling the service programmatically, outside the GUI, SHALL NOT ask.

#### Scenario: Confirm

- **WHEN** the user chooses a service declaring a confirmation and confirms
- **THEN** the service runs

#### Scenario: Cancel

- **WHEN** the user chooses a service declaring a confirmation and cancels
- **THEN** the service does not run

### Requirement: Confirmation is answerable in tests

Integration tests SHALL be able to answer a pending confirmation without a person present.

#### Scenario: Test confirms

- **WHEN** a test triggers a service declaring a confirmation and answers confirm
- **THEN** the service runs
