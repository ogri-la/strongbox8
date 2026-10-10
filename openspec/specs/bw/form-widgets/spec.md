# bw/form-widgets Specification

## Purpose

Gives service forms the input widgets providers need beyond plain text: choosing from a list, choosing files or directories, and yes/no options.

## Requirements

### Requirement: Choice list

An argument with choices SHALL be rendered as a list the user picks from, showing each choice's label. An exclusive choice SHALL allow one selection and a non-exclusive choice several. The default SHALL be preselected. A submitted value that is not one of the choices SHALL fail validation.

#### Scenario: Pick one

- **WHEN** a form has an exclusive choice of three options and the user picks the second
- **THEN** the service receives the second option

#### Scenario: Value outside choices

- **WHEN** a submitted value is not among the choices
- **THEN** the form reports a validation error and the service is not called

### Requirement: File and directory pickers

An argument with a file-picker widget SHALL let the user choose one or more files using the desktop's file dialog, and an argument with a directory-picker widget SHALL let the user choose a directory. A picker MAY restrict files by extension and MAY start in a given directory. The chosen paths SHALL be shown in the form before submitting.

#### Scenario: Choose zip files

- **WHEN** a form has a file picker restricted to `.zip` and the user chooses two files
- **THEN** the service receives both paths

### Requirement: Checkbox

A boolean argument SHALL be rendered as a checkbox showing its current value.

#### Scenario: Toggle

- **WHEN** a boolean argument defaults to true and the user unticks it
- **THEN** the service receives false

### Requirement: Unsupported widgets fail loudly in development

A form argument with an unsupported widget SHALL be reported at ERROR level naming the service and argument, and the form SHALL still render its other arguments.

#### Scenario: Unknown widget

- **WHEN** an argument declares an unknown widget
- **THEN** an ERROR names the service and argument
