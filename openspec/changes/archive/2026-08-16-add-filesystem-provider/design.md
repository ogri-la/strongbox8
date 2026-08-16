# Design: add-filesystem-provider

## Context

See `proposal.md` for motivation. Current state that shapes the approach:

- The framework already models lazy children: `ITEM_CHILDREN_LOAD_LAZY` (`bw/core/iteminfo.go`), `Result.ChildrenRealised` (`bw/core/core.go:114`), and the on-demand entry point `core.Children(app, result)` (`bw/core/result.go:106`) which realises and stores children. `core.Children` currently has no callers.
- Known defect (`ISSUES.md`): `_realise_children` inverts the empty-policy/inherited-policy branches (`bw/core/result.go:44-48`), so a top-level lazy result is realised eagerly on first insert.
- The GUI already exposes expand/collapse hooks — `GUITablelist.OnExpandFnList`/`OnCollapseFnList` (`bw/ui/gui.go:80-118`), wired to Tablelist's row-expand/collapse events — but nothing subscribes to them (and implementation revealed the virtual events were bound to the wrong tag, so they had never fired). The double-click handler refuses to expand rows with zero inserted child rows (`gui.go:823-834`), and `TAG_SHOW_CHILDREN` handling skips non-eager rows, so an unrealised lazy row is inert today.
- `add_row_to_tree` panics if a child arrives before its parent row exists; here children are inserted under the row that was just expanded, so the parent always exists.
- The `bw` provider (`bw/bw/bw.go`) already owns an `os/fs` service group with `list-files` and `list-files-recursive-flat`, plus namespaces `bw/fs/file` and `bw/fs/dir`. Those services emit bare `string` items, which do not implement `core.ItemInfo` and cannot participate in the tree.
- State mutations run serially through `App.update_chan`; `GUIUI.RunService` runs work off the Tk thread and calls back on it. `TkSync` deadlocks if called from the Tk thread — expand callbacks fire on the Tk thread.

## Goals / Non-Goals

**Goals:**

- Close the lazy-loading loop end to end (policy honoured on insert → expand gesture → realisation off the Tk thread → children inserted) with the filesystem provider as its first user.
- Keep item types pure data with `ItemInfo` methods doing the only I/O (`ItemChildren` reads the directory), consistent with existing providers.
- Regression-safety for eager providers (strongbox, catalogue): no behavioural change to `LOAD_TRUE`/`LOAD_FALSE` paths.

**Non-Goals:**

- File operations (open, rename, delete), file metadata columns beyond the name, watching for filesystem changes, or refreshing an already-realised directory.
- Fixing `Catalogue.ItemChildren`'s re-expansion defect (`ISSUES.md`) — the catalogue benefits from the new expansion path but its own defect stays as recorded.
- CLI rendering — the CLI UI has been removed from the project; only the GUI exists. Stale mentions of the CLI (CLAUDE.md, code comments) are cleaned up as encountered during implementation.

## Decisions

1. **Extend the existing `bw` provider rather than add a new provider.** The `os/fs` service group, the `bw/fs/file` and `bw/fs/dir` namespaces and `DirArgDef` (directory-selection widget) already live in `bw/bw/bw.go`; a separate provider would duplicate registration for no isolation benefit. Alternative considered: a standalone `fs` provider module — rejected as premature until the fs surface grows.

2. **New `Dir` and `File` item types implementing `core.ItemInfo`.** `Dir.ItemHasChildren()` returns `ITEM_CHILDREN_LOAD_LAZY`; `Dir.ItemChildren(app)` performs one `os.ReadDir`, wrapping entries as new `Dir`/`File` results. `File.ItemHasChildren()` returns `ITEM_CHILDREN_LOAD_FALSE`. The absolute path is both the struct's key field and the result ID (mirrors `MakeAddonsDirResult` using the path as ID). Symlinks are classified by what they point at; a symlinked directory is browsable like any other — laziness means a symlink cycle only goes as deep as the user clicks, so no cycle detection is needed.

3. **Fix the `_realise_children` branch inversion as part of this change.** The lazy policy is unusable without it. The fix swaps the empty-policy/inherited-policy branches so an explicit `LAZY` on a top-level insert defers, and only the explicit realisation path (`core.Children`) loads. Guarded by unit tests over all three policies at top level and nested (the spec's regression scenarios).

4. **Placeholder child rows are a UI concern, not application state.** To give an unrealised lazy row its expand affordance, the GUI inserts a single placeholder tablelist row (the existing unused `dummy_row` mechanism) under it. Placeholders never enter `App` state — state stays a faithful model of realised data, and no filtering/diffing code needs to learn about fake results. Alternative considered: a nil-`Item` dummy `Result` in state (`build_treeview_row` already skips nil items) — rejected because every state consumer (filters, counts, removal cascades) would need to know about it.

5. **Expansion wiring: subscribe to `OnExpandFnList`.** The callback receives a tablelist fullkey; map it to a result ID via `tab.FkeyItemIndex`, look up the result, and if `!ChildrenRealised` and policy is `LAZY`, run `core.Children(app, result)` in a goroutine (the callback is on the Tk thread; realisation does blocking I/O and must not run there). The resulting `AddReplaceResults` flows through the normal `OnResultsChanged` diff, which inserts the child rows under the parent; placeholder upkeep (insert on unrealised lazy rows, remove once realised) happens in the row add/update paths on the Tk thread. The double-click toggle needed no adjustment: the placeholder row makes the child count non-zero, so lazy rows are already treated as expandable and do-not-load rows stay leaves. Re-expansion is a no-op because `ChildrenRealised` is true and rows already exist.

6. **Listing order is computed in `ItemChildren`: directories first, then files, each sorted by name.** Deterministic output keeps GUI diffing and tests stable. Hidden entries are included — filtering is a later, user-visible toggle if wanted.

7. **On-demand realisation is bounded by a timeout, enforced in the framework.** The realisation path behind `core.Children` runs `ItemChildren` in a goroutine and selects against a timer (default 5 seconds, a `bw/core` constant). On timeout the parent is marked realised with a single child: a new terminal item type (`ITEM_CHILDREN_LOAD_FALSE`, its own error namespace) representing the failure, plus a WARN log. Late results are dropped simply by discarding the goroutine's return value — because the parent is already realised-with-failure, nothing re-requests the load. No retry for now: this is not a forever-policy, just sufficient until the mechanism is proven; retry-on-re-expand would need refresh semantics the framework does not yet have. Note the timeout only guards the on-demand path — insert-time eager recursion cannot be wrapped in a timeout (state updates apply serially) and is guarded by the policy unit tests instead.

8. **Sibling row order follows snapshot order.** `DiffResults` previously iterated sets, so sibling rows appeared in scrambled order; it now reports IDs in snapshot order, meaning the GUI preserves the order `ItemChildren` returns. This is what makes the deterministic listing (decision 6) visible in the table, and it is guarded by a unit test.

9. **Read errors log at WARN and yield zero children.** For the user surface an unreadable directory is a normal environmental condition, not a program defect (per logging conventions, severity is scoped to the surface). `ItemChildren` returns an empty slice; the GUI removes the placeholder, leaving a leaf row.

## Risks / Trade-offs

- [Realisation marks `ChildrenRealised` permanently — a directory listed once never refreshes] → Accepted for this change; refresh is an explicit non-goal and the flag's semantics ("once loaded, they are not loaded again") are pre-existing.
- [Expand callback fires on the Tk thread; calling `TkSync` or blocking I/O there deadlocks or freezes the UI] → All realisation work runs in a goroutine; only placeholder insertion/removal happens on the Tk thread.
- [Fixing `_realise_children` changes behaviour for `Catalogue`, today's only `LAZY` item] → Its top-level insert stops being eagerly realised, which is the documented intended behaviour; the catalogue tab gains expansion via the new handler. Covered by a test around catalogue insertion.
- [Concurrent expansions of the same row (rapid double-clicks) could race realisation] → State mutations are serialised through `update_chan`, and `core.Children` re-checks `ChildrenRealised`; worst case is one redundant directory read. A per-row in-flight guard in the GUI handler avoids duplicate placeholder handling.
- [Huge directories (tens of thousands of entries) inserted in one update may stall the tablelist] → Partially mitigated: the realisation timeout converts a pathologically slow listing into a terminal 'failed' result; a merely large-but-fast listing still inserts in one update via the existing `InsertChildList` path.
- [A timed-out directory cannot be retried without re-browsing] → Accepted for now (explicitly not a forever-policy); revisit once refresh semantics exist.

## Migration Plan

No data or configuration migration. Pure addition plus a framework defect fix; rollback is reverting the change. The corresponding `ISSUES.md` entry is removed when the fix is verified by tests.
