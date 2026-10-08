package strongbox

import (
	"bw/core"
	"bw/http_utils"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

// addon hosts: where addons are found by URL and where their updates come from.

// what a host needs to know to list an addon's updates.
type ExpandRequest struct {
	SourceID string

	// the game tracks the addon is known to support, from the catalogue or from the nfo
	// data written when it was installed. a host that reports no game tracks of its own,
	// such as wowinterface, uses these.
	KnownGameTracks mapset.Set[GameTrackID]
}

// an addon host.
type AddonSource interface {
	// returns the source ID named by the URL `raw_url` on this host, and `true`, or
	// `false` when the URL does not name an addon on this host.
	ParseURL(raw_url string) (string, bool)

	// returns the addon `source_id` described as a catalogue entry.
	// returns an error when it cannot be found or has nothing to install.
	FindAddon(app *core.App, source_id string) (CatalogueAddon, error)

	// returns the updates available for the addon in `req`, newest first.
	ExpandSummary(app *core.App, req ExpandRequest) ([]SourceUpdate, error)
}

// the hosts updates can come from, by source.
// a map: a source names exactly one host.
var SOURCE_MAP = map[Source]AddonSource{
	SOURCE_GITHUB: &GithubAPI{},
	SOURCE_GITLAB: &GitlabAPI{},
	SOURCE_WOWI:   &WowinterfaceAPI{},
}

// the hosts URLs are tried against, in order.
var SOURCE_ORDER = []Source{SOURCE_GITHUB, SOURCE_GITLAB, SOURCE_WOWI}

var ErrUnsupportedSource = errors.New("unsupported source")
var ErrNotFound = errors.New("not found")

// returns the host for `source`, or an error wrapping `ErrUnsupportedSource`.
func addon_source(source Source) (AddonSource, error) {
	host, present := SOURCE_MAP[source]
	if !present {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSource, source)
	}
	return host, nil
}

// returns the source and source ID named by `raw_url`, or an error explaining which URLs
// are accepted.
// clj: `catalogue.clj/parse-user-string`
func ParseAddonURL(raw_url string) (Source, string, error) {
	raw_url = strings.TrimSpace(raw_url)
	host_name := ""
	if u, err := url.Parse(with_scheme(raw_url)); err == nil {
		host_name = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	}
	if strings.Contains(host_name, "curseforge") || strings.Contains(host_name, "tukui") {
		return "", "", fmt.Errorf("%w: %s no longer exists as a host for addons", ErrUnsupportedSource, host_name)
	}
	for _, source := range SOURCE_ORDER {
		if source_id, ok := SOURCE_MAP[source].ParseURL(raw_url); ok {
			return source, source_id, nil
		}
	}
	return "", "", fmt.Errorf("unrecognised addon URL %q. accepted URLs look like: "+
		"https://github.com/owner/repository, https://gitlab.com/group/project, "+
		"https://www.wowinterface.com/downloads/info12345-Name.html", raw_url)
}

// returns `raw_url` with an https scheme when it has none.
func with_scheme(raw_url string) string {
	if strings.Contains(raw_url, "://") {
		return raw_url
	}
	return "https://" + strings.TrimPrefix(raw_url, "//")
}

// returns the updates for `source`/`source_id`. an unsupported source is an error
// wrapping `ErrUnsupportedSource`.
func ExpandSummary(app *core.App, source Source, req ExpandRequest) ([]SourceUpdate, error) {
	host, err := addon_source(source)
	if err != nil {
		return []SourceUpdate{}, err
	}
	return host.ExpandSummary(app, req)
}

// returns the game tracks `a` is known to support: its catalogue entry's, else the game
// track it was installed under, else none.
func known_game_tracks(a Addon) mapset.Set[GameTrackID] {
	if a.CatalogueAddon != nil && len(a.CatalogueAddon.GameTrackIDList) > 0 {
		return mapset.NewSet(a.CatalogueAddon.GameTrackIDList...)
	}
	if a.NFO != nil && a.NFO.InstalledGameTrackID != "" {
		return mapset.NewSet(a.NFO.InstalledGameTrackID)
	}
	return mapset.NewSet[GameTrackID]()
}

// returns the request to list the updates for `a`.
func expand_request(a Addon) ExpandRequest {
	return ExpandRequest{SourceID: a.SourceID, KnownGameTracks: known_game_tracks(a)}
}

// returns the request to list the updates for the catalogue entry `ca`.
func expand_request_for(ca CatalogueAddon) ExpandRequest {
	return ExpandRequest{SourceID: string(ca.SourceID), KnownGameTracks: mapset.NewSet(ca.GameTrackIDList...)}
}

// --- http

// returns an error describing a failed response `resp` from `host` in terms the user can
// act on, or nil for a 2xx response.
// clj: `http.clj/http-error`
func response_error(resp *http_utils.ResponseWrapper, host string) error {
	if resp == nil || resp.Response == nil {
		return fmt.Errorf("%s: no response", host)
	}
	status := resp.StatusCode
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s responded with HTTP 404", ErrNotFound, host)
	case status == http.StatusForbidden && host == "github" && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return errors.New("github: we've exceeded our request quota and have been blocked for an hour. setting GITHUB_TOKEN raises the quota")
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%s: too many requests, try again later", host)
	case status >= 500:
		return fmt.Errorf("%s: the host is having problems (HTTP %d), try again later", host, status)
	default:
		return fmt.Errorf("%s: unexpected response, HTTP %d", host, status)
	}
}

// returns the body of `url` from `host`, or an error for a failed request or response.
func download_ok(app *core.App, url string, host string, headers map[string]string) ([]byte, error) {
	resp, err := app.Download(url, headers)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", host, err)
	}
	if resp.Response != nil {
		if err := response_error(resp, host); err != nil {
			return nil, err
		}
	}
	return resp.Bytes, nil
}

// returns the game tracks in `set`, in the order they are offered to the user.
func sorted_game_tracks(set mapset.Set[GameTrackID]) []GameTrackID {
	out := []GameTrackID{}
	for _, gt := range GAME_TRACK_LIST {
		if set.Contains(gt.ID) {
			out = append(out, gt.ID)
		}
	}
	return out
}

// returns `true` when `list` holds a release for one of `game_tracks`.
func has_release_for(list []SourceUpdate, game_tracks []GameTrackID) bool {
	return slices.ContainsFunc(list, func(su SourceUpdate) bool {
		return slices.ContainsFunc(game_tracks, func(gt GameTrackID) bool { return su.GameTrackIDSet.Contains(gt) })
	})
}
