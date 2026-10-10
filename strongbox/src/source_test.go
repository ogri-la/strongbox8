package strongbox

// addon hosts: URL parsing, finding addons and listing updates.
// clj: `github_api_test.clj`, `gitlab_api_test.clj`, `wowinterface_api_test.clj`,
// `catalogue_test.clj/parse-user-string-router*`

import (
	"bw/core"
	"bw/http_utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
)

// returns an app whose HTTP requests are answered by `routes`, and the transport.
func host_app(t *testing.T, routes map[string]http_utils.Fixture) (*core.App, *http_utils.FixtureTransport) {
	t.Helper()
	app := core.NewApp()
	ft := http_utils.NewFixtureTransport(routes)
	app.HTTPClient.Transport = ft
	return app, ft
}

func fixture(name string) http_utils.Fixture {
	return http_utils.Fixture{Body: test_fixture_bytes(name)}
}

func json_fixture(t *testing.T, v any) http_utils.Fixture {
	t.Helper()
	b, err := json.Marshal(v)
	assert.NoError(t, err)
	return http_utils.Fixture{Body: b}
}

// --- URL parsing

// clj: `github_api_test.clj/parse-user-string`, `--empty-cases`
func Test_GithubAPI_ParseURL(t *testing.T) {
	expected := "Aviana/HealComm"
	for _, given := range []string{
		"https://github.com/Aviana/HealComm",
		"https://github.com/Aviana/HealComm/",
		"http://github.com/Aviana/HealComm",
		"github.com/Aviana/HealComm",
		"https://user:pass@github.com/Aviana/HealComm",
		"https://github.com/Aviana/HealComm?foo=bar",
		"https://github.com/Aviana/HealComm#anchor",
		"https://github.com/Aviana/HealComm/releases/tag/2.04",
		"https://www.github.com/Aviana/HealComm",
	} {
		actual, ok := (&GithubAPI{}).ParseURL(given)
		assert.True(t, ok, given)
		assert.Equal(t, expected, actual, given)
	}
	for _, given := range []string{"", "https://github.com", "https://github.com/Aviana", "https://gitlab.com/a/b"} {
		_, ok := (&GithubAPI{}).ParseURL(given)
		assert.False(t, ok, given)
	}
}

// clj: `gitlab_api_test.clj/parse-user-string`, `--with-group`, `--bad-cases`
func Test_GitlabAPI_ParseURL(t *testing.T) {
	for _, given := range []string{
		"https://gitlab.com/woblight/nitro",
		"https://gitlab.com/woblight/nitro/",
		"http://gitlab.com/woblight/nitro",
		"https://www.gitlab.com/woblight/nitro",
		"https://gitlab.com/woblight/nitro/-/releases",
		"https://user:pass@gitlab.com/woblight/nitro?foo=bar#baz",
	} {
		actual, ok := (&GitlabAPI{}).ParseURL(given)
		assert.True(t, ok, given)
		assert.Equal(t, "woblight/nitro", actual, given)
	}
	actual, ok := (&GitlabAPI{}).ParseURL("https://www.gitlab.com/thing-engineering/wowthing/wowthing-sync/-/releases")
	assert.True(t, ok)
	assert.Equal(t, "thing-engineering/wowthing/wowthing-sync", actual)

	actual, ok = (&GitlabAPI{}).ParseURL("gitlab.com/group/subgroup/project")
	assert.True(t, ok)
	assert.Equal(t, "group/subgroup/project", actual)

	for _, given := range []string{"https://gitlab.com/", "https://gitlab.com/foo"} {
		_, ok := (&GitlabAPI{}).ParseURL(given)
		assert.False(t, ok, given)
	}
}

// clj: `gitlab_api_test.clj/api-url`
func Test_gitlab_api_url(t *testing.T) {
	assert.Equal(t, "https://gitlab.com/api/v4/projects/foo%2Fbar", gitlab_api_url("foo/bar"))
	assert.Equal(t, "https://gitlab.com/api/v4/projects/foo%2Fbar%2Fbaz%21", gitlab_api_url("foo/bar/baz!"))
}

// clj: `wowinterface_api_test.clj/parse-user-string`
func Test_WowinterfaceAPI_ParseURL(t *testing.T) {
	for _, given := range []string{
		"https://www.wowinterface.com/downloads/info8882",
		"https://www.wowinterface.com/downloads/info8882-BetterBags.html",
		"https://wowinterface.com/downloads/download8882-BetterBags",
		"www.wowinterface.com/downloads/info8882-Some-Very-Long-Name-That-Goes-On.html",
	} {
		actual, ok := (&WowinterfaceAPI{}).ParseURL(given)
		assert.True(t, ok, given)
		assert.Equal(t, "8882", actual, given)
	}
	for _, given := range []string{
		"https://www.wowinterface.com/downloads/dlfile8882-BetterBags.html",
		"https://www.wowinterface.com/forums/",
		"https://example.org/downloads/info8882",
	} {
		_, ok := (&WowinterfaceAPI{}).ParseURL(given)
		assert.False(t, ok, given)
	}
}

// clj: `catalogue_test.clj/parse-user-string-router`, `--bad-cases`
func Test_ParseAddonURL(t *testing.T) {
	cases := []struct {
		given     string
		source    Source
		source_id string
	}{
		{"https://github.com/Aviana/HealComm", SOURCE_GITHUB, "Aviana/HealComm"},
		{"//github.com/Aviana/HealComm/releases", SOURCE_GITHUB, "Aviana/HealComm"},
		{"https://gitlab.com/woblight/nitro", SOURCE_GITLAB, "woblight/nitro"},
		{"https://www.wowinterface.com/downloads/info8882-Name.html", SOURCE_WOWI, "8882"},
	}
	for _, c := range cases {
		source, source_id, err := ParseAddonURL(c.given)
		assert.NoError(t, err, c.given)
		assert.Equal(t, c.source, source, c.given)
		assert.Equal(t, c.source_id, source_id, c.given)
	}
	for _, given := range []string{"", "   ", "foo", "123", "https://", "https://foo.com", "https://www.curseforge.com/wow/addons/x", "https://www.tukui.org/addons.php?id=1"} {
		_, _, err := ParseAddonURL(given)
		assert.Error(t, err, given)
	}
	_, _, err := ParseAddonURL("https://www.curseforge.com/wow/addons/x")
	assert.ErrorIs(t, err, ErrUnsupportedSource)
}

// --- github

// clj: `github_api_test.clj/find-addon--gametracks-release-list`
func Test_GithubAPI_FindAddon__release_assets(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("aviana/healcomm"): fixture("v7/github-repo-releases--aviana-healcomm.json"),
	})
	actual, err := (&GithubAPI{}).FindAddon(app, "aviana/healcomm")
	assert.NoError(t, err)
	assert.Equal(t, FlexString("Aviana/HealComm"), actual.SourceID, "the case is corrected")
	assert.Equal(t, "https://github.com/Aviana/HealComm", actual.URL)
	assert.Equal(t, "HealComm", actual.Label)
	assert.Equal(t, "healcomm", actual.Name)
	assert.NotEmpty(t, actual.GameTrackIDList)
	assert.Greater(t, actual.DownloadCount, 0)
}

// clj: `github_api_test.clj/find-addon--gametracks-release-json`
func Test_GithubAPI_FindAddon__release_json(t *testing.T) {
	releases := []map[string]any{{
		"name": "1.0", "tag_name": "1.0", "html_url": "https://github.com/robert388/Necrosis/releases/tag/1.0",
		"published_at": "2021-06-01T00:00:00Z",
		"assets": []map[string]any{
			{"name": "Necrosis.zip", "state": "uploaded", "content_type": "application/zip", "download_count": 10888,
				"browser_download_url": "https://github.com/robert388/Necrosis/releases/download/Necrosis.zip"},
			{"name": "release.json", "state": "uploaded", "content_type": "application/json",
				"browser_download_url": "https://github.com/robert388/Necrosis/releases/download/release.json"},
		},
	}}
	release_json := map[string]any{"releases": []map[string]any{{"filename": "Necrosis-other.zip", "nolib": false,
		"metadata": []map[string]any{{"flavor": "classic"}, {"flavor": "bcc"}, {"flavor": "mainline"}}}}}
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("robert388/Necrosis"):                          json_fixture(t, releases),
		"https://github.com/robert388/Necrosis/releases/download/release.json": json_fixture(t, release_json),
	})
	actual, err := (&GithubAPI{}).FindAddon(app, "robert388/Necrosis")
	assert.NoError(t, err)
	assert.Equal(t, []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC}, actual.GameTrackIDList)
	assert.Equal(t, 10888, actual.DownloadCount)
}

// clj: `github_api_test.clj/find-addon--gametracks-toc-data`
func Test_GithubAPI_FindAddon__toc_data(t *testing.T) {
	contents := []map[string]any{{"name": "Addon.toc", "type": "file", "download_url": "https://raw.githubusercontent.com/Aviana/HealComm/master/Addon.toc"}}
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("Aviana/HealComm"):                           fixture("v7/github-repo-releases--no-game-tracks.json"),
		"https://api.github.com/repos/Aviana/HealComm/contents":              json_fixture(t, contents),
		"https://raw.githubusercontent.com/Aviana/HealComm/master/Addon.toc": {Body: []byte("## Interface: 20501\n## Title: HealComm\n")},
	})
	actual, err := (&GithubAPI{}).FindAddon(app, "Aviana/HealComm")
	assert.NoError(t, err)
	assert.Equal(t, []GameTrackID{GAMETRACK_CLASSIC_TBC}, actual.GameTrackIDList)
}

// no game tracks anywhere: the addon is not found rather than assumed retail.
func Test_GithubAPI_FindAddon__no_game_tracks(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("Aviana/HealComm"):              fixture("v7/github-repo-releases--no-game-tracks.json"),
		"https://api.github.com/repos/Aviana/HealComm/contents": {Body: []byte("[]")},
	})
	_, err := (&GithubAPI{}).FindAddon(app, "Aviana/HealComm")
	assert.ErrorIs(t, err, ErrNotFound)
}

// clj: `github_api_test.clj/find-addon--no-assets`
func Test_GithubAPI_FindAddon__no_assets(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): fixture("v7/github-repo-releases--no-assets.json"),
	})
	_, err := (&GithubAPI{}).FindAddon(app, "a/b")
	assert.ErrorIs(t, err, ErrNotFound)
}

// clj: `github_api_test.clj/rate-limit-exceeded`
func Test_GithubAPI__rate_limited(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("a/b"): {Status: 403, Header: http.Header{"X-Ratelimit-Remaining": {"0"}}},
	})
	_, err := (&GithubAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "a/b"})
	assert.ErrorContains(t, err, "request quota")
	_, err = (&GithubAPI{}).FindAddon(app, "a/b")
	assert.ErrorContains(t, err, "request quota")
}

func Test_GithubAPI__token(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret")
	assert.Equal(t, "Bearer secret", github_headers()["Authorization"])
	t.Setenv("GITHUB_TOKEN", "")
	assert.NotContains(t, github_headers(), "Authorization")
}

// a single unhinted zip takes the addon's known game tracks, several do not.
func Test_GithubAPI_ExpandSummary__known_game_tracks(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		github_release_list_url("Aviana/HealComm"): fixture("v7/github-repo-releases--no-game-tracks.json"),
	})
	actual, err := (&GithubAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "Aviana/HealComm"})
	assert.NoError(t, err)
	assert.Empty(t, actual, "no known game tracks, no updates")

	known := mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC)
	actual, err = (&GithubAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "Aviana/HealComm", KnownGameTracks: known})
	assert.NoError(t, err)
	assert.NotEmpty(t, actual)
	for _, su := range actual {
		assert.Equal(t, known, su.GameTrackIDSet)
	}

	// an addon not in the catalogue takes the game track it was installed for
	a := Addon{Source: SOURCE_GITHUB, SourceID: "Aviana/HealComm", NFO: &NFO{InstalledGameTrackID: GAMETRACK_CLASSIC_TBC}}
	actual, err = (&GithubAPI{}).ExpandSummary(app, expand_request(a))
	assert.NoError(t, err)
	assert.NotEmpty(t, actual)
	for _, su := range actual {
		assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_TBC), su.GameTrackIDSet)
	}
}

func Test_classify4(t *testing.T) {
	known := mapset.NewSet(GAMETRACK_RETAIL)
	one := []SourceUpdate{{AssetName: "a.zip", GameTrackIDSet: mapset.NewSet[GameTrackID]()}}
	assert.Equal(t, known, classify4(one, known)[0].GameTrackIDSet)

	two := []SourceUpdate{{AssetName: "a.zip", GameTrackIDSet: mapset.NewSet[GameTrackID]()}, {AssetName: "b.zip", GameTrackIDSet: mapset.NewSet[GameTrackID]()}}
	two = classify4(two, known)
	assert.True(t, two[0].GameTrackIDSet.IsEmpty())
	assert.True(t, two[1].GameTrackIDSet.IsEmpty())

	classified := []SourceUpdate{{AssetName: "a-classic.zip", GameTrackIDSet: mapset.NewSet(GAMETRACK_CLASSIC)}}
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC), classify4(classified, known)[0].GameTrackIDSet)

	none := []SourceUpdate{{AssetName: "a.zip", GameTrackIDSet: mapset.NewSet[GameTrackID]()}}
	assert.True(t, classify4(none, nil)[0].GameTrackIDSet.IsEmpty())
}

// clj: `github_api_test.clj/--odd-one-out--forever`
func Test_classify2__forever_never_inferred(t *testing.T) {
	sul := []SourceUpdate{{AssetName: "x.zip", GameTrackIDSet: mapset.NewSet[GameTrackID]()}}
	for _, gt := range []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_CLASSIC_WOTLK, GAMETRACK_CLASSIC_CATA} {
		sul = append(sul, SourceUpdate{GameTrackIDSet: mapset.NewSet(gt)})
	}
	actual := classify2(sul)
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_MISTS), actual[0].GameTrackIDSet)
}

// --- gitlab

func gitlab_routes(t *testing.T) map[string]http_utils.Fixture {
	return map[string]http_utils.Fixture{
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro":                 fixture("v7/gitlab-repo--woblight-nitro.json"),
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro/repository/tree": fixture("v7/gitlab-repo-tree--woblight-nitro.json"),
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro/releases":        fixture("v7/gitlab-repo-releases--woblight-nitro.json"),
	}
}

// clj: `gitlab_api_test.clj/find-addon--multi-toc`
func Test_GitlabAPI_FindAddon__multi_toc(t *testing.T) {
	routes := gitlab_routes(t)
	var tree []gitlab_tree_item
	json.Unmarshal(test_fixture_bytes("v7/gitlab-repo-tree--woblight-nitro.json"), &tree)
	for _, item := range tree {
		if item.Name == "Nitro.toc" {
			routes["https://gitlab.com/api/v4/projects/woblight%2Fnitro/repository/blobs/"+item.ID] = fixture("v7/gitlab-repo-blobs--woblight-nitro.json")
		}
	}
	app, ft := host_app(t, routes)
	actual, err := (&GitlabAPI{}).FindAddon(app, "woblight/nitro")
	assert.NoError(t, err, ft.Unrouted())
	assert.Equal(t, "https://gitlab.com/woblight/nitro", actual.URL)
	assert.Equal(t, FlexString("woblight/nitro"), actual.SourceID)
	assert.Equal(t, "Nitro", actual.Label)
	assert.Equal(t, "nitro", actual.Name)
	assert.Equal(t, 0, actual.DownloadCount)
	assert.Equal(t, []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC}, actual.GameTrackIDList)
}

// clj: `gitlab_api_test.clj/find-addon--single-toc`
func Test_GitlabAPI_FindAddon__single_toc(t *testing.T) {
	routes := gitlab_routes(t)
	tree := []map[string]any{{"id": "abc", "name": "Nitro.toc", "type": "blob", "path": "Nitro.toc"}}
	routes["https://gitlab.com/api/v4/projects/woblight%2Fnitro/repository/tree"] = json_fixture(t, tree)
	blob := map[string]any{"content": base64.StdEncoding.EncodeToString([]byte("## Interface: 90005\n## Title: Nitro\n"))}
	routes["https://gitlab.com/api/v4/projects/woblight%2Fnitro/repository/blobs/abc"] = json_fixture(t, blob)
	app, _ := host_app(t, routes)
	actual, err := (&GitlabAPI{}).FindAddon(app, "woblight/nitro")
	assert.NoError(t, err)
	assert.Equal(t, []GameTrackID{GAMETRACK_RETAIL}, actual.GameTrackIDList)
}

// clj: `gitlab_api_test.clj/expand-summary`
func Test_GitlabAPI_ExpandSummary(t *testing.T) {
	app, _ := host_app(t, gitlab_routes(t))
	known := mapset.NewSet(GAMETRACK_CLASSIC, GAMETRACK_CLASSIC_TBC, GAMETRACK_RETAIL)
	actual, err := (&GitlabAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "woblight/nitro", KnownGameTracks: known})
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Equal(t, "https://gitlab.com/woblight/nitro/-/releases/v1.0-0-gddcb65a/downloads/Nitro", actual[0].DownloadURL)
	assert.Equal(t, "v1.0-0-gddcb65a", actual[0].Version)
	assert.Equal(t, known, actual[0].GameTrackIDSet)
}

// clj: `gitlab_api_test.clj/expand-summary--http-error`
func Test_GitlabAPI_ExpandSummary__http_error(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro/releases": {Status: 504},
	})
	actual, err := (&GitlabAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "woblight/nitro"})
	assert.Error(t, err)
	assert.Empty(t, actual)
}

// clj: `gitlab_api_test.clj/expand-summary--strip-pre-release`, `--detect-classic`
func Test_GitlabAPI_ExpandSummary__upcoming_and_classic(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro/releases": fixture("v7/gitlab-repo-releases--upcoming-release-dummy.json"),
	})
	actual, _ := (&GitlabAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "woblight/nitro", KnownGameTracks: mapset.NewSet(GAMETRACK_RETAIL)})
	assert.Empty(t, actual, "upcoming releases are excluded")

	app, _ = host_app(t, map[string]http_utils.Fixture{
		"https://gitlab.com/api/v4/projects/woblight%2Fnitro/releases": fixture("v7/gitlab-repo-releases--classic-release-dummy.json"),
	})
	actual, _ = (&GitlabAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "woblight/nitro"})
	assert.Len(t, actual, 1)
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_TBC), actual[0].GameTrackIDSet)
}

// clj: `gitlab_api_test.clj/parse-release--*`
func Test_process_gitlab_release_list(t *testing.T) {
	release := func(name string, links ...gitlab_link) gitlab_release {
		r := gitlab_release{Name: name, TagName: "1.2.3"}
		r.ReleasedAt = dummy_dt
		r.Assets.Links = links
		return r
	}
	link := func(name string) gitlab_link {
		return gitlab_link{Name: name, DirectAssetURL: "https://example.org/" + name, LinkType: "package"}
	}
	app, _ := host_app(t, nil)

	assert.Empty(t, process_gitlab_release_list(app, []gitlab_release{release("1.2.3")}, nil), "no assets")
	assert.Empty(t, process_gitlab_release_list(app, []gitlab_release{release("1.2.3", link("Foo"))}, nil), "worst case")

	actual := process_gitlab_release_list(app, []gitlab_release{release("1.2.3", link("Foo-Classic"))}, nil)
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC), actual[0].GameTrackIDSet, "asset name")

	actual = process_gitlab_release_list(app, []gitlab_release{release("Foo 1.2.3-Classic-BCC", link("Foo"))}, nil)
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_TBC), actual[0].GameTrackIDSet, "release name")

	// a link naming its game track is classified, an unhinted sibling is not
	actual = process_gitlab_release_list(app, []gitlab_release{release("1.2", link("Nitro-1.2.zip"), link("Nitro-1.2-classic-bcc.zip"))}, nil)
	assert.Len(t, actual, 1)
	assert.Equal(t, "https://example.org/Nitro-1.2-classic-bcc.zip", actual[0].DownloadURL)
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_TBC), actual[0].GameTrackIDSet)

	external := link("Foo-Classic")
	external.External = true
	image := link("Foo-Retail")
	image.LinkType = "image"
	assert.Empty(t, process_gitlab_release_list(app, []gitlab_release{release("1.2.3", external, image)}, nil), "external links and images excluded")
}

// --- wowinterface

// clj: `wowinterface_api_test.clj/expand-summary`
func Test_WowinterfaceAPI_ExpandSummary(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{
		wowinterface_release_url("25079"): fixture("v7/wowinterface-api--addon-details.json"),
	})
	known := mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC)
	actual, err := (&WowinterfaceAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "25079", KnownGameTracks: known})
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	assert.Equal(t, "1.2.3", actual[0].Version)
	assert.Equal(t, "https://cdn.wowinterface.com/downloads/getfile.php?id=25079", actual[0].DownloadURL)
	assert.Equal(t, known, actual[0].GameTrackIDSet)
}

// no known game tracks: no updates rather than an assumed retail update.
func Test_WowinterfaceAPI_ExpandSummary__no_known_game_tracks(t *testing.T) {
	app, ft := host_app(t, nil)
	actual, err := (&WowinterfaceAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "25079"})
	assert.Error(t, err)
	assert.Empty(t, actual)
	assert.Empty(t, ft.Requested(), "nothing is requested")
}

// clj: `wowinterface_api_test.clj/download-addon-404`
func Test_WowinterfaceAPI_ExpandSummary__404(t *testing.T) {
	app, _ := host_app(t, map[string]http_utils.Fixture{wowinterface_release_url("1"): {Status: 404}})
	actual, err := (&WowinterfaceAPI{}).ExpandSummary(app, ExpandRequest{SourceID: "1", KnownGameTracks: mapset.NewSet(GAMETRACK_RETAIL)})
	assert.True(t, errors.Is(err, ErrNotFound))
	assert.Empty(t, actual, "no updates")
}

// a WoWInterface addon's updates support the game tracks its catalogue entry lists, so the
// addons dir's game track decides whether it has one.
// clj: `wowinterface_api_test.clj/expand-summary--*`
func Test_CheckForUpdates__wowinterface_catalogue_game_tracks(t *testing.T) {
	cases := []struct {
		name        string
		catalogued  []GameTrackID
		addons_dir  GameTrackID
		expected    string
		has_classic bool
	}{
		{"classic-only addon in a retail addons dir", []GameTrackID{GAMETRACK_CLASSIC}, GAMETRACK_RETAIL, "", false},
		{"retail and classic addon in a classic addons dir", []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC}, GAMETRACK_CLASSIC, "1.2.3", true},
	}
	for _, c := range cases {
		app, path, _ := app_with_installed(t, map[string]http_utils.Fixture{
			wowinterface_release_url("25079"): fixture("v7/wowinterface-api--addon-details.json"),
		}, test_addon_spec{DirList: []string{"EveryAddon"}, Source: SOURCE_WOWI, SourceID: "25079", Version: "1.0"})
		assert.NoError(t, SetAddonsDirGameTrack(app, path, c.addons_dir))
		ca := CatalogueAddon{URL: "https://www.wowinterface.com/downloads/info25079", Name: "everyaddon", Label: "EveryAddon",
			Source: SOURCE_WOWI, SourceID: "25079", GameTrackIDList: c.catalogued}
		catalogue_to_state(app, Catalogue{AddonSummaryList: []CatalogueAddon{ca}}, Catalogue{})
		assert.NoError(t, Reconcile(app))

		CheckForUpdates(app)
		a := addon_results(app)[0].Item.(Addon)
		assert.Equal(t, c.expected, a.AvailableVersion, c.name)
		if c.has_classic {
			assert.True(t, a.SourceUpdate.GameTrackIDSet.Contains(GAMETRACK_CLASSIC), c.name)
		}
	}
}

func Test_WowinterfaceAPI_FindAddon(t *testing.T) {
	app := test_app_with_settings(t, NewSettings())
	catalogue_to_state(app, Catalogue{AddonSummaryList: []CatalogueAddon{{Source: SOURCE_WOWI, SourceID: "8882", Label: "Found"}}}, Catalogue{})
	actual, err := (&WowinterfaceAPI{}).FindAddon(app, "8882")
	assert.NoError(t, err)
	assert.Equal(t, "Found", actual.Label)
	_, err = (&WowinterfaceAPI{}).FindAddon(app, "1")
	assert.ErrorIs(t, err, ErrNotFound)
}

// the addon's known game tracks: its catalogue entry's, else the installed game track.
func Test_known_game_tracks(t *testing.T) {
	assert.True(t, known_game_tracks(Addon{}).IsEmpty())
	a := Addon{NFO: &NFO{InstalledGameTrackID: GAMETRACK_CLASSIC_TBC}}
	assert.Equal(t, mapset.NewSet(GAMETRACK_CLASSIC_TBC), known_game_tracks(a))
	a.CatalogueAddon = &CatalogueAddon{GameTrackIDList: []GameTrackID{GAMETRACK_RETAIL, GAMETRACK_CLASSIC}}
	assert.Equal(t, mapset.NewSet(GAMETRACK_RETAIL, GAMETRACK_CLASSIC), known_game_tracks(a))
}
