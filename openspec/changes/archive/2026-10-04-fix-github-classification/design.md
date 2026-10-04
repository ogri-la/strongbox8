## Context

`process_github_release_list` (`strongbox/src/github_api.go`) runs a pipeline for each release:

1. filter assets
2. `to_sul`
3. `classify1`, per asset
4. `classify2`, per release
5. `classify3`, newest release only, from its `release.json`
6. force retail on anything still empty

`.toc` game tracks (`strongbox/src/toc.go`) come from `InterfaceVersionToGameTrack` and from `GuessGameTrack` on the file name suffix. `InterfaceVersionToGameTrack` goes through `InterfaceVersionToGameVersion`, which matches the decimal digits with a regex, and then `GameVersionToGameTrack`, which looks at the first two characters.

That regex misreads six-digit interface versions and two-digit minors:

| interface | regex result | game track today |
|---|---|---|
| `110002` | `1.0.02` | classic |
| `11503` | `1.5.3` | classic |
| `16000` | `1.0.0` | classic |
| `40400` | `4.4.0` | retail |

So current retail `.toc` files are classified as classic, and cata `.toc` files as retail. `InterfaceVersionToGameVersion` is also used for display, in `addon.go` and `toc.go`.

Requirements are in `specs/strongbox/github-classification/spec.md`, `specs/strongbox/toc-classification/spec.md` and `specs/strongbox/toc-selection/spec.md`.

## Goals / Non-Goals

**Goals:**

- Every update returned has a non-empty game track set containing only supported game tracks. Every `.toc` game track set contains only supported game tracks. Both invariants are tested.
- No Github or `.toc` classification path assigns retail except from evidence: a name, an interface version of 6.x or later, a `release.json` flavour, a publication date before WoW Classic, or sibling inference with exactly one game track left. WowInterface updates are outside this change and are recorded in `ISSUES.md`.
- Each fix is a change to an existing pure function, or the extraction of one.

**Non-Goals:**

- Making `download_release_json` testable without a `core.App`. `core.MapDownloader` already lets tests go through `app.Download`.
- The `.toc` file name regex in `toc.go` and its `todo` about directory names. That logic already works through `GuessGameTrack`, so it gains `cata` and `standard` for free.

## Decisions

### Interface versions are parsed arithmetically, and game tracks come from a range table

An interface version is an integer, `major * 10000 + minor * 100 + patch`. A pure `parse_interface_version(iv int) (major, minor, patch int, err error)` uses division and remainder, which handles any number of digits without a regex. It returns an error for versions outside `10000`–`999999`, the 5- and 6-digit range, and `InterfaceVersionToGameVersion` and `InterfaceVersionToGameTrack` pass that error on. `InterfaceVersionToGameVersion` formats the parts, so `110002` becomes `11.0.2` and `11503` becomes `1.15.3`. This matches the Clojure implementation, which moved to the same arithmetic in `ff52266` ("Forever support").

The game track comes from a table of half-open interface version ranges, kept in order. The spec's mapping is literally a list of ranges, so a table states it directly and the mists and forever change becomes a matter of adding rows:

```
[10000, 16000)  classic
[16000, 20000)  none      (forever)
[20000, 30000)  classic-tbc
[30000, 40000)  classic-wotlk
[40000, 50000)  classic-cata
[50000, 60000)  none      (mists)
[60000, 1000000) retail
```

`InterfaceVersionToGameTrack` returns `""` for a version with no game track, and `toc.go` skips `""` the same way it skips an error. `GameVersionToGameTrack` has no callers left after this and is deleted.

Alternative: patch the regex and add a minor-version check to `GameVersionToGameTrack`. That keeps two string conversions on an integer, and the defect came from those conversions.

### Filter releases before the loop, so index 0 is the newest published release

The release list is a sequence and the defect is an index into the wrong sequence. A pure `published_release_list(release_list)` drops drafts and prereleases first. The loop then runs over that list, and `i == 0` means what the code intends.

Alternative: a `seen_published` flag set inside the loop. It works, but keeps two different notions of position in one loop, which is how the defect arose.

### Drop unrecognised flavours at the source, and omit entries left empty

`ReleaseJSONGameTrackMap` and `ReleaseJSONGameTrackList` skip the empty string `GuessGameTrack` returns. `ReleaseJSONGameTrackMap` also leaves out a release whose set ends up empty. `classify3` already keeps the earlier passes' game tracks when an asset has no entry, so "no recognised flavour" and "no entry" become the same case and `classify3` needs no new branch.

Alternative: guard in `classify3`. This leaves `ReleaseJSONGameTrackMap` returning sets that contain `""`, so any other caller would hit the same defect.

### `release.json` failures are values, not panics

`ParseReleaseJSON` returns its error and the `panic` is removed. `process_github_release_list` already handles a download error by skipping `classify3`, and a parse error takes the same path. Log levels follow who can act on the problem:

- A failed download is an environment problem the user may be able to fix, such as the network or rate limiting, and classification recovers. It moves from ERROR to WARN.
- A malformed `release.json`, an asset missing from it, or a flavour strongbox does not know is the addon author's data. The user cannot fix it and classification recovers. These are logged at DEBUG. "release.json missing asset" moves from ERROR to DEBUG.

### Delimited `cata` and `standard` patterns; regexes compiled once

Go's RE2 has no lookaround, so a delimited word is written `(?i)(^|[^[:alnum:]])cata([^[:alnum:]]|$)`, and the same for `standard`. `[^[:alnum:]]` treats `_` as a delimiter, which matches how asset names use it (`1.2.3_cata`). The `cata` check goes before the wotlk check.

The Clojure pattern `[\W_]?cata([\W_]?|$)` makes both delimiters optional, so it matches `Catalyst` and `catalogue`. The Go version requires delimiters on purpose.

`GuessGameTrack` calls `regexp.MustCompile` four times on every call, and it runs at least twice per asset and once per `.toc`. The regexes move to package-level variables.

### `classify2` keeps only the exactly-one-left inference

The branch that assumes retail when several game tracks are left is deleted. The exactly-one-left branch stays. It is sound inference while the set of supported game tracks is complete, and the mists and forever change restores that.

### The final pass filters instead of assigning

Step 6 becomes a filter: updates with an empty game track set are left out, with `slog.Debug` naming the asset.

### `.toc` selection uses one ranking for both modes

A pure `best_toc(toc_map, game_track_id) *TOC` returns the `.toc` that supports `game_track_id`, or nil. Among several candidates it ranks by fewest game tracks, then lowest path. Map iteration order never affects the result. Strict mode calls it once. Relaxed mode calls it for each game track in the preference order and returns the first non-nil result. That also fixes the relaxed-mode `break` defect.

"Fewest game tracks" is the specificity rule because a suffixed `.toc` such as `EveryAddon_Cata.toc` exists to serve one game track, while an unsuffixed multi-interface `.toc` is the general case.

### Testing

The characterisation tests named in `tasks.md` are changed to assert the specified behaviour and lose their "characterisation test" comments. New tests:

- table tests for `parse_interface_version`, `InterfaceVersionToGameVersion` and `InterfaceVersionToGameTrack` covering every range boundary (`9999`, `10000`, `15999`, `16000`, `19999`, `20000`, `59999`, `60000`, `999999`, `1000000`). The `InterfaceVersionToGameVersion` cases are ported from the Clojure `interface-version-to-game-version` test.
- a Go fuzz test for `GuessGameTrack`: for any input, the result is `""` or a member of `SUPPORTED_GAME_TRACKS`
- a property test for `InterfaceVersionToGameTrack` over random integers: the result is `""` or a member of `SUPPORTED_GAME_TRACKS`, and it never panics
- a property test over generated `.toc` interface versions and file names: every `.toc` game track is supported
- log level tests: an excluded asset is logged at DEBUG, a failed `release.json` download at WARN, and a malformed `release.json` at DEBUG
- a property test over generated release lists: no update has an empty game track set or an unsupported game track, and at most one `release.json` is requested
- `best_toc` and `_make_addon__find_toc` tests for both modes, including a tie-break test that runs repeatedly to catch dependence on map order

## Risks / Trade-offs

- [Some Github addons will produce no updates where a retail update was guessed before] → Intended. The `release.json` and pattern fixes recover some of them. The project is unreleased, so no user's installed state depends on the old behaviour.
- [An installed addon whose `.toc` declares only a mists or forever interface has no game tracks until those game tracks are added] → Its strict-mode `.toc` lookup finds nothing, so it shows no `.toc` details and gets no update. That is correct for an addon strongbox cannot classify yet.
- [Delimited `standard` could match an addon name such as `Standard-UI-1.0.zip` and label it retail] → Matches the Clojure behaviour for the same input. `standard` is checked last, so any other game track in the name wins.
- [Interface version `60000` and above is retail, so a future classic progression client at 6.x would be classified as retail] → The range table makes that a one-row change when such a client exists.
