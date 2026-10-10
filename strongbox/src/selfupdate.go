package strongbox

import (
	"bw/core"
	"cmp"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
)

// checking whether a newer strongbox has been released. strongbox never updates itself.
// clj: `core.clj/latest-strongbox-release!`

const STRONGBOX_RELEASES_URL = "https://api.github.com/repos/ogri-la/strongbox/releases"

// keyval set to the newer version available, when there is one.
const KV_UPDATE_AVAILABLE = "app.update-available"

// state key for where a newer strongbox can be downloaded, shown with `KV_UPDATE_AVAILABLE`.
const KV_UPDATE_URL = "app.update-url"

// a version split into its numeric parts and any pre-release suffix.
// "v8.0.0-alpha.3" => [8 0 0], "alpha.3"
type semver struct {
	numbers    []int
	prerelease string
}

// returns `version` parsed for ordering. non-numeric parts count as zero.
func parse_semver(version string) semver {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	core_part, prerelease, _ := strings.Cut(version, "-")
	numbers := []int{}
	for _, bit := range strings.Split(core_part, ".") {
		n, _ := strconv.Atoi(bit)
		numbers = append(numbers, n)
	}
	return semver{numbers: numbers, prerelease: prerelease}
}

// returns -1, 0 or 1 as `a` orders before, equal to or after `b` by semantic version:
// numeric parts numerically, missing parts as zero, and a pre-release before the same
// version without one.
func compare_semver(a string, b string) int {
	va, vb := parse_semver(a), parse_semver(b)
	for i := range max(len(va.numbers), len(vb.numbers)) {
		na, nb := 0, 0
		if i < len(va.numbers) {
			na = va.numbers[i]
		}
		if i < len(vb.numbers) {
			nb = vb.numbers[i]
		}
		if c := cmp.Compare(na, nb); c != 0 {
			return c
		}
	}
	switch {
	case va.prerelease == vb.prerelease:
		return 0
	case va.prerelease == "":
		return 1
	case vb.prerelease == "":
		return -1
	}
	return cmp.Compare(va.prerelease, vb.prerelease)
}

type strongbox_release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	PreRelease bool   `json:"prerelease"`
}

// returns the newest version among `release_list`, ignoring drafts and pre-releases, or
// "" when there is none.
func latest_release_version(release_list []strongbox_release) string {
	latest := ""
	for _, r := range release_list {
		if r.Draft || r.PreRelease || r.TagName == "" {
			continue
		}
		if latest == "" || compare_semver(r.TagName, latest) > 0 {
			latest = r.TagName
		}
	}
	return latest
}

// when the `check-for-update` preference is on, asks GitHub for strongbox's releases and,
// when one is newer than the running version, tells the user and records it in app state
// for the About dialog. a failure is logged at WARN.
func CheckForStrongboxUpdate(app *core.App) {
	if !FindSettings(app).Preferences.CheckForUpdate {
		return
	}
	b, err := download_ok(app, STRONGBOX_RELEASES_URL, "github", github_headers())
	if err != nil {
		slog.Warn("failed to check for a newer strongbox", "error", err)
		return
	}
	var release_list []strongbox_release
	if err := json.Unmarshal(b, &release_list); err != nil {
		slog.Warn("failed to check for a newer strongbox", "error", err)
		return
	}
	latest := latest_release_version(release_list)
	if latest != "" && compare_semver(latest, VERSION) > 0 {
		version := strings.TrimPrefix(latest, "v")
		app.State().SetKeyAnyVal(KV_UPDATE_URL, PROJECT_URL+"/releases")
		app.State().SetKeyAnyVal(KV_UPDATE_AVAILABLE, version)
		slog.Info("a newer strongbox is available", "version", version, "url", PROJECT_URL+"/releases")
	}
}
