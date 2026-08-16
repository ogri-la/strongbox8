# Add filesystem provider with lazy browsing

## Why

The `bw` framework is meant to be a general-purpose data browser, but its only filesystem services (`bw/fs` `list-files` and `list-files-recursive-flat`) emit bare strings with no hierarchy and either list a single level or eagerly walk an entire tree. There is no way to browse a directory tree incrementally through the UI. The framework already defines a lazy-children mechanism (`ITEM_CHILDREN_LOAD_LAZY`, `Result.ChildrenRealised`, `core.Children`) but it is unfinished: nothing triggers realisation from the UI, and a known defect (`ISSUES.md`) causes top-level lazy items to be realised eagerly. A filesystem provider is both a useful capability in itself and the concrete driver that completes the framework's lazy-loading loop.

## What Changes

- New `File`/`Dir` item types in the `bw` provider implementing `core.ItemInfo`: directories report `ITEM_CHILDREN_LOAD_LAZY`, files report `ITEM_CHILDREN_LOAD_FALSE`. Results use the absolute path as their stable ID.
- New `browse` service (`fs-browse`) in the existing `os/fs` service group: given a directory (validated by the existing `DirArgDef`, which gains a text-field input widget), it returns a single lazy root result for that directory. Children are realised one level at a time as the user expands rows.
- Framework fix: `_realise_children` currently inverts the empty-policy/inherited-policy branches, eagerly realising top-level lazy items on insert (`ISSUES.md`). Lazy items must not be realised until explicitly requested.
- GUI: wire the unused `GUITablelist.OnExpandFnList`/`OnCollapseFnList` hooks so expanding a row whose result has unrealised lazy children calls `core.Children` off the Tk thread and inserts the results under the parent row.
- Framework: on-demand realisation is bounded by a timeout. If loading a row's children exceeds a few seconds, the load is abandoned, a single terminal 'failed' result is presented as the child instead, and results arriving after the timeout are discarded. This bounds the damage from any accidental eager recursive realisation of a large tree.
- GUI: unrealised lazy rows currently cannot be expanded at all (the double-click handler refuses rows with zero inserted children, and `TAG_SHOW_CHILDREN` skips non-`LOAD_TRUE` rows). Lazy rows with unrealised children get a placeholder child row (the existing unused `dummy_row` mechanism) so the expand affordance exists; the placeholder is replaced by real children on first expansion.
- Strongbox: a "files" tab showing `bw/fs` results and load failures, seeded on start with the user's home directory as a browsable root.
- A File → "Browse Directory" menu entry on the bw provider so the `fs-browse` service is invocable from the GUI.
- Framework defects found and fixed during implementation: expand/collapse virtual events were bound to the wrong tag and never fired; `DiffResults` iterated sets and scrambled sibling row order; `DirArgDef` had no input widget so forms rendering it panicked; `ServiceID` menu entries deadlocked the Tk thread; children arriving after an excluded ancestor panicked `add_row_to_tree`.

## Capabilities

### New Capabilities

- `bw/lazy-child-realisation`: framework behaviour for results whose children load on demand — the `LAZY` policy is honoured on insert, `core.Children` realises exactly one level exactly once, realisation is recorded so it is not repeated.
- `bw/gui-lazy-expansion`: GUI behaviour for lazy rows — expand affordance on unrealised lazy rows, realisation triggered by row expansion, work performed off the Tk thread, children appearing under the correct parent row.
- `bw/filesystem-browsing`: the filesystem provider surface — the `browse` service, `File`/`Dir` item shapes, stable path-based IDs, lazy directory listing semantics, error behaviour for unreadable paths.

### Modified Capabilities

None — no existing specs; this change introduces the first.

## Impact

- `bw/bw/bw.go`: new item types and `browse` service alongside the existing `bw/fs/file` and `bw/fs/dir` namespaces.
- `bw/core/result.go`: fix the inverted policy branches in `_realise_children`; `core.Children` gains its first caller.
- `bw/ui/gui.go`: expand/collapse subscription, placeholder child rows, mapping tablelist full-keys back to result IDs via `FkeyItemIndex`.
- `strongbox/main.go`: the "files" tab and its home-directory root; GUI tests in `strongbox/main_test.go`.
- `bw/core/ui.go`: `DiffResults` reports IDs in snapshot order. `bw/core/argdefs.go`: `DirArgDef` gains an input widget.
- `ISSUES.md`: the "top-level lazy items realised eagerly" defect is resolved by this change.
- Existing eager providers (strongbox, catalogue) keep their behaviour; `Catalogue` is the only current `LAZY` item and gains a working expansion path, though its own re-expansion defect remains out of scope.
- No new dependencies; standard library `os` only.
