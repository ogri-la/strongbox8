# bw/http-cache Specification

## Purpose

Makes boardwalk's HTTP layer cache responses for a bounded time, store them where the application keeps its data, download files atomically, and be fully replaceable in tests.

## Requirements

### Requirement: Cache expiry

A cached HTTP response SHALL be used only when it is younger than its expiry, one hour by default. An older entry SHALL be fetched again. Error responses and non-2xx responses SHALL NOT be cached. Cache entries older than their expiry SHALL be deleted when the application starts.

#### Scenario: Fresh entry

- **WHEN** a URL was fetched 10 minutes ago and is requested again
- **THEN** no network request is made

#### Scenario: Expired entry

- **WHEN** a URL was fetched 2 hours ago and is requested again
- **THEN** a network request is made and the cache entry is replaced

### Requirement: Cache location

The HTTP cache SHALL be stored in a `cache` directory inside the application's data directory, not in a shared system directory.

#### Scenario: Location

- **WHEN** the application's data directory is `/tmp/data/app`
- **THEN** cache entries are written under `/tmp/data/app/cache`

### Requirement: Atomic file downloads

Downloading a file SHALL write to a temporary file in the destination directory and rename it into place only after a successful, complete response. A non-2xx response or an interrupted transfer SHALL leave any existing destination file unchanged, SHALL NOT create the destination file, and SHALL remove the temporary file.

#### Scenario: Not found

- **WHEN** a download responds with HTTP 404
- **THEN** no destination file exists afterwards

#### Scenario: Existing file preserved

- **WHEN** a download of an existing file fails part way
- **THEN** the existing file is unchanged

### Requirement: Requests identify the application

HTTP requests SHALL send a User-Agent naming the application, its version and its project URL, and SHALL time out when a host does not respond within a bounded time.

#### Scenario: User agent

- **WHEN** any request is made
- **THEN** it carries the application's User-Agent

### Requirement: Isolated fake transport for tests

Tests SHALL be able to replace all HTTP access, including file downloads, with fixtures keyed by URL. In isolation mode, a request for a URL with no fixture SHALL fail the request and be recorded so the test can fail.

#### Scenario: Unrouted request

- **WHEN** a test in isolation mode triggers a request for a URL with no fixture
- **THEN** the request fails and the test can see which URL was requested
