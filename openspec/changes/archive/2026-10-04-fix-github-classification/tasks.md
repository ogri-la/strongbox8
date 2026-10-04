# Tasks: fix-github-classification

## 1. Interface versions

- [x] 1.1 Add table tests for `InterfaceVersionToGameVersion`, ported from the Clojure `interface-version-to-game-version` test in `/home/torkus/dev/clojure/strongbox/test/strongbox/utils_test.clj` (including `11302` → `1.13.2`, `11507` → `1.15.7`, `16001` → `1.60.1`, `101010` → `10.10.10`, `110002` → `11.0.2`; `1234` and `1234567` → error) and `InterfaceVersionToGameTrack` (every row and boundary in `specs/strongbox/toc-classification/spec.md`). The tests fail.
- [x] 1.2 Add a pure `parse_interface_version` using integer arithmetic, with tests. `InterfaceVersionToGameVersion` formats its result, and the regex is removed.
- [x] 1.3 Add the interface version range table. `InterfaceVersionToGameTrack` looks the version up in it and returns `""` when there is no game track. Delete `GameVersionToGameTrack`. Tests from 1.1 pass.
- [x] 1.4 `toc.go` skips `""` game tracks from interface versions. Add `.toc` tests for the multi-interface, unsupported-interface-only and file-name-supplies-game-track scenarios.
- [x] 1.5 Add a property test over random integers: `InterfaceVersionToGameTrack` never panics, and returns `""` or a member of `SUPPORTED_GAME_TRACKS`
- [x] 1.6 Check the display call sites of `InterfaceVersionToGameVersion` in `addon.go` and `toc.go`, and update any test fixtures that recorded the old misread game versions

## 2. Game track guessing

- [x] 2.1 Rewrite `TestGuessGameTrack__known_gaps` as `TestGuessGameTrack__cata_and_standard`, asserting the specified results: delimited `cata` forms give classic-cata, `retail-classic-tbc-classic-wotlk-cata` gives classic-cata, `Catalyst-1.0.zip` is not classic-cata, delimited `standard` gives retail, `mists` gives `""`. The test fails.
- [x] 2.2 Move the `GuessGameTrack` regexes to package-level variables. Add the delimited `cata` pattern before wotlk and the delimited `standard` pattern in the retail check. Tests from 2.1 and the existing `TestGuessGameTrack` cases pass.
- [x] 2.3 Add a fuzz test `FuzzGuessGameTrack`, seeded with the existing cases: the result is always `""` or a member of `SUPPORTED_GAME_TRACKS`

## 3. release.json

- [x] 3.1 Change `TestReleaseJSONGameTrackMap__unknown_flavor`, `TestReleaseJSONGameTrackMap__partially_unknown_flavor` and `Test_classify3__unknown_flavor_erases_a_good_guess` to assert the specified behaviour: an entry with only unrecognised flavours is absent from the map, a partly recognised entry holds only the recognised game tracks, `""` is never in a set, and `classify3` keeps the classic guess. The tests fail.
- [x] 3.2 `ReleaseJSONGameTrackMap` and `ReleaseJSONGameTrackList` skip unrecognised flavours, and `ReleaseJSONGameTrackMap` leaves out entries with an empty set. Tests from 3.1 pass.
- [x] 3.3 Add `TestParseReleaseJSON__malformed` asserting an error and no panic, and a `process_github_release_list` test where the `release.json` body is malformed: that release keeps its earlier game tracks, and a second release's updates are still returned. The tests fail (panic).
- [x] 3.4 Remove the `panic` from `ParseReleaseJSON` and update its doc comment. Tests from 3.3 pass.
- [x] 3.5 Set log levels as design.md describes: download failure WARN; parse failure, missing asset and unrecognised flavour DEBUG

## 4. Classification passes

- [x] 4.1 Change `Test_classify2__assumes_retail` to assert that the lone unclassified update stays unclassified when several game tracks are left, and check `Test_classify2_using_2` and `Test_classify2_using_3` against the spec. The tests fail where they relied on the retail assumption.
- [x] 4.2 Delete the retail-assumption branch from `classify2` and update its doc comment. Tests from 4.1 pass.
- [x] 4.3 Change `Test_process_github_release_list__draft_first_skips_release_json` to assert that the `release.json` is requested once and the update is classic-wotlk. Add a test where a prerelease is first. The tests fail.
- [x] 4.4 Add a pure `published_release_list` that drops drafts and prereleases, with a unit test. `process_github_release_list` iterates over its result so `i == 0` is the newest published release. Tests from 4.3 and the existing release.json tests pass.
- [x] 4.5 Add a test that no `release.json` is downloaded when the newest published release has none and an older release does
- [x] 4.6 Replace `Test_process_github_release_list__unclassified_assumed_retail` with `Test_process_github_release_list__unclassified_excluded`: a single unhinted zip produces no updates, and `Addon-classic.zip` + `Addon-mists.zip` produce only the classic update. The tests fail.
- [x] 4.7 The final pass in `process_github_release_list` leaves out updates with an empty game track set and logs each at DEBUG. Update the function's doc comment. Tests from 4.6 pass.
- [x] 4.8 Add a property test over generated release lists (random asset name hints, draft and prerelease flags, dates, and valid, malformed or missing `release.json` bodies): no returned update has an empty game track set or an unsupported game track, and at most one `release.json` is requested. The fixtures are inline per test, so a generated property test replaces iterating them.

## 5. .toc selection

- [x] 5.1 Add tests from `specs/strongbox/toc-selection/spec.md` for both modes: strict match, strict most-specific wins, strict tie-break by path (run repeatedly to catch map-order dependence), strict no match, relaxed prefers classic-cata for a classic-wotlk dir, relaxed exact match. The relaxed preference and tie-break tests fail.
- [x] 5.2 Add a pure `best_toc(toc_map, game_track_id)` that ranks by fewest game tracks, then lowest path. `_make_addon__find_toc` uses it in both modes. Tests from 5.1 pass.

## 6. Records and verification

- [x] 6.1 Remove the six resolved Github classification entries from `ISSUES.md`: `release.json` newest release, unrecognised flavour, cata/mists/standard, unclassified assumed retail, `find_toc`, `gametrack_set`
- [x] 6.2 Run `./manage.sh test`. All tests pass.

## 7. Verification follow-ups (added during verify)

- [x] 7.1 Cover the remaining spec scenarios in tests: a `release.json` asset in the asset filter test, asset order within a release, asset name beating release name in `classify1`, partly recognised flavours through `classify3`, and the spec's `Addon-1.2.3-mists.zip` string
- [x] 7.2 Add log level tests: exclusion at DEBUG, `release.json` download failure at WARN, malformed `release.json` at DEBUG
- [x] 7.3 Add a direct `best_toc` test and a `.toc` game track property test, as design.md lists
- [x] 7.4 Correct design.md: `parse_interface_version` returns an error, and the retail-from-evidence goal covers the Github and `.toc` paths only
- [x] 7.5 Record in `ISSUES.md` that every WowInterface update is assumed to be retail
