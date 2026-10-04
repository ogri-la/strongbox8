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

// returns the API url for the first page of releases of the given `source_id`.
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

// returns a version for the given `asset`, or an empty string when there is nothing to
// use.
// prefers the release name the author chose, then the git tag, then the asset name:
// the tag is typically more meaningful than the asset's file name.
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

// returns one unclassified `SourceUpdate` per asset in `asset_list`.
// game tracks are left empty for the `classify*` functions to fill in.
// each update takes the release's publication date, not the asset's own dates.
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

// first classification pass: guesses the game track of a single update from its asset
// name, then its publication date, then the release name.
// an update published before WoW Classic existed must be retail.
// an update that cannot be guessed is left unclassified for the later passes.
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

// second classification pass: infers the game track of a lone unclassified update from
// the game tracks its siblings already have.
// with exactly one supported game track unaccounted for, the unclassified update must be
// it. With several unaccounted for, no game track is assumed.
// more than one unclassified update, or none, leaves `sul` unchanged.
func classify2(sul []SourceUpdate) []SourceUpdate {
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

	if num_unclassified != 1 || diff.Cardinality() != 1 {
		return sul
	}

	game_track, _ := diff.Pop()
	for i, su := range sul {
		if su.GameTrackIDSet.IsEmpty() {
			sul[i].GameTrackIDSet.Add(game_track)
		}
	}
	return sul
}

// downloads the release.json at the given `url`.
// todo: takes a `core.App` rather than a downloader interface, which makes it awkward to
// test.
func download_release_json(app *core.App, url string) ([]byte, error) {
	headers := map[string]string{}
	resp, err := app.Download(url, headers)
	if err != nil {
		return nil, err
	}
	return resp.Bytes, nil
}

// third classification pass: replaces guessed game tracks with those the addon author
// declared in `release_json`.
// an update with no entry, or with an entry of only unrecognised flavors, is left as it
// was.
func classify3(sul []SourceUpdate, release_json ReleaseJSON) []SourceUpdate {
	// a map of asset-name => supported-game-tracks
	m := ReleaseJSONGameTrackMap(release_json)
	for i, su := range sul {
		gts, present := m[su.AssetName]
		if !present {
			slog.Debug("release.json has no recognised game track for asset", "asset", su.AssetName)
			continue
		}
		su.GameTrackIDSet = gts
		sul[i] = su
	}
	return sul
}

// downloads the release.json at `url` and applies `classify3` with it.
// a failed download or a bad release.json leaves `sul` unchanged.
// a failed download is logged at WARN: the network is the user's to fix. A bad release.json
// is logged at DEBUG: it is the addon author's data, not the user's.
func classify_using_release_json(app *core.App, url string, sul []SourceUpdate) []SourceUpdate {
	b, err := download_release_json(app, url)
	if err != nil {
		slog.Warn("failed to download release.json, classifying without it", "url", url, "error", err)
		return sul
	}
	release_json, err := ParseReleaseJSON(b)
	if err != nil {
		slog.Debug("failed to parse release.json, classifying without it", "url", url, "error", err)
		return sul
	}
	return classify3(sul, release_json)
}

// returns the releases in `release_list` that are neither drafts nor prereleases, in the
// same order.
func published_release_list(release_list []GithubRelease) []GithubRelease {
	published := []GithubRelease{}
	for _, r := range release_list {
		if r.Draft || r.PreRelease {
			continue
		}
		published = append(published, r)
	}
	return published
}

// returns the given Github `release_list` as a list of source updates, classified by
// game track.
// drafts and pre-releases are skipped, as are assets that are not fully uploaded .zips.
// the release.json is downloaded for the newest published release only, to keep this to
// one extra HTTP request rather than one per release.
// an update still unclassified after all three passes is excluded. No game track is
// assumed for it.
func process_github_release_list(app *core.App, release_list []GithubRelease) []SourceUpdate {
	final_source_update_list := []SourceUpdate{}
	for i, r := range published_release_list(release_list) {
		var release_json_asset *GithubReleaseAsset
		asset_list := []GithubReleaseAsset{}
		for _, a := range r.AssetList {
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
			source_update_list = classify_using_release_json(app, release_json_asset.BrowserDownloadURL, source_update_list)
		}

		for _, su := range source_update_list {
			if su.GameTrackIDSet.IsEmpty() {
				slog.Debug("excluding Github asset, game track unknown", "release", r.Name, "asset", su.AssetName)
				continue
			}
			final_source_update_list = append(final_source_update_list, su)
		}

		// todo: pre-8.0 a list of known game tracks, from the catalogue or from when the
		// addon was installed, was also used to extrapolate updates. that extrapolation
		// belongs in a step of its own rather than back inside `classify3`.
	}
	return final_source_update_list
}

// only the first page of releases is fetched, so an addon with many releases is
// truncated.
// a release list that fails to parse yields no updates rather than an error.
func (g *GithubAPI) ExpandSummary(app *core.App, source_id string) ([]SourceUpdate, error) {
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
