# Tasks: add-filesystem-provider

## 1. Framework: honour the lazy policy

- [x] 1.1 Add failing unit tests in `bw/core` covering all three child-loading policies on insert, top-level and nested: eager loads immediately, do-not-load never loads, lazy defers (reproduces the `ISSUES.md` inverted-branch defect)
- [x] 1.2 Fix the empty-policy/inherited-policy branch inversion in `_realise_children` (`bw/core/result.go:44-48`) so lazy items are not realised on insert; tests from 1.1 pass
- [x] 1.3 Add unit tests for `core.Children`: first call realises exactly one level, marks the result realised, stores children with the correct `ParentID`; second call returns the same children without invoking `ItemChildren` again; lazy grandchildren stay unrealised
- [x] 1.4 Add a regression test that inserting a `Catalogue` result (the existing `LAZY` item) no longer realises it eagerly
- [x] 1.5 Add a terminal 'failed' item type in `bw/core` (`ITEM_CHILDREN_LOAD_FALSE`, own error namespace) representing an abandoned load
- [x] 1.6 Add a timeout (default 5s constant) to the on-demand realisation path behind `core.Children`: `ItemChildren` runs in a goroutine selected against a timer; on timeout the parent is marked realised with a single 'failed' child and a WARN is logged
- [x] 1.7 Unit tests with a deliberately slow `ItemChildren` fixture: timeout yields exactly one non-expandable 'failed' child; late results from the abandoned load never enter state; a fast fixture is unaffected
- [x] 1.8 Remove the resolved "top-level lazy items realised eagerly" entry from `ISSUES.md`

## 2. Filesystem item types and browse service

- [x] 2.1 Add `Dir` and `File` item types to `bw/bw/bw.go` implementing `core.ItemInfo`: `Dir` reports `ITEM_CHILDREN_LOAD_LAZY`, `File` reports `ITEM_CHILDREN_LOAD_FALSE`; both expose the entry name via `ItemKeys`/`ItemMap`
- [x] 2.2 Implement `Dir.ItemChildren`: one `os.ReadDir`, entries wrapped as `Dir`/`File` results with the absolute path as result ID, directories first then files, each group sorted by name; symlinks classified by target type
- [x] 2.3 On read error, `Dir.ItemChildren` logs at WARN and returns zero children
- [x] 2.4 Add the `browse` service to the `os/fs` service group using `DirArgDef` validation, returning a single unrealised lazy `Dir` result for the given directory
- [x] 2.5 Unit tests over a fixture directory tree: browse adds one root with nothing read; expanding lists one level only; ordering is deterministic; hidden entries included; repeat browse of the same path does not duplicate results; unreadable directory yields zero children plus a WARN log

## 3. GUI: lazy row expansion

- [x] 3.1 Insert a placeholder child row (`dummy_row`) under rows whose result has unrealised `LAZY` children, in `add_row_to_tree` and `update_row_in_tree`; placeholders never enter application state
- [x] 3.2 Adjust the double-click toggle so a row whose only child is a placeholder counts as expandable; rows with the do-not-load policy remain leaves
- [x] 3.3 Subscribe to `GUITablelist.OnExpandFnList`: map fullkey to result ID via `tab.FkeyItemIndex`, and when the result has unrealised lazy children run `core.Children` in a goroutine (never on the Tk thread), with a per-row in-flight guard against duplicate expansion
- [x] 3.4 Remove the placeholder on the Tk thread once realised children are inserted (or when realisation yields zero children, leaving a leaf row)
- [x] 3.5 Verify collapse then re-expand redisplays existing child rows without re-invoking realisation
- [x] 3.6 GUI tests (Xvfb): lazy row shows expand affordance; expansion inserts children under the correct parent and removes the placeholder; empty-directory expansion leaves a leaf; re-expansion performs no reload
- [x] 3.7 GUI test: a timed-out expansion replaces the placeholder with a single non-expandable 'failed' row while the UI stays responsive

## 4. Verification

- [x] 4.1 Remove stale CLI-UI mentions from `CLAUDE.md` and from comments in any files touched during implementation (the CLI UI has been removed from the project)
- [x] 4.2 Run `./manage.sh test` across all modules; all tests pass
- [x] 4.3 Manual check via the GUI: browse a directory, expand several levels including a large directory and an unreadable one, confirm responsiveness and WARN logging

## 5. Surfacing and defects found during implementation (added during apply)

- [x] 5.1 Add a "files" tab to `strongbox/main.go` showing `bw/fs` results and load failures (user-approved addition)
- [x] 5.2 Add a File → "Browse Directory" menu entry to the bw provider so the `fs-browse` service is invocable from the GUI
- [x] 5.3 Fix: expand/collapse virtual events were bound to the tablelist body tag but generated on the widget itself, so they never fired (`bw/ui/gui.go` binds them directly now)
- [x] 5.4 Fix: `ServiceID` menu entries deadlocked the Tk thread by opening the service form synchronously from a menu command
- [x] 5.5 Fix: `DirArgDef` had no input widget, so any form rendering it panicked; it now uses the text-field widget like the addons-dir argdef
- [x] 5.6 Fix: `DiffResults` iterated sets, scrambling sibling row order in the GUI; it now reports IDs in snapshot order, with a unit test
- [x] 5.7 Fix: children arriving in a later update than an excluded ancestor panicked `add_row_to_tree`; a parent present in state but without a row now excludes its subtree from that tab
