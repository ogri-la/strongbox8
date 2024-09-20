package ui

// we need to capture collapsed/expanded state
// selection state (already done)
// essentially: gui state
// when expanded, update list of those expanded
// when collapsed, same

import (
	"bw/internal/core"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"

	"github.com/visualfc/atk/tk"
)

const (
	key_gui_state          = "bw.ui.gui"
	key_details_pane_state = "bw.gui.details-pane"
	key_selected_results   = "bw.gui.selected-rows"
	key_expanded_rows      = "bw.gui.expanded_rows"
)

var NS_KEYVAL = core.NewNS("bw", "ui", "keyval")
var NS_VIEW = core.NewNS("bw", "ui", "view")
var NS_DUMMY_ROW = core.NewNS("bw", "ui", "dummyrow")

// ---

// a row to be inserted into a Tablelist
type Row struct {
	row      map[string]string
	children []Row
}

// a column in a View

// a function is derived from ViewFilter that filters the list of results in the app's state.
// we don't use a function directly because they can't be serialised.
type ViewFilter func(core.Result) bool

func NewViewFilter() ViewFilter {
	return func(core.Result) bool {
		return true
	}
}

// a View describes how to render a list of core.Result structs.
// it is stateful and sits somewhere between the main app state and the internal Tablelist state.
// the GUI may have many Views over it's data, each one represented by a tab (notebook in tcl/tk).
type View struct {
	Name        string
	ViewFilter  ViewFilter `json:"-"`
	DetailsOpen bool
	//SelectedRows []string
	//Columns    []Column
	//Rows       []Row
}

func NewView() *View {
	return &View{
		Name:       "untitled",
		ViewFilter: NewViewFilter(),
	}
}

// ---

type GUIMenuItem struct {
	name string
	fn   func()
}

type GUIMenu struct {
	name  string
	items []GUIMenuItem
}

type Window struct {
	*tk.Window
	tabber *tk.Notebook
}

// ---

type GUITab struct {
	gui   *GUIUI
	ref   *tk.PackLayout
	id    string
	title string
}

/*
	func (tab *GUITab) SetTitle(title string) {
		// all well and good, but ...
		tab.title = title

		slog.Warn("gui tab SetTitle broken")

		// ... we kind of expect the gui to be updated when we do this ...
		// however the tab bar breaks and disappears if we switch away.
			tabber := tab.gui.mw.tabber
			tabber.SetTab(*tab.ref, title)

			// should work, harmless
			//tk.Pack(tab.gui.mw.tabber)
			//tk.Pack(tabber, layout_attr("fill", "both"), layout_attr("expand", "1"))

			slog.Info("tabber text", "txt", tab.gui.mw.tabber.Text(0))
	}
*/
func (tab GUITab) AddRow()      {}
func (tab GUITab) AddManyRows() {}
func (tab GUITab) UpdateRow()   {}

var _ UITab = (*GUITab)(nil)

// ---

func dummy_row() []core.Result {
	return []core.Result{core.NewResult(NS_DUMMY_ROW, "", fmt.Sprintf("dummy-%v", core.UniqueID()))}
}

func OppositeVal(val string) (string, error) {
	switch val {
	case "true":
		return "false", nil
	case "false":
		return "true", nil
	case "open":
		return "close", nil
	case "close":
		return "open", nil
	case "opened":
		return "closed", nil
	case "closed":
		return "opened", nil
	case "show":
		return "hide", nil
	case "hide":
		return "show", nil
	}
	return "", errors.New("unsupported value: " + val)
}

func ToggleKeyVal(app *core.App, key string) string {
	slog.Debug("toggling keyval", "key", key)
	current := app.KeyVal(key)
	opposite, err := OppositeVal(current)
	if err != nil {
		panic("programming error, key val not set or unsupported: " + err.Error())
	}
	app.SetKeyVal(key, opposite)
	return opposite
}

/*
// just like `app.AddListener`,
// but wraps given `callback` function in `tk.Async` so it executes on main thread.
func AddGuiListener(app *core.App, callback func(old_state, new_state core.State)) {
	wrapped_fn := func(os, ns core.State) {
		tk.Async(func() {
			callback(os, ns)
		})
	}
	app.AddListener(wrapped_fn)
}
*/

func AddGuiListener(app *core.App, listener core.Listener2) {
	original_callback := listener.CallbackFn
	listener.CallbackFn = func(rl []core.Result) {
		tk.Async(func() {
			original_callback(rl)
		})
	}
	app.AddListener(listener)
}

/*
// convenience. function `lookup` extracts a value from state,
// and if this value is different between new and old states,
// calls function `callback` with the new value
// on the main thread.
func AddGuiListener2(app *core.App, lookup func(new_state core.State) any, callback func(someval any)) {
	wrapped_fn := func(os, ns core.State) {
		old := lookup(os)
		new := lookup(ns)
		if !reflect.DeepEqual(old, new) {
			callback(new)
		}
	}
	AddGuiListener(app, wrapped_fn)
}
*/

func donothing() {}

func build_theme_menu() []GUIMenuItem {
	theme_list := []GUIMenuItem{}
	for _, theme := range tk.TtkTheme.ThemeIdList() {
		if theme == "scid" {
			// something wrong with this one
			continue
		}
		theme := theme
		theme_list = append(theme_list, GUIMenuItem{name: theme, fn: func() {
			tk.TtkTheme.SetThemeId(theme)
		}})
	}
	return theme_list

}

func build_menu(gui *GUIUI, parent tk.Widget) *tk.Menu {
	app := gui.app
	menu_bar := tk.NewMenu(parent)
	menu_data := []GUIMenu{
		{
			name: "File",
			items: []GUIMenuItem{
				{name: "Open", fn: donothing},
				{name: "Exit", fn: gui.Stop},
			},
		},
		{
			name:  "View",
			items: build_theme_menu(),
		},
		{
			name: "Details",
			items: []GUIMenuItem{
				{name: "Toggle", fn: func() {
					ToggleKeyVal(app, key_details_pane_state)
				}},
			},
		},
		{
			name: "Preferences",
		},
		{
			name: "Help",
			items: []GUIMenuItem{
				{name: "Debug", fn: func() { fmt.Println(tk.MainInterp().EvalAsStringList(`wtree::wtree`)) }},
				{name: "About", fn: func() {
					title := "bw"
					heading := app.KeyVal("bw.app.name")
					version := app.KeyVal("bw.app.version")
					message := fmt.Sprintf(`version: %s
https://github.com/ogri-la/strongbox
AGPL v3`, version)
					tk.MessageBox(parent, title, heading, message, "ok", tk.MessageBoxIconInfo, tk.MessageBoxTypeOk)
				}},
			},
		},
	}

	for _, toplevel_item := range menu_data {
		submenu := menu_bar.AddNewSubMenu(toplevel_item.name)
		submenu.SetTearoff(false)
		for _, submenu_item := range toplevel_item.items {
			submenu_item_action := tk.NewAction(submenu_item.name)
			submenu_item_action.OnCommand(submenu_item.fn)
			submenu.AddAction(submenu_item_action)
		}
	}

	return menu_bar
}

func build_treeview_data(app *core.App, res_list []core.Result, col_list *[]string, col_set *map[string]bool) []Row {
	row_list := []Row{}

	for _, res := range res_list {
		if res.Item == nil { // dummy row, do not descend any further
			continue
		}

		row := Row{row: map[string]string{
			"id": res.ID,
			"ns": res.NS.String(),
		}}

		r := reflect.TypeOf((*core.ItemInfo)(nil)).Elem()
		if reflect.TypeOf(res.Item).Implements(r) {
			item_as_row := res.Item.(core.ItemInfo)

			// build up a list of known columns
			for _, col := range item_as_row.ItemKeys() {
				_, present := (*col_set)[col]
				if !present {
					(*col_list) = append((*col_list), col)
					(*col_set)[col] = true
				}
			}

			for key, val := range item_as_row.ItemMap() {
				row.row[key] = val
			}

			if item_as_row.ItemHasChildren() != core.ITEM_CHILDREN_LOAD_FALSE {

				// this is *not* the place to be modifying state (and causing a feedback loop).
				// an item with children has either been realised at this point or hasn't.
				// if it hasn't, it gets a dummy row that *will* trigger a state update.

				if res.ChildrenRealised {
					// children have already been visited. insert them now.
					children, _ := core.Children(app, res)
					row.children = append(row.children, build_treeview_data(app, children, col_list, col_set)...)
				} else {
					// children haven't been realised yet.
					// insert a dummy row indicating a row potentially has children.
					row.children = append(row.children, build_treeview_data(app, dummy_row(), col_list, col_set)...)
				}
			}
		}

		row_list = append(row_list, row)
	}

	return row_list
}

func layout_attr(key string, val any) *tk.LayoutAttr {
	return &tk.LayoutAttr{Key: key, Value: val}
}

func update_tablelist(app *core.App, result_list []core.Result, expanded_rows map[string]bool, tree *tk.Tablelist) {

	slog.Debug("update tablelist")

	col_list := []string{"id", "ns"}
	col_set := map[string]bool{"id": true, "ns": true} // urgh
	row_list := build_treeview_data(app, result_list, &col_list, &col_set)

	// when the number of columns has changed, rebuild them.
	// todo: this is naive, there may be other changes that result in the same number of columns, but this is fine for now.
	old_col_len := tree.ColumnCount()
	if old_col_len != len(col_list) {

		slog.Debug("num cols changed, rebuilding", "old", old_col_len, "new", len(col_list))

		tk_col_list := []*tk.TablelistColumn{}
		for _, col_title := range col_list {
			col := tk.NewTablelistColumn()
			col.Title = col_title
			tk_col_list = append(tk_col_list, col)
		}
		tree.DeleteAllColumns()
		tree.InsertColumnsEx(0, tk_col_list)
	}

	tree.DeleteAllItems()

	var insert_treeview_items func(int, []Row)
	insert_treeview_items = func(parent int, row_list []Row) {
		var parent_idx string
		if parent == -1 {
			// "root" is the invisible top-most element in a tree of items.
			// to insert items that appear to be top-level their parent must be 'root'.
			// to insert children of these top-level items, their parent must be 0.
			parent_idx = "root"
			parent = parent + 1 // -1 == 0, 0 == 1, 1 == 2 // this isn't tested.
		} else {
			parent_idx = strconv.Itoa(parent)
			parent += 1
		}
		for _, row := range row_list {
			vals := []string{}
			for _, col := range col_list {
				val, present := row.row[col]
				if !present {
					vals = append(vals, "")
				} else {
					vals = append(vals, val)
				}
			}
			cidx := 0 // todo: nfi. children-of-children?

			tli := tree.InsertChildListEx(parent_idx, cidx, [][]string{
				vals,
			})[0]

			result_id := vals[0]
			tli.SetName(result_id)

			if len(row.children) > 0 {
				insert_treeview_items(parent, row.children)

				_, is_expanded := expanded_rows[result_id]
				if !is_expanded && !tli.IsCollapsed() {
					//slog.Debug("calling collapse.", "tli", tli.IsCollapsed(), "children", len(row.children))
					tli.Collapse()
				}

			}
		}
	}
	insert_treeview_items(-1, row_list)

	/*
		tree.OnItemCollapsed(func(e *tk.Event) {
			println("8888888888888888888")
			fmt.Println(e.UserData)
		})

		tree.OnItemPopulate(func(e *tk.Event) {
			println("ppppppppppppppppp")
			fmt.Println(e.UserData)
		})
	*/

	// todo: any updates to the results collapses children again, assuming the result is still present.
	// this doesn't seem like a reasonable thing to do.
	//tree.CollapseAll()

}

// ---

func tablelist_widj(app *core.App, parent tk.Widget, view View) *tk.TablelistEx {

	//app.SetKeyVal(key_expanded_rows, map[string]bool{}) // todo: this is global, not per-widj
	//app.AddResults(core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows))

	// the '-name' values of the selected rows
	//app.SetKeyVal(key_selected_results, []string{}) // todo: this is global, not per-widj

	widj := tk.NewTablelistEx(parent)
	widj.LabelCommandSortByColumn()                       // column sort
	widj.LabelCommand2AddToSortColumns()                  // multi-column-sort
	widj.SetSelectMode(tk.TABLELIST_SELECT_MODE_EXTENDED) // click+drag to select
	widj.MovableColumns(true)                             // draggable columns
	/*
		tl.SetColumns([]tk.TablelistColumn{
			{Title: "foo"},
			{Title: "bar"},
		})

		// inserts two top-level items.
		// there are now two items in item list.
		tl.InsertChildList("root", 0, [][]string{
			{"boop", "baap"},
			{"baaa", "dddd"},
		})
		// inserts two more items under parent '0' (first item in list)
		// there are now four items in item list (0-3), items 1 and 2 are children.
		tl.InsertChildList(0, 0, [][]string{
			{"foo", "bar"},
			{"baz", "bop"},
		})

		// inserts an item under parent '1' (second item in list, which is a child of row 0)
		tl.InsertChildList(1, 0, [][]string{
			{"aaa", "aaa("},
		})
	*/

	// "when the core result list changes."
	/*
		AddGuiListener(app, core.Listener2{
			ID:        "changed rows listener",
			ReducerFn: view.ViewFilter,
			CallbackFn: func(new_results []core.Result) {

				// changes to state must include the keyvals.
				// should the listener instead be attached to State rather than State.Results or State.KeyVals?
				// - how does the reducer fn work then?
				// - should we ditch keyvals?

				expanded_rows := app.GetResult(key_expanded_rows).Item.(map[string]bool)

				// just top-level rows,
				// as children are included as necessary.
				result_list := core.FilterResultList(new_results, func(r core.Result) bool {
					return r.Parent == nil
				})

				app.AtomicUpdates(func() {
					update_tablelist(app, result_list, expanded_rows, widj.Tablelist)
				})

			},
		})
	*/
	// when a row is expanded
	widj.OnItemExpanded(func(tablelist_item *tk.TablelistItem) {

		slog.Debug("item expanded")

		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows := app.GetResult(key_expanded_rows)
		expanded_rows.Item.(map[string]bool)[key] = true
		app.SetResults(*expanded_rows)

		// update app state, fetching the children of the result
		res := app.FindResultByID(key)
		if core.EmptyResult(res) {
			slog.Debug("could not expand item, no results found", "item-key", key)
			return
		}
		core.Children(app, res)
	})

	// when a row is collapsed
	// disabled. this is called whenever children are realised.
	// using keyvals it didn't act so nutty (right?), but now it's really expensive
	// as an individual collapse event is sent for each
	// should we have a SetResultNoUpdate? SetResultBlind?

	widj.OnItemCollapsed(func(tablelist_item *tk.TablelistItem) {

		slog.Debug("item collapsed")

		// update app state, marking the row as expanded.
		key := tablelist_item.Name()
		expanded_rows_result := app.GetResult(key_expanded_rows)
		expanded_rows := expanded_rows_result.Item.(map[string]bool)
		delete(expanded_rows, key)
		expanded_rows_result.Item = expanded_rows
		app.SetResults(*expanded_rows_result)
	})

	// when rows are selected
	widj.OnSelectionChanged(func() {

		slog.Debug("selection changed")

		// fetch the associated 'name' attribute (result ID) of each selected row
		idx_list := widj.CurSelection2()
		selected_key_list := []string{}
		for _, idx := range idx_list {
			name := widj.RowCGet(idx, "-name")
			selected_key_list = append(selected_key_list, name)
		}

		// update the list of selected rows
		selected := core.NewResult(NS_KEYVAL, selected_key_list, key_selected_results)

		// show/hide the details widget
		rl := app.FilterResultList(func(r core.Result) bool {
			v, is_view := r.Item.(View)
			if is_view {
				fmt.Println("found view with name", v.Name)
			}
			return is_view && v.Name == view.Name
		})
		if len(rl) > 1 {
			panic(fmt.Sprintf("expected a single view, got %v", len(rl)))
		}
		r := rl[0]
		v := r.Item.(View)
		v.DetailsOpen = len(selected_key_list) > 0
		r.Item = v

		app.SetResults(selected, r)

		/*
			gui_state := app.KeyAnyVal(key_gui_state).(GUIState)
			for i, v := range gui_state.Views {
				if v.Name == view.Name {
					v.SelectedRows = selected_key_list
					v.DetailsOpen = len(selected_key_list) > 0
					gui_state.Views[i] = v
				} else {
					gui_state.Views[i] = v
				}
			}


			// update app state, setting the list of selected ids
			//app.SetKeyVal(key_selected_results, selected_key_list)
			app.SetKeyVal(key_gui_state, gui_state)
		*/

	})

	return widj
}

//

// todo: these accumulating parameters suggests a coupling problem
func details_widj(app *core.App, parent tk.Widget, pane *tk.TKPaned, view View, tablelist *tk.Tablelist) *tk.PackLayout {
	//app.SetKeyVal(key_details_pane_state, "opened")

	p := tk.NewPackLayout(parent, tk.SideTop)

	btn := tk.NewButton(parent, "close")
	p.AddWidget(btn)

	txt := tk.NewText(parent)
	txt.SetText("")
	p.AddWidget(txt)

	/*
		selected_rows_changed := func(s core.State) any {
			return s.KeyAnyVal(key_selected_results)
		}
		AddGuiListener2(app, selected_rows_changed, func(new_key_val any) {
			if new_key_val == nil {
				txt.SetText("")
				return
			}

			key_list := new_key_val.([]string)

			repr := ""
			for _, r := range app.FindResultByIDList(key_list) {
				repr += r.ID
			}

			txt.SetText(fmt.Sprintf("%v", repr))
		})
	*/

	// when 'close' button is clicked, toggle the 'details open' state of this view to the opposite of what it was.
	btn.OnCommand(func() {

		// todo: use app.GetResult somehow
		rl := app.FilterResultList(func(r core.Result) bool {
			v, is_view := r.Item.(View)
			return is_view && v.Name == view.Name
		})
		if len(rl) > 1 {
			panic(fmt.Sprintf("expected a single view, got %v", len(rl)))
		}

		r := rl[0]
		v := r.Item.(View)
		v.DetailsOpen = !v.DetailsOpen
		r.Item = v

		app.SetResults(r)
	})

	/*
		// on-state-change, find our view and return it's current 'details open' state
		details_pane_toggled := func(s core.State) any {
			gui_state := s.KeyAnyVal(key_gui_state).(*GUIState)
			for _, v := range gui_state.Views {
				if v.Name == view.Name {
					fmt.Println("found this view", v.Name, "open", v.DetailsOpen)
					return v.DetailsOpen
				}
			}
			return nil
		}

		AddGuiListener2(app, details_pane_toggled, func(val any) {
			hide, is_bool := val.(bool)
			if is_bool {
				pane.HidePane(1, hide)
			}
		})
	*/

	/*
		AddGuiListener(app, core.Listener2{
			ID: "view details listener",
			ReducerFn: func(s core.Result) bool {
				v, is_view := s.Item.(View)
				return is_view && v.Name == view.Name
			},
			CallbackFn: func(new_results []core.Result) {
				if len(new_results) != 1 {
					// view has been deleted?
					// future: delete listeners, or
					// create listeners in such a way that they are not tied to widgets.
					// will lead to fewer, more complex listeners.
					return
				}
				new_view := new_results[0].Item.(View)
				if new_view.DetailsOpen {
					fmt.Println("hiding")
				} else {
					fmt.Println("not hiding")
				}
				//pane.HidePane(1, !new_view.DetailsOpen)
			},
		})
	*/
	return p
}

func AddTab(gui *GUIUI, title string, view View) { //app *core.App, mw *Window, title string) {

	/*
	    ___________ ______
	   |tab|_|_|_|_|     x|
	   |           |      |
	   |  results  |detail|
	   |   view    |      |
	   |___________|______|

	*/

	paned := tk.NewTKPaned(gui.mw, tk.Horizontal)

	results_widj := tablelist_widj(gui.app, gui.mw, view)
	d_widj := details_widj(gui.app, gui.mw, paned, view, results_widj.Tablelist)

	paned.AddWidget(results_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"stretch", "always"})
	paned.AddWidget(d_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"width", "50p"})

	//paned.HidePane(1, !view.DetailsOpen)

	// ---

	tab_body := tk.NewVPackLayout(gui.mw)
	tab_body.AddWidgetEx(paned, tk.FillBoth, true, 0)

	gui.mw.tabber.AddTab(tab_body, title)

	gui.tab_idx[title] = tab_body.Id()

	//tk.Pack(paned, layout_attr("expand", 1), layout_attr("fill", "both"))

}

//

func NewWindow(gui *GUIUI) *Window {
	//app := gui.app

	mw := &Window{}
	mw.Window = tk.RootWindow()
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(gui, mw))
	mw.tabber = tk.NewNotebook(mw)

	vbox := tk.NewVPackLayout(mw)
	vbox.AddWidgetEx(mw.tabber, tk.FillBoth, true, 0)

	/*
		app.AddListener(core.Listener2{
			ID: "view listener",
			ReducerFn: func(r core.Result) bool {
				_, is_view := r.Item.(View)
				return is_view
			},
			CallbackFn: func(rl []core.Result) {

				// this listener is concerned about:
				// adding view tabs
				// destroy view tabs
				// moving view tabs
				// ...
				// it doesn't care about the internal state of the View itself,
				// that should be handled in some other listener.

				old_views := map[string]bool{}
				num_tabs := mw.tabber.TabCount()
				for i := 0; i < num_tabs; i++ {
					old_views[mw.tabber.Text(i)] = true
				}

				// future: the below doesn't preserve tab order.
				// future: it is possible to move the position of tabs without recreating them.
				// future: the below doesn't destroy tabs

				for _, r := range rl {
					view := r.Item.(View)
					_, is_present := old_views[view.Name]
					if !is_present {
						AddViewTab(app, mw, view)
					}
				}
			},
		})
	*/

	return mw
}

type GUIUI struct {
	tab_idx map[string]string

	inc UIEventChan
	out UIEventChan

	wg  *sync.WaitGroup
	app *core.App
	mw  *Window // intended to be the gui 'root', from where we can reach all gui elements
}

var _ UI = (*GUIUI)(nil)

func (gui *GUIUI) SetTitle(title string) {
}

func (gui *GUIUI) Get() UIEvent {
	return <-gui.inc
}

func (gui *GUIUI) Put(event UIEvent) {

}

func (gui *GUIUI) GetTab(title string) UITab {
	return &GUITab{}
}
func (gui *GUIUI) AddTab(title string) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	tk.Async(func() {
		AddTab(gui, title, *NewView())
		wg.Done()
	})
	return &wg
}

func (gui *GUIUI) SetActiveTab(title string) *sync.WaitGroup {
	slog.Info("setting active tab", "title", title)
	var wg sync.WaitGroup
	tab_id, exists := gui.tab_idx[title]
	if !exists {
		slog.Error("tab not found in index, cannot set active tab", "title", title)
		return &wg
	}
	widj, exists := tk.LookupWidget(tab_id)
	if !exists {
		slog.Error("widget with id not found. cannot set active tab", "title", title, "id", tab_id)
	}
	wg.Add(1)
	tk.Async(func() {
		gui.mw.tabber.SetCurrentTab(widj)
		wg.Done()
	})
	return &wg
}

func (gui *GUIUI) Stop() {
	gui.wg.Done()
	tk.Quit()
}

func (gui *GUIUI) Start() *sync.WaitGroup {
	var init sync.WaitGroup
	init.Add(1)

	app := gui.app

	/*
		default_view := NewView()
		default_view.Name = "all"
		default_view.ViewFilter = func(r core.Result) bool {
			// everything
			//return true
			return r.NS != NS_KEYVAL && r.NS != NS_VIEW
		}
	*/
	//gui_state.AddView(*default_view)
	//app.SetKeyVal(key_gui_state, gui_state)

	/*
		default_view_item := core.NewResult(NS_VIEW, *default_view, core.PrefixedUniqueId("view-"))
		expanded_rows_item := core.NewResult(NS_KEYVAL, map[string]bool{}, key_expanded_rows)
		app.SetResults(default_view_item, expanded_rows_item)
	*/
	/*
		vendor_view := NewView()
		vendor_view.Name = "vendor"
		vendor_view.ViewFilter = func(r core.Result) bool {
			// everything that isn't bw.*.*
			return r.NS.Major != "bw"
		}
		gui_state.AddView(vendor_view)
	*/

	// --- tcl/tk init

	go func() {

		tk.Init()
		tk.SetErrorHandle(core.PanicOnErr)

		// tablelist: https://www.nemethi.de
		// ttkthemes: https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
		slog.Info("tcl/tk", "tcl", tk.TclVersion(), "tk", tk.TkVersion())

		// --- configure path
		// todo: fix environment so this isn't necessary

		cwd, _ := os.Getwd()
		tk.SetAutoPath(filepath.Join(cwd, "tcl-tk"))
		_, err := tk.MainInterp().EvalAsStringList(`
# has no package
#source tcl-tk/widgettree/widgettree.tcl # disabled 2024-09-15: 'invalid command name "console"'

# $auto_path doesn't seem to work until searched

# tablelist/scaleutil is doing crazy fucking things
# like peering into running processes looking for and calling
# xfconf-query, gsettings, xrdb, xrandr etc.
# shortcircuit it's logic by giving it what it wants up front.
# we'll deal with it later.
set ::tk::scalingPct 100

package require Tablelist_tile 7.0`)
		core.PanicOnErr(err)

		// --- configure theme
		// todo: set as bw preference
		// todo: limit available themes
		// todo: dark theme
		// todo: main menu seems to resist styling

		default_theme := "clearlooks"
		tk.TtkTheme.SetThemeId(default_theme)

		// ---

		tk.MainLoop(func() {

			mw := NewWindow(gui)
			gui.mw = mw

			mw.SetTitle(app.KeyVal("bw.app.name"))
			mw.Center(nil)
			mw.ShowNormal()
			mw.OnClose(func() bool {
				gui.Stop()
				return true
			})
			init.Done() // the GUI isn't 'done', but we're done with init and ready to go.

			// listen for events from the app and tie them to UI methods
			//go Dispatch(gui)

			// populate widgets
			//app.KickState()          // an empty update
			//app.RealiseAllChildren() // an update that realises all non-lazy children
		})

	}()

	return &init
}

func GUI(app *core.App, wg *sync.WaitGroup) *GUIUI {
	wg.Add(1)

	return &GUIUI{
		tab_idx: make(map[string]string),
		inc:     make(chan UIEvent),
		out:     make(chan UIEvent),
		wg:      wg,
		app:     app,
	}
}
