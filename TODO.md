# TODO

A structured capture of deferred work, ideas, and nice-to-haves.
Each item is separated by `---` and has key-value metadata followed
by optional free-text context.

---
title: Investigate 'check addon' context menu behaviour
added: 2026-06-07
tags: strongbox, gui, ux
summary: Unclear what 'check addon' does — also why 'update' is offered for addons with no updates, and why the update form is empty

Part of alpha.4 UX audit. Also includes: "how do I update all addons?" —
the service exists (update_all_addons) but discoverability may be lacking.
---

---
title: Fix initial strongbox config creation flow
added: 2026-06-07
effort: high
tags: strongbox, ux
summary: Empty-state onboarding is broken — creating addons dir then installing an addon does nothing

Problems: no directory selection widget on new addons dir, installing
doesn't switch to installed pane, installing with no dir selected fails
silently.
---

---
title: Selecting an addon dir should not expand or collapse others
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Selecting one addons dir currently expands/collapses siblings — each dir's state should be independent
---

---
title: Expand selected addons dir before updating its contents
added: 2026-06-07
effort: low
tags: bw, gui
summary: When an addons dir is selected, expand it first then load/refresh the addons within it
---

---
title: Install from search results with progress feedback
added: 2026-06-07
effort: high
tags: strongbox, gui, ux
summary: Installing addon(s) from search should switch to addons-dir tab, show download progress, and not freeze the UI
---

---
title: Reach 50% test coverage
added: 2026-06-07
effort: high
tags: testing
summary: Continuously ratchet up test coverage across modules — target 50% as minimum baseline
---

---
title: Feature parity checklist with strongbox 7.x
added: 2026-06-07
effort: medium
tags: strongbox, planning
summary: Enumerate all 7.x features and track implementation status — first beta must be feature-complete
---

---
title: Refresh should update results with children
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Refreshing doesn't re-render results that have children — expanded items go stale after refresh
---

---
title: Enter key should submit forms
added: 2026-06-07
effort: low
tags: bw, gui
summary: Pressing Return in a form field doesn't trigger submit — add <Return> binding to form entries
---

---
title: Fix namespace trailing slash in GUI display
added: 2026-06-07
effort: low
tags: bw, gui, namespace
summary: NS.String() emits a trailing slash when Type is empty — the GUI displays this raw
---

---
title: Reconsider namespace separator and depth
added: 2026-06-07
tags: bw, architecture, namespace
summary: Evaluate whether NS should use pipes instead of slashes and whether some namespaces need only major+minor

Current format is "major/minor/type" via NS.String(). Slashes may
confuse with filesystem paths. Some namespaces (e.g. addons-dir) may
not need the type component.
---

---
title: Implement F5 refresh keybinding
added: 2026-06-07
effort: low
tags: bw, gui
summary: F5 doesn't trigger a refresh — bind <F5> to the refresh service
---

---
title: Show count in 'pruning zip files' message
added: 2026-06-07
effort: low
tags: strongbox, logging
summary: The info message when pruning zip files should include the number of files pruned
---

---
title: Don't create logs directory unless writing to filesystem
added: 2026-06-07
effort: low
tags: strongbox, filesystem
summary: The logs directory is created eagerly — only create it when log output is actually directed to a file
---

---
title: Audit log levels
added: 2026-06-07
effort: medium
tags: strongbox, logging
summary: Review all log calls against level semantics — debug: dev only, info: normal usage, warn: eventual attention, error: immediate attention
---

---
title: Context menu should not offer to re-select current addons dir
added: 2026-06-07
effort: low
tags: strongbox, gui, ux
summary: Right-clicking an already-selected addons dir offers to select it again (and reloads it) — suppress or no-op
---

---
title: Batch row collapse after full insert
added: 2026-06-07
effort: low
tags: bw, gui, performance
summary: Collapse rows once after all rows are inserted rather than collapsing per-batch during insertion
---

---
title: Add tooltips to columns and headers
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Add real tooltip widgets for column values (e.g. WoW column) and column headers — current form-field tooltips are just labels
---

---
title: Auto-size columns to fit content
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Columns should size to fit their content — currently no auto-sizing logic exists
---

---
title: Per-tab customisable filterable results
added: 2026-06-07
effort: high
tags: bw, gui
summary: Each tab should support ctrl-f filtering on tab-specific fields (e.g. folder name + game track on addons-dir tab)
---

---
title: Clickable elements in result rows
added: 2026-06-07
effort: high
tags: bw, gui
summary: URLs and tags in result cells should be clickable — URLs open in browser, tags filter to matching results
---

---
title: Auto-pagination for long result lists
added: 2026-06-07
effort: high
tags: bw, gui, performance
summary: Long lists should paginate automatically rather than rendering all rows at once
---

---
title: Task runners with progress bars
added: 2026-06-07
effort: high
tags: bw, gui
summary: Long-running operations (installs, updates) need task runners with visible progress indication
---

---
title: Focus option for search results
added: 2026-06-07
effort: medium
tags: bw, gui
summary: A "focus" action that creates a new view containing only the selected results
---

---
title: Expand-all option for search results
added: 2026-06-07
effort: low
tags: bw, gui
summary: An "expand all" action that expands all non-lazy children in the current result set
---

---
title: Fix details pane collapse behaviour
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Details pane only collapses to its content minimum — once resized it won't collapse further

The pane should fully collapse regardless of content height or
prior resize state.
---

---
title: Exploded views for results
added: 2026-06-07
effort: high
tags: bw, gui, architecture
summary: Allow a result to yield a flat list of related items (directories, URLs, toc files) distinct from its hierarchical children
---

---
title: Rename NFO.Primary to IsPrimary
added: 2026-06-07
effort: low
tags: strongbox, naming
source: strongbox/src/nfo.go:30
summary: Rename the `Primary` field on NFO struct to `IsPrimary` for clarity (also update JSON tag)
---

---
title: Honour xdg_path test-isolation guidance (env vars + cwd)
added: 2026-06-07
updated: 2026-06-14
effort: low
tags: strongbox, docs, testing
source: strongbox/src/core.go:32
summary: The xdg comment says tests must set env vars AND cwd before init for isolation — cwd is not set in test helpers and isolation leaks

The lifecycle prose in the comment (generate during start → check during
init-dirs → fix in state) now reads as complete. What's outstanding is the
last clause: "during testing, ensure the correct environment variables and
cwd are set prior to init for proper isolation." DummyApp2 in
strongbox/src/common_test.go sets XDG_DATA_HOME/XDG_CONFIG_HOME but never
sets cwd, and carries a live "todo: the envvars above are not preventing the
catalogue from loading" — isolation is not actually achieved.
---

---
title: Fix catalogue loading in ItemChildren
added: 2026-06-07
updated: 2026-06-14
effort: medium
tags: strongbox, catalogue
source: strongbox/src/catalogue.go:229
summary: Catalogue.ItemChildren re-loads the whole catalogue from disk to list its children instead of reading them off the already-loaded parent

The inline TODO comment was removed but the problem stands. To expand a
Catalogue result the parent must already be loaded in state, yet
ItemChildren calls _db_load_catalogue(app) again — which guards with
`if db_catalogue_loaded(app) { return error("catalogue already loaded") }`
(catalogue.go:415), so the call errors and yields an empty child list.
The parent (Catalogue) should yield its children (CatalogueAddon entries)
from its own already-unmarshalled data, not re-read the file. Contrast
read_catalogue_file, which simply unmarshals JSON into a Catalogue struct.
---

---
title: Rename InstalledAddon.Name to DirName
added: 2026-06-07
effort: low
tags: strongbox, naming
source: strongbox/src/addon.go:73
summary: The derived `Name` field on InstalledAddon should be renamed to `DirName` to reflect its actual semantics
---

---
title: Shift primary addon selection into MakeAddon
added: 2026-06-07
effort: medium
tags: strongbox, addon
source: strongbox/src/addon.go:295
summary: Primary NFO selection is assumed at Addon construction — move the selection logic into MakeAddon for correctness
---

---
title: Consolidate primary-picking logic into MakeAddon
added: 2026-06-07
effort: medium
tags: strongbox, addon
source: strongbox/src/addon.go:766
summary: Evaluate how much of the primary-selection and group-handling in load_addons can be pushed into MakeAddon
---

---
title: Prevent multiple-primary propagation
added: 2026-06-07
effort: low
tags: strongbox, addon
source: strongbox/src/addon.go:800
summary: When multiple NFO files claim primary, ensure the "last one wins" choice doesn't propagate inconsistently
---

---
title: Delete corrupt NFO data on read failure
added: 2026-06-07
effort: low
tags: strongbox, data-integrity
source: strongbox/src/core.go:571
summary: When .nfo data exists but is unreadable (bad JSON), delete the corrupt file rather than leaving it in place
---

---
title: Revisit expensive column reset on row update
added: 2026-06-07
effort: medium
tags: bw, gui, performance
source: bw/ui/gui.go:1172
summary: set_tablelist_cols is called (currently disabled) on every row update — find a cheaper way to handle column changes
---

---
title: Move tablelist styling logic to parade theme
added: 2026-06-07
effort: medium
tags: bw, gui, theming
source: bw/ui/gui.go:1251
summary: ApplyTablelistStyling belongs in the parade theme module rather than gui.go — consolidate with similar tk eval in gui.Start()
---

---
title: Clarify StateRoot semantics and rename to ResultList
added: 2026-06-07
effort: low
tags: bw, naming
source: bw/core/core.go:197
summary: StateRoot() claims to return a copy but may not — verify copy semantics and rename to app.ResultList()
---

---
title: Document why filter_result_list sorts by ID
added: 2026-06-07
effort: low
tags: bw, clarity
source: bw/core/core.go:509
summary: A sort by Result.ID after filtering has no documented rationale — determine if it's needed and document or remove
---

---
title: Convert ResetState to stop-and-restart
added: 2026-06-07
effort: high
tags: bw, lifecycle
source: bw/core/core.go:763
summary: ResetState currently just replaces state in-place — should throw an error caught by main to trigger proper stop() then start()
---

---
title: Enforce provider uniqueness on registration
added: 2026-06-07
effort: low
tags: bw, providers
source: bw/core/core.go:817
summary: RegisterProvider appends without checking for duplicates — add a uniqueness check by provider ID
---

---
title: Flatten nested service function groups in StartProviders
added: 2026-06-07
effort: medium
tags: bw, providers, architecture
source: bw/core/core.go:831
summary: StartProviders iterates provider → service → service_fn — investigate removing a nesting level
---

---
title: Move bw.Start() to a dedicated bw/main module
added: 2026-06-07
effort: low
tags: bw, architecture
source: bw/core/core.go:899
summary: The Start() function in core.go is module-level bootstrapping that would be better placed in its own bw/main package
---

---
title: Stretch result columns to fill available width
added: 2026-06-07
effort: medium
tags: bw, gui
summary: Result table leaves empty space on the far right — a column should expand to consume the horizontal slack

Distinct from "Auto-size columns to fit content": that is per-column content fitting,
this is the table not filling its container's width. Tablelist supports a stretch
option (e.g. -stretch on a column or "all") to absorb leftover space.
---

---
title: GUI freezes on 'check for update' over a multi-row selection
added: 2026-06-14
effort: medium
tags: strongbox, gui, bug, performance
summary: Click-dragging to select multiple addons then choosing 'check for update' from the context menu freezes the GUI

Reported during alpha.4 UX testing. Likely the check runs synchronously on the Tk
thread across the whole selection.
---
