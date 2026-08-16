package main

import (
	"bw/bw"
	"bw/core"
	"bw/ui"
	"log/slog"
	"os"
	"path/filepath"
	strongbox "strongbox/src"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/visualfc/atk/tk"
)

// test fixture: a lazy item whose children take longer to load than any timeout.
type SlowItem struct {
	Name string
}

func (si SlowItem) ItemKeys() []string {
	return []string{core.ITEM_FIELD_NAME}
}

func (si SlowItem) ItemMap() map[string]string {
	return map[string]string{core.ITEM_FIELD_NAME: si.Name}
}

func (si SlowItem) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_LAZY
}

func (si SlowItem) ItemChildren(*core.App) []core.Result {
	time.Sleep(1 * time.Second)
	return []core.Result{core.MakeResult(bw.BW_NS_FS_FILE, bw.File{Path: "/slow-child"}, "/slow-child")}
}

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	exitCode := m.Run()
	os.Exit(exitCode)
}

// single test to prevent flashing
func Test_main_gui(t *testing.T) {

	// todo: the envvars above are not preventing the catalogue from loading

	tmpdir := t.TempDir()

	data_dir := filepath.Join(tmpdir, "xdg-data")     // "/tmp/rand/xdg-data"
	config_dir := filepath.Join(tmpdir, "xdg-config") // "/tmp/rand/xdg-config"

	os.Setenv("XDG_DATA_HOME", data_dir)
	os.Setenv("XDG_CONFIG_HOME", config_dir)

	/*
		// Ensure catalogue directory exists but is empty (no old format files)
		catalogue_dir := filepath.Join(data_dir, "strongbox", "catalogues")
		core.MakeDirs(catalogue_dir)
	*/

	addons_dir := filepath.Join(tmpdir, "addons") // "/tmp/rand/addons"
	core.MakeDirs(addons_dir)

	gui := main_gui()
	defer gui.Stop()
	defer gui.App().Stop() // urgh, this is all over the place. don't bundle app with gui?

	testfn_list := []struct {
		label string
		fn    func(t *testing.T)
	}{
		{"default tab count is correct", func(t *testing.T) {
			assert.True(t, len(gui.TabList) > 0)
		}},
		{"a form can be opened, filled, submitted", func(t *testing.T) {
			tab := gui.TabList[0] // addons dirs

			// find a service function to render
			service, err := gui.App().FindService("new-addons-dir")
			assert.Nil(t, err)

			assert.Nil(t, tab.GUIForm) // no form is set
			initial_data := []core.KeyVal{}
			tab.OpenForm(service, initial_data) // open details and set a form
			assert.NotNil(t, tab.GUIForm)       // form is now set

			// todo: test the gui rendered the form

			// bind form
			form_data := []core.KeyVal{
				{Key: "Foo", Val: "Bar"},
			}

			// hrm: not great. we have tab.OpenForm and tab.CloseForm, why not tab.FillForm(data) ? and tab.SubmitForm() ?
			// when we SubmitForm the form will need to be re-rendered with any errors, etc.
			gui.TkSync(func() {
				tab.GUIForm.Form.Update(form_data) // bind form data
				tab.GUIForm.Fill()                 // flush changes in form to gui
			})

			// todo: test the gui rendered the data

			// we can submit the form directly and access the args to pass to the Service fn this way

			service_fn_args, form_err := tab.GUIForm.Submit()
			assert.NotNil(t, form_err.Error) // data was bad, we expect a validation error
			assert.Empty(t, service_fn_args) // because of bad data, the result should be empty

			// --- ok, try again with actual data

			// re-bind form with actual data
			form_data = []core.KeyVal{
				{Key: "addons-dir", Val: addons_dir},
			}

			gui.TkSync(func() {
				tab.GUIForm.Form.Update(form_data) // bind form data
				tab.GUIForm.Fill()                 // flush changes in form to gui
			})

			// check the 'addons-dir' field in the gui form matches our temp dir
			addons_dir_field := tab.GUIForm.Fields[0]
			var field_value string
			gui.TkSync(func() {
				field_value = addons_dir_field.Input.Get()
			})
			assert.Equal(t, addons_dir, field_value)

			service_fn_args, form_err = tab.GUIForm.Submit()
			assert.Nil(t, form_err) // data was good, form errors should be nil

			expected := core.KeyVal{Key: "addons-dir", Val: addons_dir}
			assert.Equal(t, expected, service_fn_args.ArgList[0])

			// clicking the submit button will actually call the bound service with the valid args

			submit_btn := tab.GUIForm.Fields[1].Input.(*ui.TKButton)

			gui.TkSync(func() {
				submit_btn.Invoke() // ... so, this is how the form should be invoked if we're testing gui behaviour.
			})

			gui.WaitForServices()

			// a successful invocation this way (clicking submit button) closes the open form and discards the reference to the form

			// assert.Nil(t, tab.GUIForm) // hrm: this should be nil, we're passing a pointer around
			assert.Nil(t, gui.GetCurrentTab().GUIForm)

			// we now have one addons dir in the application state whose path is equal to the temp addons dir

			rl := gui.App().FilterResultListByNS(strongbox.NS_ADDONS_DIR)
			assert.Equal(t, 1, len(rl))

			r := rl[0]
			ad := r.Item.(strongbox.AddonsDir)
			assert.Equal(t, addons_dir, ad.Path)
		}},
		{"lazy rows are expandable and expansion realises children", func(t *testing.T) {
			// the app's own files tab shows filesystem results and load failures
			tab := gui.GetTab("files")
			assert.NotNil(t, tab)

			// fixture tree: root/{sub-dir/nested.txt, empty-dir/, file.txt}
			root := t.TempDir()
			sub_dir := filepath.Join(root, "sub-dir")
			empty_dir := filepath.Join(root, "empty-dir")
			assert.Nil(t, os.Mkdir(sub_dir, 0755))
			assert.Nil(t, os.Mkdir(empty_dir, 0755))
			assert.Nil(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte{}, 0644))
			assert.Nil(t, os.WriteFile(filepath.Join(sub_dir, "nested.txt"), []byte{}, 0644))

			// row state read from the Tk thread
			row := func(id string) (fkey string, child_count int, has_placeholder bool) {
				gui.TkSync(func() {
					fkey = tab.ItemFkeyIndex[id]
					child_count = tab.RowChildCount(id)
					has_placeholder = tab.HasPlaceholderRow(id)
				})
				return fkey, child_count, has_placeholder
			}
			row_present := func(id string) func() bool {
				return func() bool {
					fkey, _, _ := row(id)
					return fkey != ""
				}
			}

			// browse the fixture tree
			service, err := gui.App().FindService(bw.SERVICE_ID_FS_BROWSE)
			assert.Nil(t, err)
			gui.RunService(service, core.MakeServiceFnArgs("dir", root), nil)
			gui.WaitForServices()

			// the root row appears with a placeholder child: it can be expanded
			// even though nothing has been read yet
			assert.Eventually(t, row_present(root), 5*time.Second, 25*time.Millisecond)
			root_fkey, child_count, has_placeholder := row(root)
			assert.Equal(t, 1, child_count)
			assert.True(t, has_placeholder)
			assert.False(t, gui.App().FindResultByID(root).ChildrenRealised)

			// expanding the root realises one level and removes the placeholder
			tab.ExpandRow(root_fkey)
			assert.Eventually(t, row_present(filepath.Join(root, "file.txt")), 5*time.Second, 25*time.Millisecond)
			_, child_count, has_placeholder = row(root)
			assert.Equal(t, 3, child_count) // sub-dir, empty-dir, file.txt
			assert.False(t, has_placeholder)

			// the nested file was not read: one level only
			assert.False(t, gui.App().HasResult(filepath.Join(sub_dir, "nested.txt")))

			// a subdirectory is itself expandable via a placeholder, a file is a leaf
			_, child_count, has_placeholder = row(sub_dir)
			assert.Equal(t, 1, child_count)
			assert.True(t, has_placeholder)
			_, child_count, has_placeholder = row(filepath.Join(root, "file.txt"))
			assert.Equal(t, 0, child_count)
			assert.False(t, has_placeholder)

			// expanding an empty directory leaves a leaf row: no children, no placeholder
			empty_fkey, _, _ := row(empty_dir)
			tab.ExpandRow(empty_fkey)
			assert.Eventually(t, func() bool {
				_, child_count, has_placeholder := row(empty_dir)
				return child_count == 0 && !has_placeholder
			}, 5*time.Second, 25*time.Millisecond)
			assert.True(t, gui.App().FindResultByID(empty_dir).ChildrenRealised)

			// collapse then re-expand redisplays existing rows without reloading
			given_result_count := len(gui.App().GetResultList())
			tab.CollapseRow(root_fkey)
			tab.ExpandRow(root_fkey)
			time.Sleep(250 * time.Millisecond) // allow any (wrong) reload to land
			_, child_count, _ = row(root)
			assert.Equal(t, 3, child_count)
			assert.Equal(t, given_result_count, len(gui.App().GetResultList()))
		}},
		{"row updates survive tcl-hostile filenames", func(t *testing.T) {
			tab := gui.GetTab("files")

			// java's `.userPrefs` tree encodes keys with punctuation, so names
			// like these occur in practice. the closing brace before the opening
			// one breaks naive tcl brace-quoting.
			root := t.TempDir()
			hostile_dir := filepath.Join(root, `japrefs_!'}{[]$\" dir`)
			hostile_file := filepath.Join(hostile_dir, `nested_}$\" file.txt`)
			assert.Nil(t, os.Mkdir(hostile_dir, 0755))
			assert.Nil(t, os.WriteFile(hostile_file, []byte{}, 0644))

			// record tk errors for the duration of this test
			var mu sync.Mutex
			tk_errors := []string{}
			tk.SetErrorHandle(func(err error) {
				mu.Lock()
				defer mu.Unlock()
				tk_errors = append(tk_errors, err.Error())
			})
			defer tk.SetErrorHandle(func(err error) {
				slog.Error("tk", "error", err)
			})

			row_present := func(id string) func() bool {
				return func() bool {
					var fkey string
					gui.TkSync(func() { fkey = tab.ItemFkeyIndex[id] })
					return fkey != ""
				}
			}

			// browse the hostile directory and expand it: realisation marks the
			// row modified, and the row update must survive the hostile cells
			service, err := gui.App().FindService(bw.SERVICE_ID_FS_BROWSE)
			assert.Nil(t, err)
			gui.RunService(service, core.MakeServiceFnArgs("dir", hostile_dir), nil)
			gui.WaitForServices()
			assert.Eventually(t, row_present(hostile_dir), 5*time.Second, 25*time.Millisecond)

			var fkey string
			gui.TkSync(func() { fkey = tab.ItemFkeyIndex[hostile_dir] })
			tab.ExpandRow(fkey)
			assert.Eventually(t, row_present(hostile_file), 5*time.Second, 25*time.Millisecond)

			mu.Lock()
			defer mu.Unlock()
			assert.Empty(t, tk_errors)
		}},
		{"a timed-out expansion presents a terminal failed row", func(t *testing.T) {
			tab := gui.GetTab("files")

			original_timeout := core.LazyRealiseTimeout
			core.LazyRealiseTimeout = 100 * time.Millisecond
			defer func() { core.LazyRealiseTimeout = original_timeout }()

			slow_id := "slow-item"
			gui.App().AddReplaceResults(core.MakeResult(bw.BW_NS_FS_DIR, SlowItem{Name: "slow"}, slow_id)).Wait()

			row := func(id string) (fkey string, child_count int, has_placeholder bool) {
				gui.TkSync(func() {
					fkey = tab.ItemFkeyIndex[id]
					child_count = tab.RowChildCount(id)
					has_placeholder = tab.HasPlaceholderRow(id)
				})
				return fkey, child_count, has_placeholder
			}

			// the slow row appears with a placeholder
			var slow_fkey string
			assert.Eventually(t, func() bool {
				slow_fkey, _, _ = row(slow_id)
				return slow_fkey != ""
			}, 5*time.Second, 25*time.Millisecond)

			tab.ExpandRow(slow_fkey)

			// the load times out: the placeholder is replaced by a single terminal 'failed' row
			failed_id := slow_id + "/load-failure"
			assert.Eventually(t, func() bool {
				fkey, _, _ := row(failed_id)
				return fkey != ""
			}, 5*time.Second, 25*time.Millisecond)

			_, child_count, has_placeholder := row(slow_id)
			assert.Equal(t, 1, child_count)
			assert.False(t, has_placeholder)
			_, failed_child_count, failed_has_placeholder := row(failed_id)
			assert.Equal(t, 0, failed_child_count)
			assert.False(t, failed_has_placeholder)
			assert.True(t, gui.App().FindResultByID(slow_id).ChildrenRealised)

			// the ui stayed responsive: the tk thread answers while the load is still sleeping
			responsive := false
			gui.TkSync(func() { responsive = true })
			assert.True(t, responsive)

			// the late results from the abandoned load are discarded
			time.Sleep(1200 * time.Millisecond)
			assert.False(t, gui.App().HasResult("/slow-child"))
		}},
	}

	for _, testfn := range testfn_list {
		t.Run(testfn.label, testfn.fn)
	}
}
