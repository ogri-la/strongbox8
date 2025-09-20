package strongbox

import (
	"bw/core"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

type GithubAPI struct{}

var _ AddonSource = (*GithubAPI)(nil)

type GithubReleaseAssetState = string

var (
	GITHUB_RELEASE_ASSET_STATE_UPLOADED GithubReleaseAssetState = "uploaded"
	GITHUB_RELEASE_ASSET_STATE_OPEN     GithubReleaseAssetState = "open"
)

// a Github release has many assets.
type GithubReleaseAsset struct {
	Name               string                  `json:"name"`
	Label              string                  `json:"label"`
	State              GithubReleaseAssetState `json:"state"`
	BrowserDownloadURL string                  `json:"browser_download_url"`
	ContentType        string                  `json:"content_type"`
	CreatedDate        time.Time               `json:"created_at"`
	UpdatedDate        time.Time               `json:"updated_at"`
}

// a Github repository has many releases.
type GithubRelease struct {
	Name          string               `json:"name"`     // "1.2.3"
	TagName       string               `json:"tag_name"` // "v1.2.3"
	AssetList     []GithubReleaseAsset `json:"assets"`
	PublishedDate time.Time            `json:"published_at"`
	Draft         bool                 `json:"draft"`
	PreRelease    bool                 `json:"prerelease"`
}

// ---

// fetch the first page of releases for a Github repository
func github_release_list_url(source_id string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases?per-page=100&page=1", source_id)
}

// ---

func is_release_json(a GithubReleaseAsset) bool {
	return a.Name == "release.json" || strings.TrimSpace(strings.ToLower(a.Name)) == "release.json"
}

func is_supported_zip(content_type string) bool {
	return content_type == "application/zip" || content_type == "application/x-zip-compressed"
}

func is_fully_uploaded(state GithubReleaseAssetState) bool {
	return state == GITHUB_RELEASE_ASSET_STATE_UPLOADED
}

// returns the first non-empty value that can be used as a 'version' from a list of good candidates.
// `asset` is a subset of `release` that has been filtered out from the other assets in the release.
// ideally we want to use the name the author has specifically chosen for a release.
// if that doesn't exist, we fallback to the git tag which is typically better than the asset's name.
func pick_asset_version_name(release GithubRelease, asset GithubReleaseAsset) string {
	if release.Name != "" {
		return release.Name
	}
	if release.TagName != "" {
		return release.TagName
	}
	if asset.Name != "" {
		return asset.Name
	}
	return ""
}

// convert a `release` and a filtered `asset_list` to an initial `SourceUpdate` list.
// these updates are then further classified by the classify* functions
func to_sul(release GithubRelease, asset_list []GithubReleaseAsset) []SourceUpdate {
	sul := []SourceUpdate{}
	for _, a := range asset_list {
		su := NewSourceUpdate()
		su.AssetName = a.Name
		su.Version = pick_asset_version_name(release, a)
		su.DownloadURL = a.BrowserDownloadURL
		// individual assets haved created and updated dates,
		// but we're not interested in that fine level of detail.
		su.PublishedDate = release.PublishedDate
		su.GameTrackIDSet = mapset.NewSet[GameTrackID]()

		sul = append(sul, su)
	}
	return sul
}

// guess the asset game track from the asset name, the release name or the time it was published.
func classify1(r GithubRelease, su SourceUpdate) SourceUpdate {
	game_track_from_release := GuessGameTrack(r.Name)
	game_track_from_asset := GuessGameTrack(su.AssetName)

	if game_track_from_asset != "" {
		// game track present in asset file name, prefer that over any game-track in release name
		su.GameTrackIDSet.Add(game_track_from_asset)

	} else if IsBeforeClassic(su.PublishedDate) {
		// I imagine there were classic addons published prior to the release of WoW Classic.
		// If we can use the asset name, brilliant, if not, and it's before the cut off, then it's retail.
		su.GameTrackIDSet.Add(GAMETRACK_RETAIL)

	} else if game_track_from_release != "" {
		// game track present in release name, prefer that over `:game-track-list`
		su.GameTrackIDSet.Add(game_track_from_release)

	} else {
		// we don't know, we couldn't guess. leave empty so we can optionally deal with it later.
	}

	return su
}

// if we have a telltale single unclassified asset in a set of classified assets, use that.
func classify2(sul []SourceUpdate) []SourceUpdate {
	if len(sul) == 0 {
		return sul
	}

	num_unclassified := 0
	classified := mapset.NewSet[GameTrackID]()
	for _, su := range sul {
		if su.GameTrackIDSet.IsEmpty() {
			num_unclassified++
		} else {
			classified = classified.Union(su.GameTrackIDSet)
		}
	}
	diff := gametrack_set().Difference(classified) // #{:classic :classic-bc :retail} #{:classic :classic-bc} => #{:retail}

	if num_unclassified > 1 {
		// too many unclassified for this logic
		return sul
	}

	if diff.Cardinality() == 0 {
		// addon covers all game tracks!
		return sul
	}

	if diff.Cardinality() == 1 {
		// best case: 1 unclassified asset and exactly 1 available game track.
		// 2024-09-01: lots of addons still support the full set of classic versions,
		// but I've noticed many addons dropping tbc in favour of wotlk, then dropping wotlk in favour of cata.
		// this cond wouldn't fit those.
		game_track, _ := diff.Pop()
		for i, su := range sul {
			i := i
			if su.GameTrackIDSet.IsEmpty() {
				sul[i].GameTrackIDSet.Add(game_track)
			}
		}
		return sul
	}

	// next case: 1 unclassified asset, multiple available game tracks and
	// *if* we have no retail asset thus far and *if* we have 1 or more assets classified as classic,
	// assume addon only supports some classic game tracks and asset is retail.
	// it is a common case and not a huge assumption but it *is* possible that
	// the addon *doesn't* support retail and only supports some classic game tracks and
	// we failed to guess a game track. would love to see a real world example here.
	if diff.Contains(GAMETRACK_RETAIL) && classified.Cardinality() >= 1 {
		for i, su := range sul {
			if su.GameTrackIDSet.IsEmpty() {
				sul[i].GameTrackIDSet.Add(GAMETRACK_RETAIL)
			}
		}
		return sul
	}

	// slog.Info("github asset classification 2 failed", ...

	return sul
}

// todo: passing `core.App` not amenable to testing.
// pass some sort of 'downloader' interface I can mock up
func download_release_json(app *core.App, url string) (ReleaseJSON, error) {
	headers := map[string]string{}
	empty_resp := ReleaseJSON{}
	resp, err := app.Download(url, headers)
	if err != nil {
		return empty_resp, err
	}
	return ParseReleaseJSON(resp.Bytes)
}

// try downloading the release.json file if it exists.
// this is not an API call but it is 1 of N HTTP requests for N releases.
func classify3(sul []SourceUpdate, release_json ReleaseJSON) []SourceUpdate {
	// a map of asset-name => supported-game-tracks
	m := ReleaseJSONGameTrackMap(release_json)
	for i, su := range sul {
		gts, present := m[su.AssetName]
		if !present {
			slog.Error("release.json missing asset", "a", su.AssetName, "rj", release_json)
			continue
		}
		su.GameTrackIDSet = gts
		sul[i] = su
	}
	return sul
}

// filter/transform/whatever the list of releases from Github.
// returns a list of SourceUpdates.
func process_github_release_list(app *core.App, release_list []GithubRelease) []SourceUpdate {
	final_source_update_list := []SourceUpdate{}
	for i, r := range release_list {
		if r.PreRelease {
			continue
		}

		if r.Draft {
			continue
		}

		var release_json_asset *GithubReleaseAsset
		asset_list := []GithubReleaseAsset{}
		for _, a := range r.AssetList {
			a := a
			if is_release_json(a) {
				release_json_asset = &a
			}
			if !is_supported_zip(a.ContentType) {
				continue
			}
			if !is_fully_uploaded(a.State) {
				continue
			}
			asset_list = append(asset_list, a)
		}

		source_update_list := to_sul(r, asset_list)

		// classify 1
		for i, su := range source_update_list {
			source_update_list[i] = classify1(r, su)
		}

		// classify 2
		source_update_list = classify2(source_update_list)

		// classify 3
		// download release.json, but only for the latest releases
		if i == 0 && release_json_asset != nil {
			release_json, err := download_release_json(app, release_json_asset.BrowserDownloadURL)
			if err != nil {
				slog.Error("failed to download release.json asset, cannot classify release this way", "error", err)
			} else {
				source_update_list = classify3(source_update_list, release_json)
			}
		}

		for i, su := range source_update_list {
			if su.GameTrackIDSet.IsEmpty() {
				slog.Warn("source update still isn't classified! classifying as retail", "su", su)
				source_update_list[i].GameTrackIDSet.Add(GAMETRACK_RETAIL)
			}
			final_source_update_list = append(final_source_update_list, su)

		}

		// pre-8.0 a list of potential game tracks was passed in as :game-track-list or known-game-tracks.
		// this was made up of information from the catalogue or from when the addon was installed.
		// classify3 was doing multiple things, classifying against the release.json file and
		// extrapolating the source updates based on the game-track-list.
		// I think it could be split into a release.json step and the extrapolation shifted into it's own thing.

	}
	return final_source_update_list
}

// ExpandSummary implements AddonSource.
func (g *GithubAPI) ExpandSummary(app *core.App, source_id string) ([]SourceUpdate, error) {

	// create releases url
	// add authentication
	// download release list
	// download release.json for each release
	// bundle them up into a SourceUpdate

	// split releases into N game track lists
	// - for example, vanilla releases, wrath releases, retail releases, etc
	//

	empty_response := []SourceUpdate{}

	github_headers := map[string]string{}
	release_list_resp, err := app.Download(github_release_list_url(source_id), github_headers)
	if err != nil {
		slog.Error("failed to download Github release list", "error", err)
		return empty_response, err
	}

	var release_list []GithubRelease
	json.Unmarshal(release_list_resp.Bytes, &release_list)

	source_update_list := process_github_release_list(app, release_list)
	return source_update_list, nil
}

// todo: better home
func downloaded_addon_fname(normalised_name string, version string) string {
	return fmt.Sprintf("%s--%s.zip", normalised_name, slugify(version)) // everyaddon--1.2.3.zip
}
