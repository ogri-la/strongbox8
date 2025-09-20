package bw

// boardwalk provider of core functions.

import (
	"bw/core"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

var (
	BW_NS_ANNOTATION_ANNOTATION = core.MakeNS("bw", "annotation", "annotation")
	BW_NS_RESULT_LIST           = core.MakeNS("bw", "core", "result-list")
	BW_NS_ERROR                 = core.MakeNS("bw", "core", "error")
	BW_NS_STATE                 = core.MakeNS("bw", "core", "state")
	BW_NS_SERVICE               = core.MakeNS("bw", "core", "service")
	BW_NS_FS_FILE               = core.MakeNS("bw", "fs", "file")
	BW_NS_FS_DIR                = core.MakeNS("bw", "fs", "dir")
)

type Annotation struct {
	Annotation  string
	AnnotatedID string
}

func start_bw(app *core.App, args core.ServiceFnArgs) core.ServiceResult {
	fmt.Println("starting bw!")
	return core.ServiceResult{}
}

// func provider(_ *core.App) []core.Service {
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
	return []core.Menu{}
}

var _ core.Provider = (*BWProvider)(nil)

func Provider(app *core.App) *BWProvider {
	return &BWProvider{}
}
