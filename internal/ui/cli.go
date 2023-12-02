package ui

import (
	"bufio"
	"bw/internal/core"
	"errors"
	"fmt"
	"os"
	"strings"
)

func stderr(msg string) {
	os.Stderr.WriteString(msg)
}

func exit(msg string) {
	stderr(msg + "\n")
	os.Exit(1)
}

func die(err error, msg string) {
	if err != nil {
		exit(msg + ": " + err.Error())
	}
}

// "os/fs/list", "os/hardware/cpus",
// "github/orgs/list-repos", "github/users/list-repos"
func FullyQualifiedFnName(f core.Fn) string {
	if f.Service != nil {
		return fmt.Sprintf("%s/%s/%s", f.Service.NS.Major, f.Service.NS.Minor, f.Label)
	}
	return f.Label
}

func read_input(prompt string) (string, error) {
	stderr(prompt)
	reader := bufio.NewReader(os.Stdin)
	uin, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(uin), nil
}

func pick_key(menu [][]string) (string, error) {
	uin, err := read_input("> ")
	if err != nil {
		return "", err
	}
	uin = strings.TrimSpace(strings.ToLower(uin))
	for _, menu_item := range menu {
		if uin == menu_item[0] {
			return uin, nil
		}
	}
	return "", fmt.Errorf("unknown option '%s'", uin)
}

func pick_idx(num_items int) (int, error) {
	if num_items == 0 {
		return 0, errors.New("no items to choose from")
	}

	uin, err := read_input("> ")
	if err != nil {
		return 0, fmt.Errorf("failed to read user input: %w", err)
	}

	uin = strings.TrimSpace(uin)
	if uin == "" {
		return 0, fmt.Errorf("no selection made")
	}

	idx, err := core.StringToInt(uin)
	if err != nil {
		return 0, fmt.Errorf("failed to convert selection to an index: %w", err)
	}

	if idx > num_items || idx < 1 {
		return 0, errors.New("idx out of range: 1-" + fmt.Sprint(num_items))
	}

	return idx - 1, nil
}

func pick_args(fn core.Fn) (func() core.FnResult, error) {
	// prompt for each argument to argument interface

	fnargs := core.FnArgs{}

	num_args := len(fn.Interface.ArgDefList)

	if num_args > 0 {
		stderr(fmt.Sprintf("this function has %d argument(s):\n", num_args))
		for i, arg := range fn.Interface.ArgDefList {
			for {
				prompt := fmt.Sprintf("[arg %d] %s: ", i+1, arg.Label)
				if arg.Default != "" {
					prompt = fmt.Sprintf("[arg %d] (default %s) %s: ", i+1, arg.Default, arg.Label)
				}
				uin, err := read_input(prompt)
				if err != nil {
					stderr("failed to read input. try again or ctrl-c to quit.\n")
					continue
				}

				// use default value if user input was blank and default value available
				if uin == "" && arg.Default != "" {
					uin = arg.Default
				}

				err = nil
				if len(arg.ValidatorList) > 0 {
					for _, validator := range arg.ValidatorList {
						err = validator(uin)
						if err != nil {
							break
						}
					}

				} else {
					err = arg.Validator(uin)
				}

				if err != nil {
					stderr(fmt.Sprintf("input is invalid: %s\n", err))
					stderr("try again or ctrl-c to quit.\n")
					continue
				}

				// value has passed validation

				someval, err := arg.Parser(uin)
				if err != nil {
					stderr("error while coercing valid input: " + err.Error() + "\n")
					stderr("cannot recover, sorry.\n")
					break
				}

				fnargs.ArgList = append(fnargs.ArgList, core.Arg{Key: arg.ID, Val: someval})
				break
			}
		}
	}

	return func() core.FnResult {
		return core.CallServiceFnWithArgs(fn, fnargs)
	}, nil
}

func CLI(app *core.App) {

	menu := [][]string{
		{"l", "list functions"},
		{"q", "quit"},
	}

	for {
		// print menu
		for _, menu_item := range menu {
			fmt.Fprintf(os.Stderr, "[%s] %s\n", menu_item[0], menu_item[1])
		}

		// pick menu item
		menu_item, err := pick_key(menu)
		if err != nil {
			stderr(err.Error() + "\n")
			continue
		}

		if menu_item == "q" {
			break
		}

		// handle function list
		if menu_item == "l" {
			// print function list
			for i, fn := range app.FunctionList() {
				fmt.Fprintf(os.Stderr, "[%d] %s\n", i+1, FullyQualifiedFnName(fn))
			}

			// pick function
			idx, err := pick_idx(len(app.FunctionList()))
			if err != nil {
				stderr(err.Error() + "\n")
				continue
			}

			// pick function args
			fn := app.FunctionList()[idx]
			callable, err := pick_args(fn)
			if err != nil {
				die(err, "cannot proceed")
			}

			// call function with function args
			fnresult := callable()
			if fnresult.Err != nil {
				die(err, "failed executing function")
			}

			// print function call results
			if core.EmptyResult(fnresult.Result) {
				stderr("(no result)\n")
			} else {
				fmt.Println(core.QuickJSON(fnresult))
			}

			// push function call results into app state
			if !core.EmptyResult(fnresult.Result) {
				app.UpdateResultList(fnresult.Result)
			}

			// offer to pop it from result stack
			// offer to select new default result list
		}

		stderr("\n")
	}
	os.Exit(0)
}
