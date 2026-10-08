package strongbox

import (
	"log/slog"
	"slices"
)

// read-only reports of how strongbox sees an addons dir and a settings file, for comparing
// strongbox 8 with strongbox 7 and for diagnosing a user's setup. nothing here writes.

// an installed addon as strongbox sees it.
type AddonReport struct {
	Label            string      `json:"label"`
	Name             string      `json:"name"`
	DirList          []string    `json:"dir-list"`           // every directory in the group, the primary first
	GroupID          string      `json:"group-id,omitempty"` // empty when strongbox did not install it
	Primary          string      `json:"primary"`
	Ignored          bool        `json:"ignored"`
	IgnoredReason    string      `json:"ignored-reason,omitempty"`
	Pinned           bool        `json:"pinned"`
	PinnedVersion    string      `json:"pinned-version,omitempty"`
	TOCFile          string      `json:"toc-file,omitempty"` // the .toc chosen for the addons dir's game track
	NFOSource        string      `json:"nfo-source,omitempty"`
	NFOSourceID      string      `json:"nfo-source-id,omitempty"`
	Source           string      `json:"source,omitempty"`
	SourceID         string      `json:"source-id,omitempty"`
	SourceMapList    []SourceMap `json:"source-map-list,omitempty"`
	InstalledVersion string      `json:"installed-version,omitempty"`
	GameVersion      string      `json:"game-version,omitempty"`
}

// returns why the directory `ia` is ignored, or "" when it is not.
func ignored_reason(ia InstalledAddon) string {
	if flag := ia.NFOFile.Ignored(); flag != nil {
		if *flag {
			return "ignored by the user"
		}
		return ""
	}
	if ia.VersionControlled {
		return "under version control"
	}
	for _, name := range ia.toc_file_names() {
		if ia.TOCMap[name].Ignored {
			return "the .toc version is a placeholder: " + string(name)
		}
	}
	return ""
}

// returns the report for the addon `a`.
func addon_report(a Addon) AddonReport {
	report := AddonReport{
		Label:            a.Label,
		Name:             a.Name,
		Primary:          a.Primary.DirName,
		DirList:          []string{a.Primary.DirName},
		Ignored:          a.IsIgnored,
		Pinned:           a.IsPinned,
		PinnedVersion:    a.PinnedVersion,
		Source:           a.Source,
		SourceID:         a.SourceID,
		SourceMapList:    a.SourceMapList,
		InstalledVersion: a.InstalledVersion,
		GameVersion:      a.GameVersion,
	}
	for _, ia := range a.InstalledAddonGroup {
		if !slices.Contains(report.DirList, ia.DirName) {
			report.DirList = append(report.DirList, ia.DirName)
		}
		if reason := ignored_reason(ia); reason != "" && report.IgnoredReason == "" {
			report.IgnoredReason = ia.DirName + ": " + reason
		}
	}
	if a.NFO != nil {
		report.GroupID = a.NFO.GroupID
		report.NFOSource = a.NFO.Source
		report.NFOSourceID = string(a.NFO.SourceID)
	}
	if a.TOC != nil {
		report.TOCFile = a.TOC.FileName
	}
	return report
}

// returns a report of each addon in the addons dir `ad`, in the order strongbox shows them.
// an addon that cannot be loaded is logged and left out, as strongbox leaves it out.
func InspectAddons(ad AddonsDir) ([]AddonReport, error) {
	addon_list, err := LoadAllInstalledAddons(ad)
	if err != nil {
		return nil, err
	}
	report_list := []AddonReport{}
	for _, a := range addon_list {
		report_list = append(report_list, addon_report(a))
	}
	slog.Debug("inspected addons dir", "addons-dir", ad.Path, "num-addons", len(report_list))
	return report_list, nil
}

// a settings file as strongbox reads it.
type SettingsReport struct {
	Format   string   `json:"format"` // the settings file's format, "strongbox 7" or "strongbox 8"
	ReadOnly bool     `json:"read-only"`
	Issues   []string `json:"issues"` // problems found and fixed while reading, "LEVEL: message"
	Settings Settings `json:"settings"`
}

// returns how strongbox reads the settings file contents `b`: the settings it would use
// and every problem it found. `available` reports whether an addons dir's directory
// exists. returns an error only when `b` is not a JSON object.
func InspectSettings(b []byte, available func(path string) bool) (SettingsReport, error) {
	parsed, err := parse_settings(b, available)
	if err != nil {
		return SettingsReport{}, err
	}
	issue_list := []string{}
	for _, issue := range parsed.Issues {
		issue_list = append(issue_list, issue.Level.String()+": "+issue.Message)
	}
	return SettingsReport{
		Format:   string(parsed.Format),
		ReadOnly: parsed.ReadOnly,
		Issues:   issue_list,
		Settings: parsed.Settings,
	}, nil
}
