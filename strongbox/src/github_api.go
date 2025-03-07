package strongbox

import (
	"bw/core"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type GithubAPI struct{}

var _ AddonSource = (*GithubAPI)(nil)

// a Github release has many assets.
type GithubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// a Github repository has many releases.
type GithubRelease struct {
	Name            string               `json:"name"` // "2.2.2"
	AssetList       []GithubReleaseAsset `json:"assets"`
	PublishedAtDate time.Time            `json:"published_at"`
}

// ---

func releases_url(source_id string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases", source_id)
}

func github_headers() map[string]string {
	return map[string]string{}
}

// ---

// ExpandSummary implements AddonSource.
func (g *GithubAPI) ExpandSummary(app *core.App, addon Addon) ([]SourceUpdate, error) {

	// create releases url
	// add authentication
	// download release list
	// download release.json for each release
	// bundle them up into a SourceUpdate

	// split releases into N game track lists
	// - for example, vanilla releases, wrath releases, retail releases, etc
	//

	empty_response := []SourceUpdate{}

	release_list_resp, err := core.Download(app, releases_url(addon.SourceID), github_headers())
	if err != nil {
		slog.Error("failed to download Github release list", "error", err)
		return empty_response, err
	}

	var release_list []GithubRelease
	json.Unmarshal(release_list_resp.Bytes, &release_list)

	source_update_list := []SourceUpdate{}
	for _, r := range release_list {
		for _, a := range r.AssetList {
			source_update_list = append(source_update_list, SourceUpdate{
				Version:          r.Name,
				DownloadURL:      a.BrowserDownloadURL,
				GameTrackID:      "???",
				InterfaceVersion: 0,
			})
		}
	}

	return source_update_list, nil

}
