# ISSUES

A structured capture of known defects.
Each item is separated by `---` and has key-value metadata followed
by optional free-text context.

Unlike `TODO.md`, entries here record something observed to be wrong in
code that has been read, so they name the location and the evidence.

---
title: `Result.IsEmpty` always returns false
added: 2026-08-15
effort: medium
tags: bw, core, correctness
location: bw/core/core.go
summary: Compares the receiver against the address of a fresh struct, so no result is ever reported as empty

`r == &Result{}` compares two pointers that can never be equal. Every
`IsEmpty()` guard in the tree-walking code is therefore dead, including
the not-found checks in `FindResultByID`, `FindRootResult` and
`FindParents`. `FindRootResult` returns a pointer to an empty `Result`
for a missing ID rather than nil. The loops still terminate, via the
`ParentID == ""` branch, so this shows up as a wrong return value rather
than a hang. `HasResultValidator` depends on it too and so accepts a
missing result.
---

---
title: `Catalogue.ItemChildren` cannot expand a loaded catalogue
added: 2026-08-15
effort: low
tags: strongbox, catalogue
summary: Expanding the row calls `_db_load_catalogue`, which treats an already-loaded catalogue as an error and returns none

`_db_load_catalogue` returns an error when a catalogue is already in
state. `ItemChildren` calls it to get the addons to display, so once the
catalogue has been loaded the row expands to nothing. Reading the
catalogue from disk to list children that are already in state may be
the wrong shape here.
---

---
title: `install_addon` swallows a failed uninstall and a failed unzip
added: 2026-08-15
effort: medium
tags: strongbox, install
location: strongbox/src/core.go
summary: Both failures are logged and installation continues, so nfo files are written for an addon that may not be on disk

`remove_addon` and `unzip_file` errors are logged, then
`update_nfo_files` runs regardless. The result is nfo data describing an
installation that did not happen. Worth deciding whether either failure
should abort.
---

---
title: `download_catalogue_addon` indexes the first update without checking
added: 2026-08-15
effort: low
tags: strongbox, catalogue
location: strongbox/src/core.go
summary: `summary_list[0]` panics when a source returns no updates

A source that returns an empty list is a plausible runtime condition —
an addon with no releases yet, or a host filtering everything out —
rather than a programming error, so a panic may be the wrong response
here even under the fail-hard-while-developing preference.
---

---
title: `read_settings_file` extension check never matches
added: 2026-08-15
effort: low
tags: strongbox, settings
location: strongbox/src/settings.go
summary: Compares `filepath.Ext(path)` against "json", but `Ext` returns a leading dot

The guard intends to reject a settings path that is not a `.json` file
and never fires. Note the condition is also inverted with respect to its
error message.
---

---
title: `expand_row` logs a successful expansion at WARN
added: 2026-08-15
effort: low
tags: bw, gui, logging
location: bw/ui/gui.go
summary: The success branch logs WARN while the failure branch logs ERROR

A row expanding normally is not a deviation from expectations. Should be
DEBUG, or dropped.
---

---
title: `NFO.IsEmpty` has the same broken pointer comparison as `Result.IsEmpty`
added: 2026-08-15
effort: low
tags: strongbox, correctness
location: strongbox/src/nfo.go
summary: The pointer check is dead, though the `GroupID` check below it means the function still behaves correctly

Lower severity than the `Result` case because the fallback carries the
real logic. Worth fixing alongside it so the pattern does not get copied
again.
---
