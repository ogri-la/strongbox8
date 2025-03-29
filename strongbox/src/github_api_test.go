package strongbox

import (
	"testing"
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

func Test_classify2__empty(t *testing.T) {
	sul := []SourceUpdate{}
	expected := []SourceUpdate{}
	assert.Equal(t, expected, classify2(sul))
}

// classify2 works on single unclassified assets.
// if there is more than one unclassified, return
func Test_classify2__too_many_unclassified(t *testing.T) {
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
