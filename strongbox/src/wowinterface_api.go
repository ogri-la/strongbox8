package strongbox

import (
	"bw/core"
	"encoding/json"
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

type WowinterfaceAPI struct{}

var _ AddonSource = (*WowinterfaceAPI)(nil)

var wowinterface_api_v3 = "https://api.mmoui.com/v3/game/WOW"

func wowinterface_release_url(source_id string) string {
	return fmt.Sprintf("%s/filedetails/%s.json", wowinterface_api_v3, source_id)
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

// ExpandSummary implements AddonSource.
func (w *WowinterfaceAPI) ExpandSummary(app *core.App, addon Addon) ([]SourceUpdate, error) {
	empty_response := []SourceUpdate{}

	url := wowinterface_release_url(addon.SourceID)
	headers := map[string]string{}

	resp, err := core.Download(app, url, headers)
	if err != nil {
		return empty_response, err
	}

	var dest []WowinterfaceFileDetailsV3
	err = json.Unmarshal(resp.Bytes, &dest)
	if err != nil {
		return empty_response, err
	}

	// 2023-06-09: we don't expect more than one result from wowi, ever, but for the sake of testing and
	// consistency with other hosts it is now supported.

	source_updates := []SourceUpdate{}
	for _, update := range dest {
		source_updates = append(source_updates, SourceUpdate{
			Version:        update.Version,
			DownloadURL:    update.Download,
			GameTrackIDSet: mapset.NewSet(GAMETRACK_RETAIL),
			PublishedDate:  time.UnixMilli(update.Date),
		})
	}

	return source_updates, nil
}
