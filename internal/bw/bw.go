package bw

import (
	"fmt"
	"os"

	"bw/internal/core"
)

var BW_NS_ANNOTATION_ANNOTATION = core.NS{Major: "bw", Minor: "annotation", Type: "annotation"}

type Annotation struct {
	Annotation  string
	AnnotatedID string
}

func provider(app *core.App) []core.Service {
	empty_result := core.FnResult{}

	return []core.Service{
		{
			NS: core.NS{Major: "bw", Minor: "state", Type: "service"},
			FnList: []core.Fn{
				{
					Label:     "print-state",
					Interface: core.FnInterface{},
					TheFn: func(_ core.FnArgs) core.FnResult {
						fmt.Println(core.QuickJSON(app.State))
						return empty_result
					},
				},

				{
					Label: "reset-state",
					Interface: core.FnInterface{
						ArgDefList: []core.ArgDef{
							{
								ID:        "confirm",
								Label:     "Are you sure?",
								Default:   "yes",
								Validator: core.IsYesOrNoValidator,
								Coercer:   core.YesNoToBool,
							},
						},
					},
					TheFn: func(_ core.FnArgs) core.FnResult {
						core.ResetState()
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
							{
								ID:        "dir",
								Label:     "Directory",
								Validator: core.IsDirValidator,
								Coercer:   core.Identity,
							},
						},
					},
					TheFn: func(args core.FnArgs) core.FnResult {
						path := args.ArgList[0].Val.(string)
						results := core.FnResult{}
						file_list, err := os.ReadDir(path)
						file_name_list := []core.Result{}
						for _, file := range file_list {
							ns := core.BW_NS_FS_FILE
							if file.IsDir() {
								ns = core.BW_NS_FS_DIR
							}
							file_name_list = append(file_name_list, core.NewResult(ns, file.Name()))
						}
						if err != nil {
							results.Err = err
							return results
						}
						return core.FnResult{Result: core.NewResult(core.BW_NS_RESULT_LIST, file_name_list)}
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
								ID:        "selected",
								Label:     "Selected",
								Validator: core.HasResultValidator,
								Coercer:   core.FindResultByID,
							},
							{
								ID:        "annotation",
								Label:     "Your annotation",
								Validator: core.AlwaysTrueValidator,
								Coercer:   core.Identity,
							},
						},
					},
					TheFn: func(args core.FnArgs) core.FnResult {
						// todo: the coercer will need to find and return the selected result
						selected_result := args.ArgList[0].Val.(core.Result)
						raw_annotation := args.ArgList[1].Val.(string)

						annotation := core.NewResult(BW_NS_ANNOTATION_ANNOTATION, Annotation{
							Annotation:  raw_annotation,
							AnnotatedID: selected_result.ID,
						})

						// todo: annotating anything permanently saves the annotation and the thing being annotated.
						// the two are related.

						return core.FnResult{Result: annotation}
					},
				},
			},
		},
	}
}

func Start() {
	app := core.GetApp()
	for _, service := range provider(app) {
		app.RegisterService(service)
	}

}
