package strongbox

import (
	"path/filepath"
	"strings"
)

// filesystem locations strongbox uses.
// strongbox 8 keeps its own config and data directories, separate from strongbox 7's,
// and only ever reads strongbox 7's files.
// - https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html

// the directory name strongbox 8 uses within the XDG config and data directories.
const APP_DIR_NAME = "strongbox8"

// the directory name strongbox 7 uses within the XDG config and data directories.
const V7_APP_DIR_NAME = "strongbox"

// returns the value of an environment variable, `os.Getenv` in practice.
// passed in so path derivation is a pure function of its inputs.
type Getenv = func(key string) string

// returns `xdg_dir` joined with `app_dir_name`, or `fallback` joined with it when
// `xdg_dir` is empty or only whitespace.
// an XDG directory that is relative is ignored, as the spec requires.
// an XDG directory already named `app_dir_name` is used as it is, as strongbox 7 did.
func xdg_app_dir(xdg_dir string, fallback string, app_dir_name string) string {
	xdg_dir = strings.TrimSpace(xdg_dir)
	if xdg_dir == "" || !filepath.IsAbs(xdg_dir) {
		return filepath.Join(fallback, app_dir_name)
	}
	xdg_dir = filepath.Clean(xdg_dir)
	if filepath.Base(xdg_dir) == app_dir_name {
		return xdg_dir
	}
	return filepath.Join(xdg_dir, app_dir_name)
}

// returns every filesystem path strongbox 8 uses, keyed by name, for the given `home`
// directory and environment.
// every key ends in '-dir' or '-file'. `init_dirs` creates the '-dir' ones.
// `XDG_CONFIG_DIRS` and `XDG_DATA_DIRS` are not consulted.
func GeneratePathMap(getenv Getenv, home string) map[string]string {
	config_dir := xdg_app_dir(getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"), APP_DIR_NAME)
	data_dir := xdg_app_dir(getenv("XDG_DATA_HOME"), filepath.Join(home, ".local", "share"), APP_DIR_NAME)
	return map[string]string{
		"app.config-dir": config_dir,
		"app.data-dir":   data_dir,

		// "/home/$you/.local/share/strongbox8"
		"strongbox.paths.catalogue-dir": data_dir,

		// "/home/$you/.local/share/strongbox8/cache"
		"strongbox.paths.cache-dir": filepath.Join(data_dir, "cache"),

		// "/home/$you/.config/strongbox8/config.json"
		"strongbox.paths.cfg-file": filepath.Join(config_dir, "config.json"),

		// "/home/$you/.config/strongbox8/user-catalogue.json"
		"strongbox.paths.user-catalogue-file": filepath.Join(config_dir, "user-catalogue.json"),
	}
}

// strongbox 7's files that strongbox 8 imports once.
type V7Paths struct {
	CfgFile           string // "/home/$you/.config/strongbox/config.json"
	UserCatalogueFile string // "/home/$you/.config/strongbox/user-catalogue.json"
}

// returns the paths of strongbox 7's files for the given `home` directory and environment.
func GenerateV7Paths(getenv Getenv, home string) V7Paths {
	config_dir := xdg_app_dir(getenv("XDG_CONFIG_HOME"), filepath.Join(home, ".config"), V7_APP_DIR_NAME)
	return V7Paths{
		CfgFile:           filepath.Join(config_dir, "config.json"),
		UserCatalogueFile: filepath.Join(config_dir, "user-catalogue.json"),
	}
}
