# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

All development tasks are managed through the `./manage.sh` script:

- `./manage.sh build` - Build the project (runs clean first)
- `./manage.sh test` - Run all tests across modules
- `./manage.sh coverage` - Run tests and generate coverage report

## Architecture

### Multi-Module Structure

The project uses Go workspaces with three main modules:

1. **strongbox/** - Core addon management logic
   - Addon parsing and management
   - Provider integrations (Github, Gitlab, WowInterface)
   - Settings and configuration
   - TOC file parsing

2. **bw/** - "Boardwalk" framework for generic data browsing
   - Core application framework with provider/service architecture
   - UI abstraction layer (CLI and GUI support)
   - State management and event system
   - HTTP utilities and form validation

3. **atk/** - Go wrapper for Tcl/TK (external dependency)
   - Cloned automatically during `./manage.sh clean`
   - Fork of visualfc/atk maintained by project author

### Key Components

- **Provider System**: Modular architecture where `strongbox` is a provider to the `bw` framework
- **Namespace System**: Three-part classification `major/minor/type` (e.g., `strongbox/addons-dir/dir`)
- **State Management**: Central state with listeners for UI updates
- **Result System**: Hierarchical data structure for UI display with parent-child relationships

### Main Entry Point

The application starts in `strongbox/main.go`:
- Handles CLI flags (verbosity levels)
- Initializes both CLI and GUI interfaces
- Sets up XDG directory paths for config and data
- Registers providers and starts services

## Testing

- Tests use standard Go testing framework
- GUI tests require X11 display (uses Xvfb in CI)
- Test fixtures generated with 7z for zip file handling

## Linting

Uses `revive` linter with configuration in `revive.toml`:
- Line length limit: 200 characters
- Enforces lowercase filenames with underscores
- Standard Go conventions with some customizations

## Build System

- Go 1.24.4+ required
- CGO enabled for Tcl/TK integration
- Cross-compilation supports Linux and Windows (arm64/amd64)
- Release builds include UPX compression for amd64 binaries

## Dependencies

Key external dependencies:
- `github.com/visualfc/atk` - Tcl/TK bindings (forked version)
- `github.com/deckarep/golang-set/v2` - Set data structures
- `github.com/lmittmann/tint` - Structured logging
- `github.com/Oudwins/zog` - Validation (strongbox module)

## Development Philosophy

**Strong emphasis on controlling state**: application behaviour should always be deterministic to aid testing.

**Test-Driven Development**: This project is test-driven. Replicate problem with a test first then develop a fix. Continuously ratchet up test coverage and look for opportunities to improve unit tests.

**Code Organization**:
- Topological ordering: Code with fewer dependencies rise to the top of files
- Un-exported code uses underscores (easier to read): `private_function()`
- Exported code uses standard Go capitalization: `PublicFunction()`
- Pure functions preferred where appropriate (easier to test and reason about)

**Project Goals**: See `TODO.md` for current release goals and development priorities.

## Related Projects

- **Strongbox 7.x**: Original Clojure/JavaFX implementation at `/home/torkus/dev/clojure/strongbox` (maintenance mode, patches only)
- **Boardwalk Framework**: The `bw/` module is designed as a general-purpose data browsing framework that Strongbox drives forward

## Development Notes

- XDG directory structure for Linux compatibility
- Supports both CLI and GUI modes (GUI is primary interface)
- Provider pattern allows for extensible data sources
- UI framework abstracts between CLI table output and Tcl/TK GUI
- Project has seen significant churn while building proper separation of concerns
- ignore TODO.md