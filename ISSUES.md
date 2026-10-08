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
title: the atk fork fails Go's pointer checks
added: 2026-10-05
effort: medium
tags: atk, race
location: atk/tk/interp/interp_unix.go
summary: `CreateAction` passes an integer handle to C as an `unsafe.Pointer`, which `checkptr` rejects

`-race` turns on `checkptr`, and the GUI then fails at its first menu
action with "pointer arithmetic computed bad pointer value".
`manage.sh test` runs with `-gcflags=all=-d=checkptr=0` to get past it.
`runtime/cgo.Handle`, or a C-side table keyed by integer, would avoid
converting an integer to a pointer.
---

---
title: `tk.Async` reads the main loop thread without synchronisation
added: 2026-10-05
effort: low
tags: atk, bw, gui, race
location: atk/tk/interp/interp_unix.go, bw/ui/gui.go
summary: `MainLoop` writes `mainLoopThreadId` and `Async` reads it with no lock, and the GUI's service worker can still call `Async` after the main loop has ended

Found by `-race` when a test built a GUI after another test had stopped
Tk. The tests now replace `GUIUI.async`. In the application, a service
finishing after the window closes would send an event to a nil thread.
Guarding the thread ID in atk, or having the service worker stop before
`Tk_MainLoop` returns, are the candidate fixes.
---

---
title: Tk initialisation crashed once while starting the GUI test
added: 2026-10-05
effort: unknown
tags: atk, gui, crash
location: atk/tk/tk.go, bw/ui/gui.go
summary: A SIGSEGV inside `Tcl_EvalEx`, called from `tk.InitEx` → `Window.SetMenu`, before any test code ran

Seen once during `./manage.sh coverage` on a real X display, in about 20
GUI test runs that day. It did not happen again in 8 runs of the
strongbox GUI test or in 2 more coverage runs. The fault address was
0x4, so a near-nil pointer dereference in Tk's menu setup. Note it if it
happens again, along with the display (Xvfb or real) and the window
manager.
---
