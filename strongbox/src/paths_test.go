package strongbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns a `Getenv` answering from `env`.
func fake_env(env map[string]string) Getenv {
	return func(key string) string { return env[key] }
}

func Test_GeneratePathMap__defaults(t *testing.T) {
	actual := GeneratePathMap(fake_env(nil), "/home/you")
	assert.Equal(t, "/home/you/.config/strongbox8", actual["app.config-dir"])
	assert.Equal(t, "/home/you/.local/share/strongbox8", actual["app.data-dir"])
	assert.Equal(t, "/home/you/.config/strongbox8/config.json", actual["strongbox.paths.cfg-file"])
	assert.Equal(t, "/home/you/.config/strongbox8/user-catalogue.json", actual["strongbox.paths.user-catalogue-file"])
	assert.Equal(t, "/home/you/.local/share/strongbox8/cache", actual["strongbox.paths.cache-dir"])
	assert.Equal(t, "/home/you/.local/share/strongbox8", actual["strongbox.paths.catalogue-dir"])
}

func Test_GeneratePathMap__xdg(t *testing.T) {
	actual := GeneratePathMap(fake_env(map[string]string{"XDG_CONFIG_HOME": "/tmp/cfg", "XDG_DATA_HOME": "/tmp/data"}), "/home/you")
	assert.Equal(t, "/tmp/cfg/strongbox8/config.json", actual["strongbox.paths.cfg-file"])
	assert.Equal(t, "/tmp/data/strongbox8", actual["app.data-dir"])
}

func Test_GeneratePathMap__xdg_empty_or_relative(t *testing.T) {
	for _, given := range []string{"", "   ", "relative/dir"} {
		actual := GeneratePathMap(fake_env(map[string]string{"XDG_CONFIG_HOME": given, "XDG_DATA_HOME": given}), "/home/you")
		assert.Equal(t, "/home/you/.config/strongbox8", actual["app.config-dir"], given)
		assert.Equal(t, "/home/you/.local/share/strongbox8", actual["app.data-dir"], given)
	}
}

func Test_GeneratePathMap__xdg_already_named(t *testing.T) {
	actual := GeneratePathMap(fake_env(map[string]string{"XDG_CONFIG_HOME": "/tmp/strongbox8/"}), "/home/you")
	assert.Equal(t, "/tmp/strongbox8", actual["app.config-dir"])
}

// every path is absolute and named for what it is. strongbox 8 never shares a directory
// with strongbox 7.
// clj: `core_test.clj/paths`
func Test_GeneratePathMap__shape(t *testing.T) {
	for _, env := range []map[string]string{nil, {"XDG_CONFIG_HOME": "/tmp/cfg", "XDG_DATA_HOME": "/tmp/data"}} {
		v7 := GenerateV7Paths(fake_env(env), "/home/you")
		for key, val := range GeneratePathMap(fake_env(env), "/home/you") {
			assert.Regexp(t, `-(dir|file)$`, key)
			assert.Regexp(t, `^/`, val)
			assert.Contains(t, val, "strongbox8", key)
			assert.NotEqual(t, v7.CfgFile, val)
		}
	}
}

func Test_GenerateV7Paths(t *testing.T) {
	actual := GenerateV7Paths(fake_env(nil), "/home/you")
	assert.Equal(t, V7Paths{CfgFile: "/home/you/.config/strongbox/config.json", UserCatalogueFile: "/home/you/.config/strongbox/user-catalogue.json"}, actual)

	actual = GenerateV7Paths(fake_env(map[string]string{"XDG_CONFIG_HOME": "/tmp/cfg"}), "/home/you")
	assert.Equal(t, "/tmp/cfg/strongbox/config.json", actual.CfgFile)

	// strongbox 7 did not add 'strongbox' when the directory already ended with it
	actual = GenerateV7Paths(fake_env(map[string]string{"XDG_CONFIG_HOME": "/tmp/cfg/strongbox"}), "/home/you")
	assert.Equal(t, "/tmp/cfg/strongbox/config.json", actual.CfgFile)
}
