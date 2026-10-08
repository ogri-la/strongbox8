package strongbox

import (
	"bw/core"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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
	DownloadCount      int                     `json:"download_count"`
	CreatedDate        time.Time               `json:"created_at"`
	UpdatedDate        time.Time               `json:"updated_at"`
}

// a Github repository has many releases.
type GithubRelease struct {
	Name          string               `json:"name"`     // "1.2.3"
	TagName       string               `json:"tag_name"` // "v1.2.3"
	HTMLURL       string               `json:"html_url"` // "https://github.com/Owner/Repo/releases/tag/v1.2.3"
	AssetList     []GithubReleaseAsset `json:"assets"`
	PublishedDate time.Time            `json:"published_at"`
	Draft         bool                 `json:"draft"`
	PreRelease    bool                 `json:"prerelease"`
}

// ---

// returns the API url for the first page of releases of the given `source_id`.
func github_release_list_url(source_id string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100&page=1", source_id)
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
	// forever is never inferred: an unlabelled asset far more likely belongs to an
	// established game track.
	candidates := gametrack_set()
	candidates.Remove(GAMETRACK_FOREVER)
	diff := candidates.Difference(classified) // #{:classic :classic-bc :retail} #{:classic :classic-bc} => #{:retail}

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

// fourth classification pass: a release's only update, still unclassified, takes the game
// tracks the addon is known to support. an addon shipping one zip for every game track is
// common, and the catalogue's game tracks come from the addon's own .toc files.
// several updates, an already classified update, or no known game tracks change nothing:
// giving several zips the same game tracks would be a guess.
func classify4(sul []SourceUpdate, known_game_tracks mapset.Set[GameTrackID]) []SourceUpdate {
	if len(sul) != 1 || !sul[0].GameTrackIDSet.IsEmpty() || known_game_tracks == nil || known_game_tracks.IsEmpty() {
		return sul
	}
	sul[0].GameTrackIDSet = known_game_tracks.Clone()
	return sul
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
// `known_game_tracks` are used by the fourth pass, see `classify4`.
// an update still unclassified after every pass is excluded. No game track is assumed
// for it.
func process_github_release_list(app *core.App, release_list []GithubRelease, known_game_tracks mapset.Set[GameTrackID]) []SourceUpdate {
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

		// classify 4
		source_update_list = classify4(source_update_list, known_game_tracks)

		for _, su := range source_update_list {
			if su.GameTrackIDSet.IsEmpty() {
				slog.Debug("excluding Github asset, game track unknown", "release", r.Name, "asset", su.AssetName)
				continue
			}
			final_source_update_list = append(final_source_update_list, su)
		}
	}
	return final_source_update_list
}

// returns the headers for a GitHub API request, authenticated with `GITHUB_TOKEN` when
// it is set.
func github_headers() map[string]string {
	headers := map[string]string{"Accept": "application/vnd.github+json"}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

// returns the releases of the GitHub repository `source_id`, newest first.
// only the first page is fetched, so an addon with very many releases is truncated.
func download_github_release_list(app *core.App, source_id string) ([]GithubRelease, error) {
	b, err := download_ok(app, github_release_list_url(source_id), "github", github_headers())
	if err != nil {
		return nil, err
	}
	var release_list []GithubRelease
	if err := json.Unmarshal(b, &release_list); err != nil {
		return nil, fmt.Errorf("github: unexpected response listing releases: %w", err)
	}
	return release_list, nil
}

// returns the 'owner/repo' named by a GitHub URL.
// clj: `github_api.clj/parse-user-string`
func (g *GithubAPI) ParseURL(raw_url string) (string, bool) {
	return github_source_id_from_url(raw_url)
}

// returns the updates available from the GitHub repository in `req`.
// GitHub releases carry their own game tracks, see `process_github_release_list`.
func (g *GithubAPI) ExpandSummary(app *core.App, req ExpandRequest) ([]SourceUpdate, error) {
	release_list, err := download_github_release_list(app, req.SourceID)
	if err != nil {
		return []SourceUpdate{}, err
	}
	return process_github_release_list(app, release_list, req.KnownGameTracks), nil
}

// returns the newest published release with at least one asset, or `false`.
// clj: `github_api.clj/find-latest-release`
func find_latest_github_release(release_list []GithubRelease) (GithubRelease, bool) {
	for _, r := range published_release_list(release_list) {
		if len(r.AssetList) > 0 {
			return r, true
		}
	}
	return GithubRelease{}, false
}

// returns the game tracks declared by the first release.json found in `release_list`.
// clj: `github_api.clj/find-gametracks-release-json`
func github_release_json_game_tracks(app *core.App, release_list []GithubRelease) mapset.Set[GameTrackID] {
	for _, r := range release_list {
		for _, a := range r.AssetList {
			if is_release_json(a) && is_fully_uploaded(a.State) {
				b, err := download_release_json(app, a.BrowserDownloadURL)
				if err != nil {
					return mapset.NewSet[GameTrackID]()
				}
				rj, err := ParseReleaseJSON(b)
				if err != nil {
					return mapset.NewSet[GameTrackID]()
				}
				return ReleaseJSONGameTrackList(rj)
			}
		}
	}
	return mapset.NewSet[GameTrackID]()
}

// a file in a GitHub repository's root listing.
type github_content struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

// returns the game tracks the .toc files in the root of the GitHub repository
// `source_id` support: from each file name, else from its interface versions.
// clj: `github_api.clj/find-gametracks-toc-data`
func github_toc_game_tracks(app *core.App, source_id string) mapset.Set[GameTrackID] {
	game_tracks := mapset.NewSet[GameTrackID]()
	b, err := download_ok(app, fmt.Sprintf("https://api.github.com/repos/%s/contents", source_id), "github", github_headers())
	if err != nil {
		slog.Debug("failed to list github repository contents", "source-id", source_id, "error", err)
		return game_tracks
	}
	var listing []github_content
	if err := json.Unmarshal(b, &listing); err != nil {
		return game_tracks
	}
	for _, item := range listing {
		if item.Type != "file" || !strings.HasSuffix(strings.ToLower(item.Name), ".toc") {
			continue
		}
		toc_b, err := download_ok(app, item.DownloadURL, "github", nil)
		if err != nil {
			continue
		}
		toc := coerce_toc_data(parse_toc_file(string(toc_b)), "/"+source_id+"/"+item.Name)
		game_tracks = game_tracks.Union(toc.GameTrackIDSet)
	}
	return game_tracks
}

// returns the GitHub repository `source_id` as a catalogue entry.
// it must have a published release with assets, and its game tracks must be found in its
// release assets, a release.json or its .toc files: none are assumed.
// the source ID is taken from the release, correcting the case of `source_id`.
// clj: `github_api.clj/find-addon`
func (g *GithubAPI) FindAddon(app *core.App, source_id string) (CatalogueAddon, error) {
	release_list, err := download_github_release_list(app, source_id)
	if err != nil {
		return CatalogueAddon{}, err
	}
	latest, ok := find_latest_github_release(release_list)
	if !ok {
		return CatalogueAddon{}, fmt.Errorf("%w: github repository %s has no published release with files to install", ErrNotFound, source_id)
	}

	game_tracks := mapset.NewSet[GameTrackID]()
	for _, su := range process_github_release_list(app, release_list, nil) {
		game_tracks = game_tracks.Union(su.GameTrackIDSet)
	}
	if game_tracks.IsEmpty() {
		game_tracks = github_release_json_game_tracks(app, release_list)
	}
	if game_tracks.IsEmpty() {
		game_tracks = github_toc_game_tracks(app, source_id)
	}
	if game_tracks.IsEmpty() {
		return CatalogueAddon{}, fmt.Errorf("%w: no game tracks could be found for github repository %s", ErrNotFound, source_id)
	}

	canonical_id, ok := github_source_id_from_url(latest.HTMLURL)
	if !ok {
		canonical_id = source_id
	}
	_, repo, _ := strings.Cut(canonical_id, "/")

	download_count := 0
	for _, r := range release_list {
		for _, a := range r.AssetList {
			download_count += a.DownloadCount
		}
	}

	return CatalogueAddon{
		URL:             "https://github.com/" + canonical_id,
		Name:            slugify(repo),
		Label:           repo,
		TagList:         []string{},
		UpdatedDate:     latest.PublishedDate,
		DownloadCount:   download_count,
		Source:          SOURCE_GITHUB,
		SourceID:        FlexString(canonical_id),
		GameTrackIDList: sorted_game_tracks(game_tracks),
	}, nil
}

// returns the 'owner/repo' a GitHub repository URL names, and `true`, or `false` when
// `raw_url` is not a GitHub repository URL.
// the scheme, a 'www.' prefix, user info, a query, an anchor, a trailing slash and deeper
// paths are tolerated: "https://www.github.com/Aviana/HealComm/releases?x=1" => "Aviana/HealComm"
// clj: `github_api.clj/parse-user-string`
func github_source_id_from_url(raw_url string) (string, bool) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", false
	}
	if !strings.Contains(raw_url, "://") {
		raw_url = "https://" + strings.TrimPrefix(raw_url, "//")
	}
	u, err := url.Parse(raw_url)
	if err != nil {
		return "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "github.com" {
		return "", false
	}
	bits := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(bits) < 2 || bits[0] == "" || bits[1] == "" {
		return "", false
	}
	return bits[0] + "/" + strings.TrimSuffix(bits[1], ".git"), true
}
