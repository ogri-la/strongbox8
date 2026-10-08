package strongbox

import (
	"bw/core"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WoWInterface: updates come from its API, which reports no game tracks, so the game
// tracks an addon is known to support are used instead.

type WowinterfaceAPI struct{}

var _ AddonSource = (*WowinterfaceAPI)(nil)

var wowinterface_api_v3 = "https://api.mmoui.com/v3/game/WOW"

func wowinterface_release_url(source_id string) string {
	return fmt.Sprintf("%s/filedetails/%s.json", wowinterface_api_v3, source_id)
}

// returns the download URL of the WoWInterface addon `source_id`.
func wowinterface_download_url(source_id string) string {
	return "https://cdn.wowinterface.com/downloads/getfile.php?id=" + source_id
}

type WowinterfaceFileDetailsV3 struct {
	ID              string `json:"UID"`
	CatID           string `json:"UICATID"`
	Version         string `json:"UIVersion"`
	Date            int64  `json:"UIDate"`
	MD5             string `json:"UIMD5"`
	FileName        string `json:"UIFileName"`
	Download        string `json:"UIDownload"`
	Pending         string `json:"UIPending"`
	Name            string `json:"UIName"`
	AuthorName      string `json:"UIAuthorName"`
	Description     string `json:"UIDescription"`
	ChangeLog       string `json:"UIChangeLog"`
	HitCount        string `json:"UIHitCount"`
	HitCountMonthly string `json:"UIHitCountMonthly"`
	FavoriteTotal   string `json:"UIFavoriteTotal"`
}

// an addon's page: '/downloads/info8882-Name.html' or '/downloads/download8882-Name'.
var WOWINTERFACE_PATH_REGEX = regexp.MustCompile(`^/downloads/(?:info|download)(\d+)(?:[-.].*)?$`)

// returns the addon ID in a WoWInterface addon page URL.
// clj: `wowinterface_api.clj/parse-user-string`
func (w *WowinterfaceAPI) ParseURL(raw_url string) (string, bool) {
	u, err := url.Parse(with_scheme(strings.TrimSpace(raw_url)))
	if err != nil {
		return "", false
	}
	if strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") != "wowinterface.com" {
		return "", false
	}
	matches := WOWINTERFACE_PATH_REGEX.FindStringSubmatch(u.Path)
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

// returns the WoWInterface addon `source_id` from the loaded catalogue.
// strongbox only knows WoWInterface addons through the catalogue: its API does not
// describe an addon well enough to install it.
func (w *WowinterfaceAPI) FindAddon(app *core.App, source_id string) (CatalogueAddon, error) {
	cat, ok := loaded_catalogue(app)
	if !ok {
		return CatalogueAddon{}, fmt.Errorf("%w: no catalogue is loaded to find WoWInterface addon %s in", ErrNotFound, source_id)
	}
	for _, ca := range cat.AddonSummaryList {
		if ca.Source == SOURCE_WOWI && string(ca.SourceID) == source_id {
			return ca, nil
		}
	}
	return CatalogueAddon{}, fmt.Errorf("%w: WoWInterface addon %s is not in the catalogue", ErrNotFound, source_id)
}

// returns the update for the addon in `req`, supporting the game tracks it is known to
// support. an addon with no known game tracks has no updates: assuming retail would offer
// a classic-only addon to a retail addons dir.
// a 404 is an error wrapping `ErrNotFound`.
func (w *WowinterfaceAPI) ExpandSummary(app *core.App, req ExpandRequest) ([]SourceUpdate, error) {
	if req.KnownGameTracks == nil || req.KnownGameTracks.IsEmpty() {
		return []SourceUpdate{}, fmt.Errorf("wowinterface: no game tracks are known for addon %s", req.SourceID)
	}

	b, err := download_ok(app, wowinterface_release_url(req.SourceID), "wowinterface", nil)
	if err != nil {
		return []SourceUpdate{}, err
	}

	var detail_list []WowinterfaceFileDetailsV3
	if err := json.Unmarshal(b, &detail_list); err != nil {
		return []SourceUpdate{}, fmt.Errorf("wowinterface: unexpected response: %w", err)
	}

	source_updates := []SourceUpdate{}
	for _, detail := range detail_list {
		su := NewSourceUpdate()
		su.Version = detail.Version
		su.DownloadURL = wowinterface_download_url(req.SourceID)
		su.GameTrackIDSet = req.KnownGameTracks.Clone()
		su.PublishedDate = time.UnixMilli(detail.Date).UTC()
		su.AssetName = detail.FileName
		source_updates = append(source_updates, su)
	}
	return source_updates, nil
}
