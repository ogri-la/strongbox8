package bw

import (
	"fmt"
	"os"
	"path/filepath"

	"bw/internal/core"
)

var (
	BW_NS_ANNOTATION_ANNOTATION = core.NewNS("bw", "annotation", "annotation")
	BW_NS_RESULT_LIST           = core.NewNS("bw", "core", "result-list")
	BW_NS_ERROR                 = core.NewNS("bw", "core", "error")
	BW_NS_STATE                 = core.NewNS("bw", "core", "state")
	BW_NS_SERVICE               = core.NewNS("bw", "core", "service")
	BW_NS_FS_FILE               = core.NewNS("bw", "fs", "file")
	BW_NS_FS_DIR                = core.NewNS("bw", "fs", "dir")
)

type Annotation struct {
	Annotation  string
	AnnotatedID string
}

func provider(_ *core.App) []core.Service {
	empty_result := core.FnResult{}

	return []core.Service{
		{
			NS: core.NS{Major: "bw", Minor: "state", Type: "service"},
			FnList: []core.Fn{
				{
					Label:     "print-state",
					Interface: core.FnInterface{},
					TheFn: func(app *core.App, _ core.FnArgs) core.FnResult {
						fmt.Println(core.QuickJSON(app.State()))
						return empty_result
					},
				},

				{
					Label: "reset-state",
					Interface: core.FnInterface{
						ArgDefList: []core.ArgDef{
							core.ConfirmYesArgDef(),
						},
					},
					TheFn: func(app *core.App, _ core.FnArgs) core.FnResult {
						app.ResetState()
						return empty_result
					},
				},
			},
		},

		{
			NS: core.NS{Major: "os", Minor: "fs", Type: "service"},
			FnList: []core.Fn{
				{
					Label: "list-files",
					Interface: core.FnInterface{
						ArgDefList: []core.ArgDef{
							core.DirArgDef(),
						},
					},
					TheFn: func(_ *core.App, args core.FnArgs) core.FnResult {
						path := args.ArgList[0].Val.(string)
						results := core.FnResult{}
						file_list, err := os.ReadDir(path)
						file_name_list := []core.Result{}
						for _, file := range file_list {
							ns := BW_NS_FS_FILE
							if file.IsDir() {
								ns = BW_NS_FS_DIR
							}
							file_name_list = append(file_name_list, core.NewResult(ns, file.Name(), core.UniqueID()))
						}
						if err != nil {
							results.Err = err
							return results
						}
						return core.FnResult{Result: file_name_list}
					},
				},
				{
					Label:       "list-files-recursive-flat",
					Description: "recursively visits each subdir in given dir, return a flat list of files and directories.",
					Interface: core.FnInterface{
						ArgDefList: []core.ArgDef{
							core.DirArgDef(),
						},
					},
					TheFn: func(_ *core.App, args core.FnArgs) core.FnResult {
						path := args.ArgList[0].Val.(string)
						results := []core.Result{}
						var readdir func(string) []core.Result
						readdir = func(root string) []core.Result {
							results = append(results, core.NewResult(BW_NS_FS_DIR, root, core.UniqueID()))
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
									results = append(results, core.NewResult(BW_NS_FS_FILE, full_path, core.UniqueID()))
								}
							}
							return results
						}
						readdir(path)
						return core.NewFnResult(results...)
					},
				},
			},
		},

		{
			NS: core.NS{Major: "bw", Minor: "annotation", Type: "service"},
			FnList: []core.Fn{
				{
					Label: "annotate",
					Interface: core.FnInterface{
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
					TheFn: func(_ *core.App, args core.FnArgs) core.FnResult {
						// todo: the parser will need to find and return the selected result
						selected_result := args.ArgList[0].Val.(core.Result)
						raw_annotation := args.ArgList[1].Val.(string)

						annotation := Annotation{
							Annotation:  raw_annotation,
							AnnotatedID: selected_result.ID,
						}
						result := core.NewResult(BW_NS_ANNOTATION_ANNOTATION, annotation, core.UniqueID())

						// todo: annotating anything permanently saves the annotation and the thing being annotated.
						// the two are related.

						return core.NewFnResult(result)
					},
				},
			},
		},
	}
}

func Start(app *core.App) {
	for _, service := range provider(app) {
		app.RegisterService(service)
	}

}
