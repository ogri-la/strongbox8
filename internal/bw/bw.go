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
						fmt.Println(core.QuickJSON(app.State))
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

func Start(app *core.App) {
	for _, service := range provider(app) {
		app.RegisterService(service)
	}

}
