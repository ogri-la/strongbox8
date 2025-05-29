package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv("XDG_DATA_HOME", "/tmp/test/data")
	os.Setenv("XDG_CONFIG_HOME", "/tmp/test/config")
	exitCode := m.Run()
	os.Exit(exitCode)
}

// single test to prevent flashing
func Test_main_gui(t *testing.T) {
	gui := main_gui()
	defer gui.Stop()

	testfn_list := []struct {
		label string
		fn    func(t *testing.T)
	}{
		{"default tab count is correct", func(t *testing.T) {
			assert.True(t, len(gui.TabList) > 0)
		}},
		{"a form can be opened, splitting the tab body in two", func(t *testing.T) {
			/*
				   tab := gui.TabList[0] // addons-dir
				service, err := gui.App.FindService("new-addons-dir")
				assert.Nil(t, err)


					form := core.MakeForm(service) //
					gui_form := gui.MakeForm(form) //

					gui.TkSync(func() {
						tab.OpenForm(service)

					})
			*/
		}},
	}

	for _, testfn := range testfn_list {
		t.Run(testfn.label, testfn.fn)
	}
}
