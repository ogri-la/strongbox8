// the boardwalk provider: the services boardwalk offers about itself and the filesystem.
// registered alongside the application's own providers.
package bw

import (
	"bw/core"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
)

const SERVICE_ID_FS_BROWSE = "fs-browse"

var (
	BW_NS_ANNOTATION_ANNOTATION = core.MakeNS("bw", "annotation", "annotation")
	BW_NS_RESULT_LIST           = core.MakeNS("bw", "core", "result-list")
	BW_NS_ERROR                 = core.MakeNS("bw", "core", "error")
	BW_NS_STATE                 = core.MakeNS("bw", "core", "state")
	BW_NS_SERVICE               = core.MakeNS("bw", "core", "service")
	BW_NS_FS_FILE               = core.MakeNS("bw", "fs", "file")
	BW_NS_FS_DIR                = core.MakeNS("bw", "fs", "dir")
)

// a user's note about another result, identified by `AnnotatedID`.
type Annotation struct {
	Annotation  string
	AnnotatedID string
}

// a file on the local filesystem. a leaf: it never has children.
type File struct {
	Path string // absolute path, also used as the result ID
}

func (f File) ItemKeys() []string {
	return []string{core.ITEM_FIELD_NAME}
}

func (f File) ItemMap() map[string]string {
	return map[string]string{core.ITEM_FIELD_NAME: filepath.Base(f.Path)}
}

func (f File) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_FALSE
}

func (f File) ItemChildren(*core.App) []core.Result {
	return []core.Result{}
}

var _ core.ItemInfo = (*File)(nil)

// wraps the file at `path` in a result. `path` is the result ID.
func MakeFileResult(path string) core.Result {
	return core.MakeResult(BW_NS_FS_FILE, File{Path: path}, path)
}

// a directory on the local filesystem.
// its children, the directory's entries, load on demand one level at a time.
type Dir struct {
	Path string // absolute path, also used as the result ID
}

func (d Dir) ItemKeys() []string {
	return []string{core.ITEM_FIELD_NAME}
}

func (d Dir) ItemMap() map[string]string {
	return map[string]string{core.ITEM_FIELD_NAME: filepath.Base(d.Path)}
}

func (d Dir) ItemHasChildren() core.ITEM_CHILDREN_LOAD {
	return core.ITEM_CHILDREN_LOAD_LAZY
}

// returns the directory's immediate entries: directories first, then files, each
// group sorted by name. symlinks are classified by what they point at.
// an unreadable directory is reported as a warning and returns no children.
func (d Dir) ItemChildren(*core.App) []core.Result {
	entry_list, err := os.ReadDir(d.Path)
	if err != nil {
		slog.Warn("failed to read directory, no children returned", "path", d.Path, "error", err)
		return []core.Result{}
	}

	// `os.ReadDir` sorts entries by name, so each group is already ordered.
	dir_list := []core.Result{}
	file_list := []core.Result{}
	for _, entry := range entry_list {
		full_path := filepath.Join(d.Path, entry.Name())
		is_dir := entry.IsDir()
		if !is_dir && entry.Type()&os.ModeSymlink != 0 {
			info, stat_err := os.Stat(full_path)
			is_dir = stat_err == nil && info.IsDir()
		}
		if is_dir {
			dir_list = append(dir_list, MakeDirResult(full_path))
		} else {
			file_list = append(file_list, MakeFileResult(full_path))
		}
	}

	return append(dir_list, file_list...)
}

var _ core.ItemInfo = (*Dir)(nil)

// wraps the directory at `path` in a result. `path` is the result ID.
func MakeDirResult(path string) core.Result {
	return core.MakeResult(BW_NS_FS_DIR, Dir{Path: path}, path)
}

func start_bw(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
	fmt.Println("starting bw!")
	return core.ServiceResult{}
}

// returns every service group boardwalk offers.
func provider() []core.ServiceGroup {
	empty_result := core.ServiceResult{}

	return []core.ServiceGroup{
		{
			NS: core.NS{Major: "bw", Minor: "state", Type: "service"},
			ServiceList: []core.Service{

				core.StartProviderService(start_bw),

				{
					Label:     "print-state",
					Interface: core.ServiceInterface{},
					Fn: func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
						fmt.Println(core.QuickJSON(app.State))
						return empty_result
					},
				},

				{
					Label: "reset-state",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							core.ConfirmYesArgDef(),
						},
					},
					Fn: func(app *core.App, _ core.ServiceFnArgs) core.ServiceResult {
						app.ResetState()
						return empty_result
					},
				},
			},
		},

		{
			NS: core.NS{Major: "os", Minor: "fs", Type: "service"},
			ServiceList: []core.Service{
				{
					ID:          SERVICE_ID_FS_BROWSE,
					Label:       "browse",
					Description: "browse a directory, listing its contents as they are expanded.",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							core.DirArgDef(),
						},
					},
					Fn: func(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
						path := args.ArgList[0].Val.(string)
						abs_path, err := filepath.Abs(path)
						if err != nil {
							return core.MakeServiceResultError(err, "cannot resolve path")
						}
						// the directory is lazy: nothing is read until the user expands it.
						result := MakeDirResult(abs_path)
						app.AddReplaceResults(result)
						return core.MakeServiceResult(result)
					},
				},
				{
					Label: "list-files",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							core.DirArgDef(),
						},
					},
					Fn: func(_ *core.App, args core.ServiceFnArgs) core.ServiceResult {
						path := args.ArgList[0].Val.(string)
						results := core.ServiceResult{}
						file_list, err := os.ReadDir(path)
						file_name_list := []core.Result{}
						for _, file := range file_list {
							ns := BW_NS_FS_FILE
							if file.IsDir() {
								ns = BW_NS_FS_DIR
							}
							file_name_list = append(file_name_list, core.MakeResult(ns, file.Name(), core.UniqueID()))
						}
						if err != nil {
							results.Err = err
							return results
						}
						return core.ServiceResult{Result: file_name_list}
					},
				},
				{
					Label:       "list-files-recursive-flat",
					Description: "recursively visits each subdir in given dir, return a flat list of files and directories.",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							core.DirArgDef(),
						},
					},
					Fn: func(_ *core.App, args core.ServiceFnArgs) core.ServiceResult {
						path := args.ArgList[0].Val.(string)
						results := []core.Result{}
						var readdir func(string) []core.Result
						readdir = func(root string) []core.Result {
							results = append(results, core.MakeResult(BW_NS_FS_DIR, root, core.UniqueID()))
							file_list, err := os.ReadDir(root)
							if err != nil {
								return results
							}
							for _, file := range file_list {
								full_path := filepath.Join(root, file.Name())
								info, err := os.Stat(full_path)
								if err != nil {
									continue
								}
								if info.IsDir() {
									readdir(full_path)
								} else {
									results = append(results, core.MakeResult(BW_NS_FS_FILE, full_path, core.UniqueID()))
								}
							}
							return results
						}
						readdir(path)
						return core.MakeServiceResult(results...)
					},
				},
			},
		},

		{
			NS: core.NS{Major: "bw", Minor: "annotation", Type: "service"},
			ServiceList: []core.Service{
				{
					Label: "annotate",
					Interface: core.ServiceInterface{
						ArgDefList: []core.ArgDef{
							{
								ID:            "selected",
								Label:         "Selected",
								Parser:        core.ParseStringAsResultID,
								ValidatorList: []core.PredicateFn{core.HasResultValidator},
							},
							{
								ID:     "annotation",
								Label:  "Your annotation",
								Parser: core.ParseStringStripWhitespace,
							},
						},
					},
					Fn: func(_ *core.App, args core.ServiceFnArgs) core.ServiceResult {
						// todo: the parser will need to find and return the selected result
						selected_result := args.ArgList[0].Val.(core.Result)
						raw_annotation := args.ArgList[1].Val.(string)

						annotation := Annotation{
							Annotation:  raw_annotation,
							AnnotatedID: selected_result.ID,
						}
						result := core.MakeResult(BW_NS_ANNOTATION_ANNOTATION, annotation, core.UniqueID())

						// todo: annotating anything permanently saves the annotation and the thing being annotated.
						// the two are related.

						return core.MakeServiceResult(result)
					},
				},
			},
		},
	}
}

type BWProvider struct{}

func (bwp *BWProvider) ID() string {
	return "boardwalk"
}

func (bwp *BWProvider) ServiceList() []core.ServiceGroup {
	return provider()
}

func (bwp *BWProvider) ItemHandlerMap() map[reflect.Type][]core.Service {
	rv := map[reflect.Type][]core.Service{}
	return rv
}

func (bwp *BWProvider) Menu() []core.Menu {
	return []core.Menu{
		{Name: "File", MenuItemList: []core.MenuItem{
			{Name: "Browse Directory", ServiceID: SERVICE_ID_FS_BROWSE},
		}},
	}
}

var _ core.Provider = (*BWProvider)(nil)

func Provider(app *core.App) *BWProvider {
	return &BWProvider{}
}
