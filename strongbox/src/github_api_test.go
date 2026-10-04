package strongbox

import (
	"bw/core"
	"bytes"
	"fmt"
	"log/slog"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/quick"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

var dummy_dt_pre_classic = time.Date(2015, 12, 31, 23, 59, 59, 0, time.UTC)
var dummy_dt = time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC)

//

func Test_github_release_list_url(t *testing.T) {
	expected := "https://api.github.com/repos/AdiAddons/AdiBags/releases?per-page=100&page=1"
	source_id := "AdiAddons/AdiBags"
	assert.Equal(t, expected, github_release_list_url(source_id))
}

func Test_is_release_json(t *testing.T) {
	var cases = []struct {
		given    string
		expected bool
	}{
		{"release.json", true},
		{"Release.Json", true},
		{"RELEASE.JSON", true},
		{" release.json ", true},
		{"", false},
		{" ", false},
		{"Addon-v1.2.3.zip", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, is_release_json(GithubReleaseAsset{
			Name: c.given,
		}))
	}
}

func Test_is_supported_zip(t *testing.T) {
	var cases = []struct {
		given    string
		expected bool
	}{
		{"application/zip", true},
		{"application/x-zip-compressed", true},
		{"application/zip; application/x-zip-compressed", false}, // this isn't html!
		{"", false},
		{" ", false},
		{"text/plain", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, is_supported_zip(c.given))
	}
}

func Test_is_fully_uploaded(t *testing.T) {
	var cases = []struct {
		given    string
		expected bool
	}{
		{GITHUB_RELEASE_ASSET_STATE_UPLOADED, true},
		{GITHUB_RELEASE_ASSET_STATE_OPEN, false},
		{"no", false},
		{"yes", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, is_fully_uploaded(c.given))
	}
}

func Test_pick_asset_version_name(t *testing.T) {
	var cases = []struct {
		release_name string
		release_tag  string
		asset_name   string
		expected     string
	}{
		{"Foo", "Bar", "Baz", "Foo"},
		{"", "Bar", "Baz", "Bar"},
		{"", "", "Baz", "Baz"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, pick_asset_version_name(
			GithubRelease{
				Name:    c.release_name,
				TagName: c.release_tag,
			}, GithubReleaseAsset{
				Name: c.asset_name,
			}))
	}
}

func Test_to_sul(t *testing.T) {
	r := GithubRelease{
		Name:          "Addon-v1.2.3",
		PublishedDate: dummy_dt,
	}
	al := []GithubReleaseAsset{
		{
			Name:               "Addon-v1.2.3.zip",
			BrowserDownloadURL: "https://example.org/foo/bar.zip",
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3.zip",
			Version:        "Addon-v1.2.3",
			DownloadURL:    "https://example.org/foo/bar.zip",
			PublishedDate:  dummy_dt,
			GameTrackIDSet: mapset.NewSet[GameTrackID](),
		},
	}
	assert.Equal(t, expected, to_sul(r, al))
}

func Test_classify1__unclassified(t *testing.T) {
	r := GithubRelease{
		Name: "Addon-v1.2.3",
	}
	su := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	expected := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	assert.Equal(t, expected, classify1(r, su))
}

// a game track (retail, classic, etc) was found in the asset name.
func Test_classify1__game_track_from_asset(t *testing.T) {
	r := GithubRelease{
		Name: "Addon-v1.2.3",
	}
	su := SourceUpdate{
		AssetName:      "Addon-v1.2.3--classic.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	expected := SourceUpdate{
		AssetName:      "Addon-v1.2.3--classic.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
	}
	assert.Equal(t, expected, classify1(r, su))
}

// addon was published before classic was a thing.
func Test_classify1__game_track_from_pubdate(t *testing.T) {
	r := GithubRelease{
		Name: "Addon-v1.2.3",
	}
	su := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt_pre_classic,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	expected := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt_pre_classic,
		GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL),
	}
	assert.Equal(t, expected, classify1(r, su))
}

// a game track (retail, classic, etc) was found in the release name.
func Test_classify1__game_track_from_release(t *testing.T) {
	r := GithubRelease{
		Name: "ClassicAddon-v1.2.3",
	}
	su := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	expected := SourceUpdate{
		AssetName:      "Addon-v1.2.3.zip",
		Version:        "Addon-v1.2.3",
		DownloadURL:    "https://example.org/foo/bar.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
	}
	assert.Equal(t, expected, classify1(r, su))
}

// a game track in the asset name wins over a different one in the release name.
func Test_classify1__asset_beats_release(t *testing.T) {
	r := GithubRelease{Name: "1.2.3-classic"}
	su := SourceUpdate{
		AssetName:      "Addon-1.2.3-wrath.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet[GameTrackID](),
	}
	expected := SourceUpdate{
		AssetName:      "Addon-1.2.3-wrath.zip",
		PublishedDate:  dummy_dt,
		GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC_WOTLK),
	}
	assert.Equal(t, expected, classify1(r, su))
}

func Test_classify2__empty(t *testing.T) {
	sul := []SourceUpdate{}
	expected := []SourceUpdate{}
	assert.Equal(t, expected, classify2(sul))
}

// classify2 works on single unclassified assets.
// if there is more than one unclassified, return
func Test_classify2__too_many_unclassified(t *testing.T) {
	all_except_retail_classic := gametrack_set()
	all_except_retail_classic.Remove(GAMETRACK_RETAIL)
	all_except_retail_classic.Remove(GAMETRACK_CLASSIC)

	sul := []SourceUpdate{
		{GameTrackIDSet: all_except_retail_classic},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: all_except_retail_classic},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	assert.Equal(t, expected, classify2(sul))
}

// classify2 has nothing to do when every asset is already classified.
func Test_classify2__none_unclassified(t *testing.T) {
	sul := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
	}
	assert.Equal(t, expected, classify2(sul))
}

// classify2 works on single unclassified assets.
// if there all are classified, return
func Test_classify2__too_many_all_classified(t *testing.T) {
	sul := []SourceUpdate{
		{GameTrackIDSet: gametrack_set()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: gametrack_set()},
	}
	assert.Equal(t, expected, classify2(sul))
}

// a single asset that could not be classified stays unclassified: there is nothing to
// infer a game track from.
func Test_classify2__lone_unclassified(t *testing.T) {
	sul := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	assert.Equal(t, expected, classify2(sul))
}

// one unclassified asset alongside a single classic asset.
// several game tracks are unaccounted for, so no game track can be inferred and retail is
// not assumed.
func Test_classify2__several_game_tracks_left(t *testing.T) {
	sul := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	assert.Equal(t, expected, classify2(sul))
}

// several game tracks are unaccounted for when retail is already classified, so the
// asset is left unclassified.
func Test_classify2__retail_already_classified(t *testing.T) {
	sul := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	assert.Equal(t, expected, classify2(sul))
}

// exactly one game track left, and it is not retail.
func Test_classify2__one_game_track_left(t *testing.T) {
	all_except_tbc := gametrack_set()
	all_except_tbc.Remove(GAMETRACK_CLASSIC_TBC)

	sul := []SourceUpdate{
		{GameTrackIDSet: all_except_tbc},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: all_except_tbc},
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC_TBC)},
	}
	assert.Equal(t, expected, classify2(sul))
}

// if one is unclassified, classify it as the missing one
func Test_classify2_using_2(t *testing.T) {
	all_except_retail := gametrack_set()
	all_except_retail.Remove(GAMETRACK_RETAIL)

	sul := []SourceUpdate{
		{GameTrackIDSet: all_except_retail},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: all_except_retail},
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
	}
	assert.Equal(t, expected, classify2(sul))
}

// if one is unclassified, classify it as the missing one,
// slightly more complex
func Test_classify2_using_3(t *testing.T) {
	all_except_retail_classic := gametrack_set()
	all_except_retail_classic.Remove(GAMETRACK_RETAIL)
	all_except_retail_classic.Remove(GAMETRACK_CLASSIC)

	sul := []SourceUpdate{
		{GameTrackIDSet: all_except_retail_classic},
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)},
		{GameTrackIDSet: mapset.NewSet[GameTrackID]()},
	}
	expected := []SourceUpdate{
		{GameTrackIDSet: all_except_retail_classic},
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)},
		{GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL)},
	}
	assert.Equal(t, expected, classify2(sul))
}

// use release.json data to classify assets.
// typically done later in the process as the http call to github is 'expensive',
// especially if everything is already classified.
func Test_classify3(t *testing.T) {
	rj := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename: "Addon-v1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: RELEASE_JSON_FLAVOR_MAINLINE},
					{Flavor: RELEASE_JSON_FLAVOR_CLASSIC},
				},
			},
		},
	}
	sul := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3.zip",
			GameTrackIDSet: mapset.NewSet[GameTrackID](),
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC),
		},
	}
	assert.Equal(t, expected, classify3(sul, rj))
}

// the author's release.json wins over a game track guessed from the asset name.
// unlike the earlier passes, classify3 replaces rather than fills in.
func Test_classify3__overrides_a_guess(t *testing.T) {
	rj := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename: "Addon-v1.2.3-classic.zip",
				MetadataList: []ReleaseJSONMetadata{
					{Flavor: RELEASE_JSON_FLAVOR_WRATH},
				},
			},
		},
	}
	sul := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC_WOTLK),
		},
	}
	assert.Equal(t, expected, classify3(sul, rj))
}

// an asset the release.json says nothing about keeps the game track it was given.
func Test_classify3__missing_asset(t *testing.T) {
	rj := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename:     "SomeOtherAddon-v1.2.3.zip",
				MetadataList: []ReleaseJSONMetadata{{Flavor: RELEASE_JSON_FLAVOR_MAINLINE}},
			},
		},
	}
	sul := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	assert.Equal(t, expected, classify3(sul, rj))
}

// a release.json declaring only a flavor `GuessGameTrack` does not know leaves the earlier
// guess in place.
func Test_classify3__unknown_flavor_keeps_the_guess(t *testing.T) {
	rj := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename:     "Addon-v1.2.3-classic.zip",
				MetadataList: []ReleaseJSONMetadata{{Flavor: "mists"}},
			},
		},
	}
	sul := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	assert.Equal(t, expected, classify3(sul, rj))
}

func Test_published_release_list(t *testing.T) {
	given := []GithubRelease{
		{Name: "1.2.5", Draft: true},
		{Name: "1.2.4"},
		{Name: "1.2.3", PreRelease: true},
		{Name: "1.2.2", Draft: true, PreRelease: true},
		{Name: "1.2.1"},
	}
	expected := []GithubRelease{
		{Name: "1.2.4"},
		{Name: "1.2.1"},
	}
	assert.Equal(t, expected, published_release_list(given))
	assert.Equal(t, []GithubRelease{}, published_release_list(nil))
}

// a release.json entry with a recognised and an unrecognised flavor replaces the guess with
// the recognised game track only.
func Test_classify3__partly_recognised_flavors(t *testing.T) {
	rj := ReleaseJSON{
		ReleaseList: []ReleaseJSONRelease{
			{
				Filename:     "Addon-v1.2.3-classic.zip",
				MetadataList: []ReleaseJSONMetadata{{Flavor: RELEASE_JSON_FLAVOR_MAINLINE}, {Flavor: "mists"}},
			},
		},
	}
	sul := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC),
		},
	}
	expected := []SourceUpdate{
		{
			AssetName:      "Addon-v1.2.3-classic.zip",
			GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL),
		},
	}
	assert.Equal(t, expected, classify3(sul, rj))
}

// --- process_github_release_list

const dummy_release_json_url = "https://example.org/foo/release.json"

// a zip asset, fully uploaded, as Github describes it.
func dummy_asset(name string) GithubReleaseAsset {
	return GithubReleaseAsset{
		Name:               name,
		State:              GITHUB_RELEASE_ASSET_STATE_UPLOADED,
		ContentType:        "application/zip",
		BrowserDownloadURL: "https://example.org/foo/" + name,
	}
}

// the release.json asset as Github describes it: not a zip, so it is never an update.
func dummy_release_json_asset() GithubReleaseAsset {
	return GithubReleaseAsset{
		Name:               "release.json",
		State:              GITHUB_RELEASE_ASSET_STATE_UPLOADED,
		ContentType:        "application/json",
		BrowserDownloadURL: dummy_release_json_url,
	}
}

// returns an app whose downloader answers `body_map`, and the downloader itself for
// asserting on what was requested.
func dummy_app_with_responses(body_map map[string][]byte) (*core.App, *core.MapDownloader) {
	downloader := core.MakeMapDownloaderBytes(body_map)
	app := core.NewApp()
	app.Downloader = downloader
	return app, downloader
}

// returns the game tracks of each update, keyed by asset name, for terser assertions.
func sul_game_tracks(sul []SourceUpdate) map[string][]GameTrackID {
	m := map[string][]GameTrackID{}
	for _, su := range sul {
		gtl := su.GameTrackIDSet.ToSlice()
		slices.Sort(gtl)
		m[su.AssetName] = gtl
	}
	return m
}

func Test_process_github_release_list__empty(t *testing.T) {
	app, _ := dummy_app_with_responses(nil)
	expected := []SourceUpdate{}
	assert.Equal(t, expected, process_github_release_list(app, []GithubRelease{}))
}

// assets named for a game track are classified from their name alone.
// no release.json means no extra HTTP request.
func Test_process_github_release_list__classified_by_asset_name(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			TagName:       "v1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_asset("Addon-1.2.3-classic.zip"),
				dummy_asset("Addon-1.2.3-bcc.zip"),
			},
		},
	}
	app, downloader := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
		"Addon-1.2.3-bcc.zip":     {GAMETRACK_CLASSIC_TBC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
	assert.Equal(t, []string{}, downloader.RequestedURLList)
}

// drafts and prereleases contribute no updates.
func Test_process_github_release_list__skips_drafts_and_prereleases(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.4",
			PublishedDate: dummy_dt,
			Draft:         true,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.4-classic.zip")},
		},
		{
			Name:          "1.2.4-beta",
			PublishedDate: dummy_dt,
			PreRelease:    true,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.4-beta-classic.zip")},
		},
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.3-classic.zip")},
		},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
}

// assets that are not zips, or not fully uploaded, are not updates.
func Test_process_github_release_list__filters_assets(t *testing.T) {
	still_uploading := dummy_asset("Addon-1.2.3-bcc.zip")
	still_uploading.State = GITHUB_RELEASE_ASSET_STATE_OPEN

	not_a_zip := dummy_asset("Addon-1.2.3-wrath.txt")
	not_a_zip.ContentType = "text/plain"

	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_asset("Addon-1.2.3-classic.zip"),
				still_uploading,
				not_a_zip,
				dummy_release_json_asset(),
			},
		},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
}

// the release.json of the newest release is downloaded and its game tracks used.
func Test_process_github_release_list__uses_release_json(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.3.zip"),
			},
		},
	}
	body := []byte(`{"releases": [
      {"filename": "Addon-1.2.3.zip", "metadata": [
        {"flavor": "mainline", "interface": 100105},
        {"flavor": "classic", "interface": 11403}]}]}`)
	app, downloader := dummy_app_with_responses(map[string][]byte{dummy_release_json_url: body})
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3.zip": {GAMETRACK_CLASSIC, GAMETRACK_RETAIL},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
	assert.Equal(t, []string{dummy_release_json_url}, downloader.RequestedURLList)
}

// only the newest published release's release.json is consulted, so when it has none, an
// older release's release.json is not downloaded instead.
func Test_process_github_release_list__newest_without_release_json(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.4",
			PublishedDate: dummy_dt,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.4-classic.zip")},
		},
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.3-classic.zip"),
			},
		},
	}
	body := []byte(`{"releases": [
      {"filename": "Addon-1.2.3-classic.zip", "metadata": [{"flavor": "wrath", "interface": 30403}]}]}`)
	app, downloader := dummy_app_with_responses(map[string][]byte{dummy_release_json_url: body})
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.4-classic.zip": {GAMETRACK_CLASSIC},
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
	assert.Equal(t, []string{}, downloader.RequestedURLList)
}

// a malformed release.json leaves the earlier passes' game tracks in place, and the other
// releases are still classified.
func Test_process_github_release_list__release_json_malformed(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.4",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.4-classic.zip"),
			},
		},
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.3-retail.zip")},
		},
	}
	body := []byte(`{"releases": [`)
	app, downloader := dummy_app_with_responses(map[string][]byte{dummy_release_json_url: body})
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.4-classic.zip": {GAMETRACK_CLASSIC},
		"Addon-1.2.3-retail.zip":  {GAMETRACK_RETAIL},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
	assert.Equal(t, []string{dummy_release_json_url}, downloader.RequestedURLList)
}

// only the newest release's release.json is downloaded, to keep this to one extra request
// rather than one per release.
// older releases fall back to whatever the earlier passes guessed.
func Test_process_github_release_list__release_json_newest_release_only(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.4",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.4.zip"),
			},
		},
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.3-classic.zip"),
			},
		},
	}
	body := []byte(`{"releases": [
      {"filename": "Addon-1.2.4.zip", "metadata": [{"flavor": "wrath", "interface": 30403}]}]}`)
	app, downloader := dummy_app_with_responses(map[string][]byte{dummy_release_json_url: body})
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.4.zip":         {GAMETRACK_CLASSIC_WOTLK},
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
	assert.Equal(t, []string{dummy_release_json_url}, downloader.RequestedURLList)
}

// a release.json that fails to download leaves the guessed game tracks alone.
func Test_process_github_release_list__release_json_download_fails(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_release_json_asset(),
				dummy_asset("Addon-1.2.3-classic.zip"),
			},
		},
	}
	// no response configured for the release.json url, so the download errors.
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3-classic.zip": {GAMETRACK_CLASSIC},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
}

// an asset published before WoW Classic existed is retail.
func Test_process_github_release_list__pre_classic_is_retail(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt_pre_classic,
			AssetList:     []GithubReleaseAsset{dummy_asset("Addon-1.2.3.zip")},
		},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := map[string][]GameTrackID{
		"Addon-1.2.3.zip": {GAMETRACK_RETAIL},
	}
	assert.Equal(t, expected, sul_game_tracks(actual))
}

// an asset that survives every pass unclassified is excluded rather than assumed to be
// retail.
func Test_process_github_release_list__unclassified_excluded(t *testing.T) {
	var cases = []struct {
		given    []GithubReleaseAsset
		expected map[string][]GameTrackID
	}{
		// a single zip with nothing to go on
		{
			[]GithubReleaseAsset{dummy_asset("Addon-1.2.3.zip")},
			map[string][]GameTrackID{},
		},
		// an unsupported game track beside a known one
		{
			[]GithubReleaseAsset{dummy_asset("Addon-classic.zip"), dummy_asset("Addon-mists.zip")},
			map[string][]GameTrackID{"Addon-classic.zip": {GAMETRACK_CLASSIC}},
		},
	}
	for _, c := range cases {
		release_list := []GithubRelease{{Name: "1.2.3", PublishedDate: dummy_dt, AssetList: c.given}}
		app, _ := dummy_app_with_responses(nil)
		actual := process_github_release_list(app, release_list)
		assert.Equal(t, c.expected, sul_game_tracks(actual))
	}
}

// a draft or prerelease at the head of the release list does not stop the release.json of
// the newest published release being downloaded.
func Test_process_github_release_list__unpublished_first_uses_release_json(t *testing.T) {
	body := []byte(`{"releases": [
      {"filename": "Addon-1.2.3.zip", "metadata": [{"flavor": "wrath", "interface": 30403}]}]}`)

	for _, unpublished := range []GithubRelease{
		{Name: "1.2.4", PublishedDate: dummy_dt, Draft: true, AssetList: []GithubReleaseAsset{dummy_asset("Addon-1.2.4.zip")}},
		{Name: "1.2.4", PublishedDate: dummy_dt, PreRelease: true, AssetList: []GithubReleaseAsset{dummy_asset("Addon-1.2.4.zip")}},
	} {
		release_list := []GithubRelease{
			unpublished,
			{
				Name:          "1.2.3",
				PublishedDate: dummy_dt,
				AssetList: []GithubReleaseAsset{
					dummy_release_json_asset(),
					dummy_asset("Addon-1.2.3.zip"),
				},
			},
		}
		app, downloader := dummy_app_with_responses(map[string][]byte{dummy_release_json_url: body})
		actual := process_github_release_list(app, release_list)

		expected := map[string][]GameTrackID{
			"Addon-1.2.3.zip": {GAMETRACK_CLASSIC_WOTLK},
		}
		assert.Equal(t, expected, sul_game_tracks(actual))
		assert.Equal(t, []string{dummy_release_json_url}, downloader.RequestedURLList)
	}
}

// updates are returned newest release first, which `_make_addon__pick_source_update`
// depends on.
func Test_process_github_release_list__preserves_release_order(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.4",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_asset("Addon-1.2.4-retail.zip"),
				dummy_asset("Addon-1.2.4-classic.zip"),
			},
		},
		{
			Name:          "1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_asset("Addon-1.2.3-classic.zip"),
				dummy_asset("Addon-1.2.3-retail.zip"),
			},
		},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	expected := []string{
		"Addon-1.2.4-retail.zip",
		"Addon-1.2.4-classic.zip",
		"Addon-1.2.3-classic.zip",
		"Addon-1.2.3-retail.zip",
	}
	actual_names := []string{}
	for _, su := range actual {
		actual_names = append(actual_names, su.AssetName)
	}
	assert.Equal(t, expected, actual_names)
}

// the version of every update in a release is the release name, not the asset name.
func Test_process_github_release_list__version_from_release(t *testing.T) {
	release_list := []GithubRelease{
		{
			Name:          "1.2.3",
			TagName:       "v1.2.3",
			PublishedDate: dummy_dt,
			AssetList: []GithubReleaseAsset{
				dummy_asset("Addon-1.2.3-classic.zip"),
				dummy_asset("Addon-1.2.3-bcc.zip"),
			},
		},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := process_github_release_list(app, release_list)

	assert.Len(t, actual, 2)
	for _, su := range actual {
		assert.Equal(t, "1.2.3", su.Version)
		assert.Equal(t, dummy_dt, su.PublishedDate)
	}
}

// returns the log output of `fn`, captured at every level including DEBUG.
func capture_log(fn func()) string {
	var log_output bytes.Buffer
	original_logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log_output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(original_logger)
	fn()
	return log_output.String()
}

// an excluded asset is logged at DEBUG, never WARN or ERROR.
func Test_process_github_release_list__exclusion_logged_at_debug(t *testing.T) {
	release_list := []GithubRelease{
		{Name: "1.2.3", PublishedDate: dummy_dt, AssetList: []GithubReleaseAsset{dummy_asset("Addon-1.2.3.zip")}},
	}
	app, _ := dummy_app_with_responses(nil)
	actual := capture_log(func() { process_github_release_list(app, release_list) })

	assert.Contains(t, actual, "level=DEBUG")
	assert.Contains(t, actual, "Addon-1.2.3.zip")
	assert.NotContains(t, actual, "level=WARN")
	assert.NotContains(t, actual, "level=ERROR")
}

// a failed release.json download is logged at WARN; a malformed release.json, which is the
// addon author's data, at DEBUG.
func Test_classify_using_release_json__log_levels(t *testing.T) {
	sul := []SourceUpdate{{AssetName: "Addon-1.2.3-classic.zip", GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)}}

	app, _ := dummy_app_with_responses(nil)
	actual := capture_log(func() { classify_using_release_json(app, dummy_release_json_url, sul) })
	assert.Contains(t, actual, "level=WARN")

	app, _ = dummy_app_with_responses(map[string][]byte{dummy_release_json_url: []byte(`{"releases": [`)})
	actual = capture_log(func() { classify_using_release_json(app, dummy_release_json_url, sul) })
	assert.Contains(t, actual, "level=DEBUG")
	assert.NotContains(t, actual, "level=WARN")
	assert.NotContains(t, actual, "level=ERROR")
}

// a random Github release list and the release.json body served for any release.
type random_github_input struct {
	release_list      []GithubRelease
	release_json_body []byte // nil when no release.json is served
}

// asset name fragments: game track hints, unsupported game tracks and noise.
var random_asset_fragment_list = []string{
	"", "-classic", "-retail", "-mainline", "-standard", "-cata", "-wrath", "-bcc",
	"-mists", "-forever", "-Catalyst", "-no-lib",
}

var random_flavor_list = []string{"mainline", "classic", "bcc", "wrath", "cata", "mists", "forever", ""}

// returns a value of `rand_list`, picked by `r`.
func random_pick[T any](r *rand.Rand, rand_list []T) T {
	return rand_list[r.Intn(len(rand_list))]
}

func (random_github_input) Generate(r *rand.Rand, size int) reflect.Value {
	input := random_github_input{}
	asset_name_list := []string{}
	for i := range r.Intn(5) {
		release := GithubRelease{
			Name:          random_pick(r, []string{fmt.Sprintf("1.2.%d", i), fmt.Sprintf("1.2.%d-classic", i), ""}),
			PublishedDate: random_pick(r, []time.Time{dummy_dt, dummy_dt_pre_classic}),
			Draft:         r.Intn(4) == 0,
			PreRelease:    r.Intn(4) == 0,
		}
		if r.Intn(2) == 0 {
			release.AssetList = append(release.AssetList, dummy_release_json_asset())
		}
		for j := range r.Intn(4) {
			name := fmt.Sprintf("Addon-%d-%d%s.zip", i, j, random_pick(r, random_asset_fragment_list))
			release.AssetList = append(release.AssetList, dummy_asset(name))
			asset_name_list = append(asset_name_list, name)
		}
		input.release_list = append(input.release_list, release)
	}

	switch r.Intn(3) {
	case 0:
		// no release.json served
	case 1:
		entry_list := []string{}
		for _, name := range asset_name_list {
			entry_list = append(entry_list, fmt.Sprintf(`{"filename": %q, "metadata": [{"flavor": %q}]}`, name, random_pick(r, random_flavor_list)))
		}
		input.release_json_body = []byte(`{"releases": [` + strings.Join(entry_list, ",") + `]}`)
	case 2:
		input.release_json_body = []byte(`{"releases": [`)
	}
	return reflect.ValueOf(input)
}

// property: every update returned has at least one game track, every game track is
// supported, and at most one release.json is requested.
func Test_process_github_release_list__property(t *testing.T) {
	classified := func(input random_github_input) bool {
		body_map := map[string][]byte{}
		if input.release_json_body != nil {
			body_map[dummy_release_json_url] = input.release_json_body
		}
		app, downloader := dummy_app_with_responses(body_map)
		for _, su := range process_github_release_list(app, input.release_list) {
			if su.GameTrackIDSet.IsEmpty() {
				return false
			}
			for game_track := range su.GameTrackIDSet.Iter() {
				if !SUPPORTED_GAME_TRACKS.Contains(game_track) {
					return false
				}
			}
		}
		return len(downloader.RequestedURLList) <= 1
	}
	assert.Nil(t, quick.Check(classified, &quick.Config{MaxCount: 2000}))
}
