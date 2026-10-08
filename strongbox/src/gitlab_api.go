package strongbox

import (
	"bw/core"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

// GitLab: addons found by project URL, updates from the project's releases.
// clj: `gitlab_api.clj`

type GitlabAPI struct{}

var _ AddonSource = (*GitlabAPI)(nil)

// returns the GitLab API URL for the project `source_id`, the project path URL-encoded.
// "woblight/nitro" => "https://gitlab.com/api/v4/projects/woblight%2Fnitro"
func gitlab_api_url(source_id string) string {
	return "https://gitlab.com/api/v4/projects/" + url.QueryEscape(strings.ToLower(source_id))
}

type gitlab_link struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	DirectAssetURL string `json:"direct_asset_url"`
	External       bool   `json:"external"`
	LinkType       string `json:"link_type"`
}

type gitlab_release struct {
	Name            string    `json:"name"`
	TagName         string    `json:"tag_name"`
	ReleasedAt      time.Time `json:"released_at"`
	UpcomingRelease bool      `json:"upcoming_release"`
	Assets          struct {
		Links []gitlab_link `json:"links"`
	} `json:"assets"`
}

type gitlab_project struct {
	WebURL            string    `json:"web_url"`
	CreatedAt         time.Time `json:"created_at"`
	LastActivityAt    time.Time `json:"last_activity_at"`
	PathWithNamespace string    `json:"path_with_namespace"`
	Name              string    `json:"name"`
	Path              string    `json:"path"`
	Description       string    `json:"description"`
}

type gitlab_tree_item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// the link types that are files to install. 'runbook' and 'image' are not.
var GITLAB_LINK_TYPES = mapset.NewSet("package", "other")

// returns the preferred download URL for `link`: its permanent direct asset URL when it
// has one. "The physical location of the asset can change at any time and the direct link
// remains unchanged."
func gitlab_link_url(link gitlab_link) string {
	if link.DirectAssetURL != "" {
		return link.DirectAssetURL
	}
	return link.URL
}

// returns the project path in a GitLab URL: two or three path segments before any '/-/'.
// "https://gitlab.com/woblight/nitro/-/releases" => "woblight/nitro"
// clj: `gitlab_api.clj/parse-user-string`
func (g *GitlabAPI) ParseURL(raw_url string) (string, bool) {
	u, err := url.Parse(with_scheme(strings.TrimSpace(raw_url)))
	if err != nil {
		return "", false
	}
	if strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") != "gitlab.com" {
		return "", false
	}
	path, _, _ := strings.Cut(u.Path, "/-")
	bits := strings.Split(strings.Trim(path, "/"), "/")
	if len(bits) < 2 || bits[0] == "" || bits[1] == "" {
		return "", false
	}
	if len(bits) > 3 {
		bits = bits[:3]
	}
	return strings.Join(bits, "/"), true
}

// returns the updates in GitLab `release_list`, newest first, classified as GitHub
// updates are. upcoming releases are excluded. `latest` is the index of the newest
// release, the only one whose release.json is consulted.
func process_gitlab_release_list(app *core.App, release_list []gitlab_release, known_game_tracks mapset.Set[GameTrackID]) []SourceUpdate {
	final := []SourceUpdate{}
	published := 0
	for _, r := range release_list {
		if r.UpcomingRelease {
			continue
		}

		var release_json_link *gitlab_link
		sul := []SourceUpdate{}
		for _, link := range r.Assets.Links {
			if link.Name == "release.json" {
				release_json_link = &link
				continue
			}
			if link.External || !GITLAB_LINK_TYPES.Contains(link.LinkType) {
				continue
			}
			su := NewSourceUpdate()
			su.AssetName = link.Name
			su.Version = r.TagName
			su.DownloadURL = gitlab_link_url(link)
			su.PublishedDate = r.ReleasedAt
			sul = append(sul, su)
		}

		// the classification passes are shared with github
		as_github := GithubRelease{Name: r.Name, TagName: r.TagName, PublishedDate: r.ReleasedAt}
		for i, su := range sul {
			sul[i] = classify1(as_github, su)
		}
		sul = classify2(sul)
		if published == 0 && release_json_link != nil {
			sul = classify_using_release_json(app, gitlab_link_url(*release_json_link), sul)
		}
		sul = classify4(sul, known_game_tracks)
		published++

		for _, su := range sul {
			if su.GameTrackIDSet.IsEmpty() {
				slog.Debug("excluding gitlab asset, game track unknown", "release", r.Name, "asset", su.AssetName)
				continue
			}
			final = append(final, su)
		}
	}
	return final
}

// returns the releases of the GitLab project `source_id`.
func download_gitlab_release_list(app *core.App, source_id string) ([]gitlab_release, error) {
	b, err := download_ok(app, gitlab_api_url(source_id)+"/releases", "gitlab", nil)
	if err != nil {
		return nil, err
	}
	var release_list []gitlab_release
	if err := json.Unmarshal(b, &release_list); err != nil {
		return nil, fmt.Errorf("gitlab: unexpected response listing releases: %w", err)
	}
	return release_list, nil
}

// returns the updates available from the GitLab project in `req`.
// clj: `gitlab_api.clj/expand-summary`
func (g *GitlabAPI) ExpandSummary(app *core.App, req ExpandRequest) ([]SourceUpdate, error) {
	release_list, err := download_gitlab_release_list(app, req.SourceID)
	if err != nil {
		return []SourceUpdate{}, err
	}
	return process_gitlab_release_list(app, release_list, req.KnownGameTracks), nil
}

// returns the .toc files in the root of the GitLab project `source_id`, file name => blob
// API URL.
// clj: `gitlab_api.clj/find-toc-files`
func gitlab_toc_files(app *core.App, source_id string) map[string]string {
	toc_files := map[string]string{}
	b, err := download_ok(app, gitlab_api_url(source_id)+"/repository/tree", "gitlab", nil)
	if err != nil {
		slog.Debug("failed to list gitlab repository", "source-id", source_id, "error", err)
		return toc_files
	}
	var tree []gitlab_tree_item
	if err := json.Unmarshal(b, &tree); err != nil {
		return toc_files
	}
	for _, item := range tree {
		if strings.HasSuffix(strings.ToLower(item.Path), ".toc") {
			toc_files[item.Name] = gitlab_api_url(source_id) + "/repository/blobs/" + item.ID
		}
	}
	return toc_files
}

// returns the parsed contents of the base64 encoded GitLab blob at `blob_url`.
// clj: `gitlab_api.clj/download-decode-blob`
func gitlab_toc_blob(app *core.App, blob_url string) (map[string]string, error) {
	b, err := download_ok(app, blob_url, "gitlab", nil)
	if err != nil {
		return nil, err
	}
	blob := struct {
		Content string `json:"content"`
	}{}
	if err := json.Unmarshal(b, &blob); err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
	if err != nil {
		return nil, err
	}
	return parse_toc_file(string(decoded)), nil
}

// returns the game tracks the .toc files of the GitLab project `source_id` support: from
// each file name, else from the interface versions inside it.
// clj: `gitlab_api.clj/guess-game-track-list`
func gitlab_game_tracks(app *core.App, source_id string) mapset.Set[GameTrackID] {
	game_tracks := mapset.NewSet[GameTrackID]()
	for file_name, blob_url := range gitlab_toc_files(app, source_id) {
		toc := coerce_toc_data(map[string]string{"title": file_name}, "/"+source_id+"/"+file_name)
		if toc.FileNameGameTrackID != "" {
			game_tracks.Add(toc.FileNameGameTrackID)
			continue
		}
		kvs, err := gitlab_toc_blob(app, blob_url)
		if err != nil {
			slog.Debug("failed to read gitlab .toc file", "file", file_name, "error", err)
			continue
		}
		game_tracks = game_tracks.Union(coerce_toc_data(kvs, "/"+source_id+"/"+file_name).GameTrackIDSet)
	}
	return game_tracks
}

// returns the GitLab project `source_id` as a catalogue entry.
// its game tracks come from its .toc files, and it must have a release offering one of them.
// GitLab does not make download counts public, so the count is zero.
// clj: `gitlab_api.clj/find-addon`
func (g *GitlabAPI) FindAddon(app *core.App, source_id string) (CatalogueAddon, error) {
	b, err := download_ok(app, gitlab_api_url(source_id), "gitlab", nil)
	if err != nil {
		return CatalogueAddon{}, err
	}
	project := gitlab_project{}
	if err := json.Unmarshal(b, &project); err != nil {
		return CatalogueAddon{}, fmt.Errorf("gitlab: unexpected response describing project: %w", err)
	}

	game_tracks := gitlab_game_tracks(app, source_id)
	if game_tracks.IsEmpty() {
		return CatalogueAddon{}, fmt.Errorf("%w: no game tracks could be found for gitlab project %s", ErrNotFound, source_id)
	}

	release_list, err := download_gitlab_release_list(app, source_id)
	if err != nil {
		return CatalogueAddon{}, err
	}
	sul := process_gitlab_release_list(app, release_list, game_tracks)
	if !has_release_for(sul, sorted_game_tracks(game_tracks)) {
		return CatalogueAddon{}, fmt.Errorf("%w: gitlab project %s has no release to install", ErrNotFound, source_id)
	}

	return CatalogueAddon{
		URL:             project.WebURL,
		Name:            project.Path,
		Label:           project.Name,
		Description:     project.Description,
		TagList:         []string{},
		CreatedDate:     project.CreatedAt,
		UpdatedDate:     project.LastActivityAt,
		DownloadCount:   0,
		Source:          SOURCE_GITLAB,
		SourceID:        FlexString(project.PathWithNamespace),
		GameTrackIDList: sorted_game_tracks(game_tracks),
	}, nil
}
