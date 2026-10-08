## Purpose

Lets providers run long work in the background while boardwalk shows the user what is happening, which rows are affected, and when it is done.

## ADDED Requirements

### Requirement: Tracked jobs

A provider SHALL be able to start a named background job, optionally with a known number of steps, report progress, and finish it. Boardwalk SHALL track running jobs in application state.

#### Scenario: Job lifecycle

- **WHEN** a job named "checking for updates" with 10 steps reports 4 steps done
- **THEN** application state shows that job running at 4 of 10

### Requirement: Status bar

The GUI SHALL show a status bar. While jobs run, it SHALL show each running job's name and progress. When none run, it SHALL show that the application is idle.

#### Scenario: Idle

- **WHEN** no jobs are running
- **THEN** the status bar shows the application is idle

#### Scenario: Running

- **WHEN** a job "checking for updates" is at 4 of 10
- **THEN** the status bar shows "checking for updates" with 4 of 10

### Requirement: Busy rows

A provider SHALL be able to mark a result busy and clear the mark. A busy result's row SHALL be visibly distinct and SHALL return to normal when the mark is cleared.

#### Scenario: Busy then done

- **WHEN** a result is marked busy and then cleared
- **THEN** its row is shown busy and then normal

### Requirement: Waiting for idle

Boardwalk SHALL provide a way to wait until every running service, background job and pending state update has finished, so tests can make assertions deterministically.

#### Scenario: Test waits

- **WHEN** a test starts a service that starts a background job and then waits for idle
- **THEN** the wait returns only after the job has finished and its state updates are applied
