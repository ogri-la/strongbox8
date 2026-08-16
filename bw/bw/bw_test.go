package bw

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"bw/core"

	"github.com/stretchr/testify/assert"
)

// builds a small directory tree and returns its root:
//
//	root/
//	  .hidden-file
//	  a-dir/           (empty)
//	  b-dir/nested-file.txt
//	  a-file.txt
//	  z-file.txt
func fixture_tree(t *testing.T) string {
	root := t.TempDir()
	for _, dir := range []string{"a-dir", "b-dir"} {
		assert.Nil(t, os.Mkdir(filepath.Join(root, dir), 0755))
	}
	for _, file := range []string{".hidden-file", "a-file.txt", "z-file.txt", "b-dir/nested-file.txt"} {
		assert.Nil(t, os.WriteFile(filepath.Join(root, file), []byte{}, 0644))
	}
	return root
}

// returns the `browse` service from the `os/fs` service group.
func browse_service(t *testing.T) core.Service {
	for _, group := range provider() {
		for _, service := range group.ServiceList {
			if service.Label == "browse" {
				return service
			}
		}
	}
	t.Fatal("browse service not found")
	return core.Service{}
}

// browsing a directory adds a single lazy result and reads nothing
func Test_browse__adds_lazy_root(t *testing.T) {
	given := fixture_tree(t)

	app := core.NewApp()
	service_result := browse_service(t).Fn(app, core.MakeServiceFnArgs("dir", given))
	app.ProcessUpdate()

	assert.Nil(t, service_result.Err)

	actual := app.GetResultList()
	assert.Equal(t, 1, len(actual))
	assert.Equal(t, given, actual[0].ID)
	assert.Equal(t, BW_NS_FS_DIR, actual[0].NS)
	assert.False(t, actual[0].ChildrenRealised)
}

// browsing the same directory twice does not duplicate results
func Test_browse__no_duplicates(t *testing.T) {
	given := fixture_tree(t)

	app := core.NewApp()
	service := browse_service(t)
	service.Fn(app, core.MakeServiceFnArgs("dir", given))
	app.ProcessUpdate()
	service.Fn(app, core.MakeServiceFnArgs("dir", given))
	app.ProcessUpdate()

	assert.Equal(t, 1, len(app.GetResultList()))
}

// browsing a path that is a file or does not exist is rejected by form
// validation, so the service never runs and no result is added
func Test_browse__invalid_path_rejected(t *testing.T) {
	root := fixture_tree(t)

	given := map[string]string{
		"a file":         filepath.Join(root, "a-file.txt"),
		"a missing path": filepath.Join(root, "no-such-dir"),
	}

	for label, path := range given {
		form := core.MakeForm(browse_service(t))
		form.Update([]core.KeyVal{{Key: "dir", Val: path}})

		actual := form.Validate()
		assert.NotNil(t, actual, label)
		if actual != nil {
			assert.Contains(t, actual.FieldErrorList, "dir", label)
		}
	}

	// the same validation passes for a real directory
	form := core.MakeForm(browse_service(t))
	form.Update([]core.KeyVal{{Key: "dir", Val: root}})
	assert.Nil(t, form.Validate())
}

// expanding a directory lists one level only: directories first then files,
// each sorted by name, hidden entries included
func Test_dir_children__one_level_ordered(t *testing.T) {
	given := fixture_tree(t)

	app := core.NewApp()
	app.AppendResults(MakeDirResult(given))
	app.ProcessUpdate()

	actual, err := core.Children(app, app.FindResultByID(given))
	app.ProcessUpdate()
	assert.Nil(t, err)

	expected := []string{
		filepath.Join(given, "a-dir"),
		filepath.Join(given, "b-dir"),
		filepath.Join(given, ".hidden-file"),
		filepath.Join(given, "a-file.txt"),
		filepath.Join(given, "z-file.txt"),
	}
	actual_ids := []string{}
	for _, child := range actual {
		assert.Equal(t, given, child.ParentID)
		actual_ids = append(actual_ids, child.ID)
	}
	assert.Equal(t, expected, actual_ids)

	// one level only: the nested file was not read
	assert.False(t, app.HasResult(filepath.Join(given, "b-dir", "nested-file.txt")))

	// subdirectories are lazy, files are leaves
	assert.False(t, app.FindResultByID(filepath.Join(given, "a-dir")).ChildrenRealised)
	_, is_file := app.FindResultByID(filepath.Join(given, "a-file.txt")).Item.(File)
	assert.True(t, is_file)
}

// listing the same directory twice produces the same order
func Test_dir_children__deterministic(t *testing.T) {
	given := Dir{Path: fixture_tree(t)}

	expected := given.ItemChildren(nil)
	actual := given.ItemChildren(nil)

	assert.NotEmpty(t, expected)
	assert.Equal(t, expected, actual)
}

// a symlink is classified by what it points at
func Test_dir_children__symlinks(t *testing.T) {
	root := fixture_tree(t)
	assert.Nil(t, os.Symlink(filepath.Join(root, "a-dir"), filepath.Join(root, "link-to-dir")))
	assert.Nil(t, os.Symlink(filepath.Join(root, "a-file.txt"), filepath.Join(root, "link-to-file")))

	actual := map[string]bool{} // path => is a Dir
	for _, child := range (Dir{Path: root}).ItemChildren(nil) {
		_, is_dir := child.Item.(Dir)
		actual[child.ID] = is_dir
	}

	assert.True(t, actual[filepath.Join(root, "link-to-dir")])
	assert.False(t, actual[filepath.Join(root, "link-to-file")])
}

// an unreadable directory yields no children and a warning
func Test_dir_children__unreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, directory permissions are not enforced")
	}

	root := fixture_tree(t)
	locked := filepath.Join(root, "a-dir")
	assert.Nil(t, os.Chmod(locked, 0000))
	defer os.Chmod(locked, 0755)

	var log_output bytes.Buffer
	original_logger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log_output, nil)))
	defer slog.SetDefault(original_logger)

	actual := Dir{Path: locked}.ItemChildren(nil)

	assert.Equal(t, []core.Result{}, actual)
	assert.Contains(t, log_output.String(), "level=WARN")
	assert.Contains(t, log_output.String(), "failed to read directory")
}

func TestBWProvider_ID(t *testing.T) {
	provider := &BWProvider{}
	assert.Equal(t, "boardwalk", provider.ID())
}

func TestBWProvider_ServiceList(t *testing.T) {
	provider := &BWProvider{}
	services := provider.ServiceList()
	assert.NotEmpty(t, services)

	// Should have at least one service group
	assert.Greater(t, len(services), 0)

	// Check that the first service group has some services
	if len(services) > 0 {
		assert.NotEmpty(t, services[0].ServiceList)
	}
}

func TestBWProvider_ItemHandlerMap(t *testing.T) {
	provider := &BWProvider{}
	handlerMap := provider.ItemHandlerMap()
	assert.NotNil(t, handlerMap)
	// Empty map is fine for this provider
}

func TestBWProvider_Menu(t *testing.T) {
	provider := &BWProvider{}
	menu := provider.Menu()
	assert.NotNil(t, menu)
	// Empty menu is fine for this provider
}

func TestProvider(t *testing.T) {
	app := core.NewApp()
	bwProvider := Provider(app)
	assert.NotNil(t, bwProvider)
	assert.Equal(t, "boardwalk", bwProvider.ID())
}

func TestAnnotationStruct(t *testing.T) {
	annotation := Annotation{
		Annotation:  "test annotation",
		AnnotatedID: "test-id",
	}

	assert.Equal(t, "test annotation", annotation.Annotation)
	assert.Equal(t, "test-id", annotation.AnnotatedID)
}
