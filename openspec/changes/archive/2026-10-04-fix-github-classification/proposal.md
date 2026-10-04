# Fix Github release classification

## Why

Github release classification and `.toc` classification decide which game tracks an update or an installed addon supports. Test coverage added on this branch found five defects in the Github path (`ISSUES.md`), and the review of them found more in the `.toc` path. In several places classification falls back to retail when it cannot decide. That was workable when WoW had one or two game tracks. It is not now, with many classic versions, the forever client, multi-interface `.toc` files and multi-flavour `release.json` files. Strongbox 8 is strict: classify from the information available, and leave out what cannot be classified rather than assume retail.

## What Changes

- An asset still unclassified after all classification passes is excluded from the update list, with a DEBUG log. Today it is labelled retail with a WARN.
- The second pass no longer assumes retail for a lone unclassified asset when several game tracks are left. It assigns a game track only when exactly one is left.
- The `release.json` is downloaded for the newest release that is not a draft or prerelease, wherever that release appears in the Github response.
- An unrecognised `release.json` flavour contributes no game track. An entry with no recognised flavour leaves the earlier passes' result unchanged instead of overwriting it.
- A malformed `release.json` is a recoverable error instead of a panic. The release keeps the earlier passes' game tracks.
- `GuessGameTrack` recognises `cata` as a delimited word and checks it before wotlk. It also recognises `standard` as retail.
- Interface version to game track is strict: 4.x is classic-cata. The forever range (1.60–1.99) and 5.x (mists) give no game track until those game tracks exist. Only 6.x and later are retail.
- `.toc` selection in relaxed mode returns the most-preferred game track's `.toc` instead of the least-preferred. In strict mode it is deterministic: the most specific `.toc` wins, then the lowest file path.
- `ISSUES.md`: the six Github classification entries are removed. The `gametrack_set` entry was fixed earlier on this branch.

## Non-goals

- The `classic-mists` and `forever` game tracks are a separate change. Until then, mists and forever assets and interface versions are unclassifiable.
- Filling in game tracks for unclassified assets from the catalogue or the installed `.toc`. This is the `todo` at the end of `process_github_release_list`.
- Classifying assets by inspecting their zip contents. An install-time check that a downloaded zip's `.toc` supports the target game track is a separate change.

## Capabilities

### New Capabilities

- `strongbox/github-classification`: how a Github release list becomes game-track-labelled updates. Covers release and asset filtering, game track guessing from names, sibling inference, `release.json` authority and failure handling, exclusion of unclassifiable assets, and ordering.
- `strongbox/toc-classification`: how a `.toc` file's interface versions and file name become its game tracks.
- `strongbox/toc-selection`: which `.toc` of an installed addon is used for a game track in strict and relaxed modes.

### Modified Capabilities

None. There are no existing `strongbox` specs.

## Impact

- `strongbox/src/github_api.go`: published-release filtering, no retail assumption in `classify2`, the final pass excludes, and `release.json` errors are handled.
- `strongbox/src/release_json.go`: unrecognised flavours are dropped, and `ParseReleaseJSON` returns its error instead of panicking.
- `strongbox/src/utils.go`: `cata` and `standard` patterns, regexes compiled once, and interface version to game track mapping from the integer.
- `strongbox/src/addon.go`: `_make_addon__find_toc` in both modes.
- An installed addon whose only `.toc` declares a cata interface is classic-cata instead of retail. One declaring a mists or forever interface, with no game track in its file name, has no game track.
- Tests: the characterisation tests that record the defects are changed to assert the corrected behaviour. New tests cover interface version mapping and `.toc` selection, which have none today.
- No new dependencies.
