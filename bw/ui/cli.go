package ui

import (
	"bufio"
	"bw/core"

	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var KV_CLI_NO_COLOR = "bw.cli.NO_COLOR"

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
func FullyQualifiedFnName(f core.Service) string {
	if f.ServiceGroup != nil {
		return fmt.Sprintf("%s/%s/%s", f.ServiceGroup.NS.Major, f.ServiceGroup.NS.Minor, f.Label)
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

func pick_args(app *core.App, fn core.Service) (core.ServiceFnArgs, error) {
	// prompt for each argument to argument interface

	fnargs := core.ServiceFnArgs{}

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

				parsed_val, err := core.ParseArgDef(app, arg, uin)
				if err != nil {
					stderr(err.Error() + "\n")
					//stderr("cannot recover, sorry.\n")
					break
				}

				err = core.ValidateArgDef(arg, parsed_val)
				if err != nil {
					stderr(fmt.Sprintf("input is invalid: %s\n", err))
					stderr("try again or ctrl-c to quit.\n")
					continue
				}

				// value was successfully parsed and validated.

				fnargs.ArgList = append(fnargs.ArgList, core.KeyVal{Key: arg.ID, Val: parsed_val})
				break
			}
		}
	}

	return fnargs, nil
}

func (cli *CLIUI) Stop() {
	cli.WG.Done()
}

// starts the CLI loop
func (cli *CLIUI) Start() *sync.WaitGroup {
	app := cli.app

	menu := [][]string{
		{"l", "list functions"},
		{"g", "start GUI"},
		{"q", "quit"},
	}

	go func() {
		// cli main loop
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
				fnargs, err := pick_args(app, fn)
				if err != nil {
					die(err, "cannot proceed")
				}

				// call function with function args
				fnresult := core.CallServiceFnWithArgs(app, fn, fnargs)
				if fnresult.Err != nil {
					die(err, "failed executing function")
				}

				// print function call results
				if fnresult.IsEmpty() {
					if fnresult.Err != nil {
						fmt.Println(core.QuickJSON(fnresult.Err))
					} else {
						stderr("(no result)\n")
					}
				} else {
					fmt.Println(core.QuickJSON(fnresult))
				}

				// push function call results into app state
				app.AddResults(fnresult.Result...)

				// offer to pop it from result stack
				// offer to select new default result list
			}

			if menu_item == "g" {
				MakeGUI(cli.app, cli.WG).Start()
			}

			stderr("\n")
		}
		cli.Stop()
	}()
	return cli.WG
}

// ---

type CLITab struct{}

var _ core.UITab = (*CLITab)(nil)

// ---

type CLIUI struct {
	app      *core.App
	WG       *sync.WaitGroup
	Incoming core.UIEventChan
	Outgoing core.UIEventChan
}

func (cli *CLIUI) App() *core.App {
	return cli.app
}

func (cli *CLIUI) SetProp(key string, val any) {
	/// ...
}

func (cli *CLIUI) SetTitle(title string) {}
func (cli *CLIUI) Get() []core.UIEvent {
	ui_event := <-cli.Incoming
	return ui_event
}
func (cli *CLIUI) Put(event ...core.UIEvent) {
	cli.Outgoing <- event
}

func (cli *CLIUI) GetTab(title string) core.UITab {
	return &CLITab{}
}
func (cli *CLIUI) AddTab(title string, view core.ViewFilter) *sync.WaitGroup {
	slog.Warn("not implemented", "ui", "cli")
	var wg sync.WaitGroup
	return &wg
}

func (cli *CLIUI) AddRow(id_list ...string) {
	for _, id := range id_list {
		app_row := cli.app.GetResult(id)
		slog.Info("cli AddRow", "row", app_row, "implemented", false)
	}
}
func (cli *CLIUI) UpdateRow(id string) {
	app_row := cli.app.GetResult(id)
	slog.Info("cli UpdateRow", "row", app_row, "implemented", false)
}
func (cli *CLIUI) DeleteRow(id string) {
	app_row := cli.app.GetResult(id)
	slog.Info("cli DeleteRow", "row", app_row, "implemented", false)
}

func (tab *CLITab) SetColumnAttrs(col_attr_list []core.Column) {
	panic("not implemented")
}

var _ core.UI = (*CLIUI)(nil)

// ---

// configures app state for running a MakeCLI
func MakeCLI(app *core.App, wg *sync.WaitGroup) *CLIUI {
	wg.Add(1)
	cli := CLIUI{
		app: app,
		WG:  wg,
	}

	// suppress colours in command line interface
	no_color, present := os.LookupEnv("NO_COLOR")
	if present && len(no_color) > 0 && no_color[0] == '1' {
		app.State.SetKeyVal(KV_CLI_NO_COLOR, "1")
	}

	return &cli
}
