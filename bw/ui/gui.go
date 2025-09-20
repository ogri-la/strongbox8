package ui

// we need to capture collapsed/expanded state
// selection state (already done)
// essentially: gui state
// when expanded, update list of those expanded
// when collapsed, same

import (
	"bw/core"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/visualfc/atk/tk"

	mapset "github.com/deckarep/golang-set/v2"
)

//go:embed tcl-tk/*
var TCLTK_FS embed.FS

// ---

const (
	key_gui_state          = "bw.ui.gui"
	key_details_pane_state = "bw.gui.details-pane"
	key_selected_results   = "bw.gui.selected-rows"
)

var NS_KEYVAL = core.MakeNS("bw", "ui", "keyval")
var NS_VIEW = core.MakeNS("bw", "ui", "view")
var NS_DUMMY_ROW = core.MakeNS("bw", "ui", "dummyrow")

var KV_GUI_ROW_MARKED_COLOUR = "bw.gui.row-marked-colour"
var GUI_ROW_MARKED_COLOUR = "#FAEBD7"

// ---

// a row to be inserted into a Tablelist
// note: gui.Row is specific to the gui for now and not general purpose
type Row struct {
	Row      map[string]string `json:"row"`
	Children []Row             `json:"children"`
}

// ---

type Window struct {
	*tk.Window
	tabber *tk.Notebook
}

// --- tablelist

type GUITablelist struct {
	tk.TablelistEx
	OnExpandFnList   []func(full_key string)
	OnCollapseFnList []func(full_key string)
}

func new_gui_tablelist(parent tk.Widget) *GUITablelist {
	tl, err := tk.NewTablelistEx(parent)
	if err != nil {
		panic("failed to create tablelist")
	}
	widj := &GUITablelist{
		TablelistEx:      *tl,
		OnExpandFnList:   []func(full_key string){},
		OnCollapseFnList: []func(full_key string){},
	}
	widj.LabelCommandSortByColumn()                       // column sort
	widj.LabelCommand2AddToSortColumns()                  // multi-column-sort
	widj.SetSelectMode(tk.TABLELIST_SELECT_MODE_EXTENDED) // click+drag to select
	widj.MovableColumns(true)                             // draggable columns

	// note: by shifting the callbacks into a single callback that calls many functions,
	// there is an opportunity to do finegrained toggling of callbacks,
	// especially as 'CollapseAll' is triggered once per-row-with-children.

	// when a row is expanded, call all the callbacks
	widj.OnItemExpanded(func(full_key string) {
		slog.Debug("item expanded", "full-key", full_key)
		for _, fn := range widj.OnExpandFnList {
			fn(full_key)
		}
	})

	// when a row is collapsed, call all the callbacks
	widj.OnItemCollapsed(func(full_key string) {
		slog.Debug("item collapsed", "full-key", full_key)
		for _, fn := range widj.OnCollapseFnList {
			fn(full_key)
		}
	})

	return widj
}

// --- tab

type GUITab struct {
	gui          *GUIUI         // reverse reference to the gui this tab belongs to. from here we can also get the app: tab.gui.app
	tab_body     *tk.PackLayout // every thing inside a tab
	paned        *tk.TKPaned    // first thing within a tab. encloses and splits the tablelist and details widj
	table_widj   *GUITablelist  // tablelist widj, left pane
	details_widj *DetailsWidj   // sidepanel, right pane, hidden by default
	GUIForm      *GUIForm       // the currently open form, if any

	title                string                 // name of tab
	filter               func(core.Result) bool // results in table are filtered by this
	column_list          []core.UIColumn        // available columns and their properties for this tab
	ItemFkeyIndex        map[string]string      // a mapping of app item IDs => tablelist 'full key'
	FkeyItemIndex        map[string]string      // a mapping of tablelist 'full key' => app item IDs
	IgnoreMissingParents bool                   // results with a parent that are missing get a parent_id of '-1' (top-level)
	expanded_rows        mapset.Set[string]     // 'open' rows
}

var _ core.UITab = (*GUITab)(nil)

func (tab *GUITab) OpenDetails() {
	tab.paned.HidePane(1, false)
}

func (tab *GUITab) CloseDetails() {
	tab.paned.HidePane(1, true)
}

func (tab *GUITab) expand_row(index string) {
	err := tab.table_widj.ExpandPartly1(index)
	if err != nil {
		slog.Error("failed to expand row", "index", index, "error", err)
		// swallow error
	} else {
		slog.Warn("expanding row", "i", index)
	}
}

func (tab *GUITab) ExpandRow(index string) {
	tab.gui.TkSync(func() {
		tab.expand_row(index)
	})
}

func highlight_row(tab *GUITab, index_list []string, colour string) {
	for _, index := range index_list {
		err := tab.table_widj.RowConfigure(index, map[string]string{"background": colour})
		if err != nil {
			slog.Error("highlighting row", "row", index, "colour", colour, "error", err)
		}
	}
}

func (tab *GUITab) HighlightManyRows(index_list []string, colour string) {
	tab.gui.TkSync(func() {
		highlight_row(tab, index_list, colour)
	})
}

func (tab *GUITab) HighlightRow(index string, colour string) {
	tab.HighlightManyRows([]string{index}, colour)
}

// higher level than `HighlightRow`, highlights all rows in `index_list` with the in the keyvals.
func (tab *GUITab) MarkRows(index_list []string) {
	val := tab.gui.App().State.GetKeyVal(KV_GUI_ROW_MARKED_COLOUR)
	if val == "" {
		// todo: consider putting KV_GUI_ROW_MARKED_COLOUR into kvstore on app start and making this a panic
		slog.Warn("keyval missing, using default", "keyval", KV_GUI_ROW_MARKED_COLOUR, "default", GUI_ROW_MARKED_COLOUR)
		val = GUI_ROW_MARKED_COLOUR
	}
	tab.HighlightManyRows(index_list, val)
}

func (tab *GUITab) SetTitle(title string) {
	tab.gui.TkSync(func() {
		tab.title = title
		tab.gui.mw.tabber.SetTab(tab.tab_body, title)
	})
}

// basic columns are created as rows are added to the table.
// each tab may specify it's own set of columns, each with their own attributes.
// tablelist columns not declared are created.
// tablelist columns present but not declared are hidden.
// tablelist column order inconsistent with declared are re-ordered.
func (tab *GUITab) SetColumnAttrs(column_list []core.UIColumn) {
	tab.gui.TkSync(func() {
		// first, find all columns to hide.
		// these are columns that are not present in the new idx.
		old_col_titles := mapset.NewSet[string]()
		for _, col := range tab.column_list {
			old_col_titles.Add(col.Title)
		}

		new_col_idx := map[string]core.UIColumn{}
		for _, col := range column_list {
			new_col_idx[col.Title] = col
		}

		// columns in old that are not in new
		new_col_titles := mapset.NewSetFromMapKeys(new_col_idx)
		cols_to_hide := old_col_titles.Difference(new_col_titles)

		// we now need to find the indicies of each of these old columns to hide.
		// some of these new columns may not exist yet!
		to_be_hidden := []int{}
		for pos, col := range tab.column_list {
			if cols_to_hide.Contains(col.Title) {
				slog.Debug("hiding column, column present in old but not new", "column", col, "pos", pos)
				to_be_hidden = append(to_be_hidden, pos)
			} else {
				// column present in both old and new,
				// however! col.Hidden attribute may have changed.
				// todo: there may be more attributes to diff in future
				new_col := new_col_idx[col.Title]
				if col.Hidden != new_col.Hidden && new_col.Hidden {
					slog.Debug("hiding column, Hidden attribute has changed", "old-column", col, "new-column", new_col, "pos", pos)
					to_be_hidden = append(to_be_hidden, pos)
				}
			}
		}
		tab.table_widj.ToggleColumnHide2(to_be_hidden)

		// next, find all columns to add.
		// ...

		// next, order the columns.
		// to be implemented: https://www.nemethi.de/tablelist/tablelistWidget.html#movecolumn
		new_col_pos_idx := map[string]int{}
		for i, col := range column_list {
			i := i
			new_col_pos_idx[col.Title] = i
		}
		for old_pos, old_col := range tab.column_list {
			new_pos, present := new_col_pos_idx[old_col.Title]
			if !present {
				// problem here? if cols are not present, `old_pos`
				slog.Debug("skipping col, not present in new", "col", old_col, "col-pos", old_pos)
				continue
			}
			if new_pos != old_pos {
				slog.Debug("moving col", "col", old_col, "col-pos", old_pos, "new-pos", new_pos)
				tab.table_widj.MoveColumn(old_pos, new_pos)
			} else {
				slog.Debug("NOT moving col", "col", old_col, "col-pos", old_pos, "new-pos", new_pos)
			}
		}

		// finally, set any attrs

		set_tablelist_cols(column_list, tab.table_widj.Tablelist)

		/*
			for i, col := range column_list {

				// urgh. this mapping between ui.Column and tk.TablelistColumn needs something.
				// https://www.nemethi.de/tablelist/tablelistWidget.html#col_options
				attrs := []tk.WidgetAttr{}

				if col.MaxWidth != 0 {
					attrs = append(attrs, tk.WidgetAttr{Key: "maxwidth", Value: col.MaxWidth})
				}
				if len(attrs) > 0 {
					tab.table_widj.ColumnConfigure(core.IntToString(i), attrs...)
				}
			}
		*/

		// set arrow column in case it has moved ...
		tab.table_widj.TreeColumn(0)

		tab.column_list = column_list
	})
}

// ---

func dummy_row() []core.Result {
	return []core.Result{core.MakeResult(NS_DUMMY_ROW, "", fmt.Sprintf("dummy-%v", core.UniqueID()))}
}

func AddGuiListener(app *core.App, listener core.Listener) {
	original_callback := listener.CallbackFn
	listener.CallbackFn = func(old_results, new_results []core.Result) {
		tk.Async(func() {
			original_callback(old_results, new_results)
		})
	}
	app.State.AddListener(listener)
}

func donothing() {}

// returns a list of menu items that switche between available themes
func build_theme_menu() []core.MenuItem {
	theme_list := []core.MenuItem{}

	// bw/ui/tcl-tk/ttk-themes
	bundled_themes := mapset.NewSet("black", "clearlooks", "plastik")

	for _, theme := range tk.TtkTheme.ThemeIdList() {
		if theme == "scid" {
			// something wrong with this one
			continue
		}
		if bundled_themes.Contains(theme) {
			theme_list = append(theme_list, core.MenuItem{Name: theme, Fn: func(app *core.App) {
				tk.TtkTheme.SetThemeId(theme)
			}})
		}
	}
	return theme_list

}

// returns the currently selected tab
func (gui *GUIUI) current_tab() *GUITab {
	idx := gui.mw.tabber.CurrentTabIndex()
	return gui.TabList[idx]
}

func (gui *GUIUI) GetCurrentTab() *GUITab {
	var tab *GUITab
	gui.TkSync(func() {
		tab = gui.current_tab()
	}).Wait()
	return tab
}

// problem: gui is initialised before providers.
// how to update menus? `gui.RebuildMenus` for now :(
func build_provider_services_menu(gui *GUIUI) []core.MenuItem {
	ret := []core.MenuItem{}
	for _, service := range gui.App().FunctionList() {
		ret = append(ret, core.MenuItem{
			Name: service.Label,
			Fn: func(app *core.App) {
				initial_data := []core.KeyVal{}
				gui.current_tab().OpenForm(service, initial_data)
			},
		})
	}

	return ret
}

// call the given `service` with `args`, opening a form for more inputs if necessary
func (gui *GUIUI) CallService(service core.Service, args core.ServiceFnArgs) {
	// todo: check the args required and only open form if necessary
	// todo: check if service.Fn is set
	tab := gui.current_tab()
	tab.OpenForm(service, args.ArgList)
	return
}

func build_menu(gui *GUIUI, parent tk.Widget) *tk.Menu {
	pre_menu_data := []core.Menu{
		{Name: "File"},
		{Name: "Edit"},
		{Name: "View"},
		{Name: "Provider Services", MenuItemList: build_provider_services_menu(gui)},
	}

	post_menu_data := []core.Menu{
		{Name: "File", MenuItemList: []core.MenuItem{
			core.MENU_SEP,
			{Name: "Quit", Fn: func(_ *core.App) { gui.Stop() }},
		}},
		{Name: "Edit", MenuItemList: build_theme_menu()},
		{Name: "Help", MenuItemList: []core.MenuItem{
			//{Name: "Debug", Fn: func() { fmt.Println(tk.MainInterp().EvalAsStringList(`wtree::wtree`)) }},
			{Name: "About", Fn: func(_ *core.App) {
				title := "bw"
				heading := gui.App().State.GetKeyVal("bw.app.name")
				version := gui.App().State.GetKeyVal("bw.app.version")
				message := fmt.Sprintf(`version: %s
https://github.com/ogri-la/strongbox
AGPL v3`, version)
				tk.MessageBox(parent, title, heading, message, "ok", tk.MessageBoxIconInfo, tk.MessageBoxTypeOk)
			}},
		}},
	}

	// 'sandwich' the provider menu between the default menu structure (File, Edit, View, etc),
	// and the items that should appear at the end of the menus ('Help', 'File->Quit', etc)
	final_menu := core.MergeMenus(pre_menu_data, gui.App().Menu)
	final_menu = core.MergeMenus(final_menu, post_menu_data)

	menu_bar := tk.NewMenu(parent)
	for _, menu := range final_menu {
		submenu := menu_bar.AddNewSubMenu(menu.Name)
		submenu.SetTearoff(false)
		for _, submenu_item := range menu.MenuItemList {
			if submenu_item.Name == core.MENU_SEP.Name {
				// add a separator instead
				submenu.AddSeparator()
				continue
			}

			if submenu_item.ServiceID != "" {
				// call the service directly
				service, err := gui.App().FindService(submenu_item.ServiceID)
				if err != nil {
					slog.Error("service with ID not found for submenu", "service-id", submenu_item.ServiceID, "submenu-name", submenu_item.Name)
					panic("programing error")
				}
				args := core.NewServiceFnArgs()
				submenu_item_action := tk.NewAction(submenu_item.Name)
				submenu_item_action.OnCommand(func() {
					gui.CallService(service, args)
				})
				submenu.AddAction(submenu_item_action)

			} else {
				// just call the callable
				submenu_item_action := tk.NewAction(submenu_item.Name)
				submenu_item_action.OnCommand(func() {
					submenu_item.Fn(gui.App())
				})
				submenu.AddAction(submenu_item_action)
			}

		}
	}

	return menu_bar
}

// https://www.nemethi.de/tablelist/tablelistWidget.html#insertchildlist
// `parentNodeIndex`: parent of the chunk of results to insert.
// - if this is a top level item the value is -1 ("root"), otherwise it's the index of the parent
// `childIndex` this is where in the list of the parent's children to insert the rows.
// - if the value is '0' the children will be inserted at the beginning
// - if the value is 'last' or equal to the number of children the parent already has, the chidlren be inserted at the end.
func _insert_treeview_items(tree *tk.Tablelist, parent string, cidx int, row_list []Row, col_list []core.UIColumn, item_idx map[string]string, fkey_idx map[string]string) int {

	if len(row_list) == 0 {
		panic("row list is empty")
	}

	if parent == "" {
		panic("parent_id is empty")
	}

	var parent_idx string
	if parent == "-1" {
		// "root" is the invisible top-most element in the tree of items.
		// to insert items that appear to be top-level their parent must be 'root'.
		// to insert children of these top-level items, their parent must be 0.
		parent_idx = "root"

	} else {
		parent_idx = parent
	}

	parent_list := [][]string{}
	for _, row := range row_list {
		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col.Title]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}
		parent_list = append(parent_list, single_row)
	}

	// insert the parents

	slog.Debug("inserting rows", "num", len(parent_list), "parent", parent, "parent-fk", parent_idx)

	full_key_list := tree.InsertChildList(parent_idx, cidx, parent_list)
	slog.Debug("results of inserting children", "fkl", full_key_list)

	for idx, row_full_key := range full_key_list {
		row_id := row_list[idx].Row["id"]
		item_idx[row_id] = row_full_key
		fkey_idx[row_full_key] = row_id
		slog.Debug("adding full key to index", "key", row_id, "val", row_full_key, "val2", row_list[idx])
	}

	return len(row_list)
}

// creates a list of rows and columns from the given `result_list`.
// does not consider children, does not recurse.
func build_treeview_row(result_list []core.Result, col_list []core.UIColumn) ([]Row, []core.UIColumn) {

	// if a list of columns `col_list` is given,
	// only those columns will be supported.
	// otherwise, all columns will be supported.
	fixed := len(col_list) > 0

	if !fixed {
		col_list = []core.UIColumn{
			{Title: "id"},
			{Title: "ns"},
		}
	}

	col_idx := mapset.NewSet[string]()
	for _, col := range col_list {
		col_idx.Add(col.Title)
	}

	row_list := []Row{}
	for _, result := range result_list {
		if result.Item == nil { // dummy row/unrealised row, skip
			continue
		}

		row := Row{Row: map[string]string{
			"id": result.ID,
			"ns": result.NS.String(),
		}}

		if core.HasItemInfo(result.Item) {
			item := result.Item.(core.ItemInfo)

			if !fixed {
				// append any missing columns
				for _, col := range item.ItemKeys() {
					if !col_idx.Contains(col) {
						col_list = append(col_list, core.UIColumn{Title: col})
						col_idx.Add(col)
					}
				}
			}

			// build up the row
			for col, val := range item.ItemMap() {
				if col_idx.Contains(col) {
					row.Row[col] = val
					//if val == core.ITEM_LOOKUP_VALUE {
					//	row.Row[col] = item.ItemValueLookup(col, gui)
					//}
				}
			}
		}
		row_list = append(row_list, row)
	}

	return row_list, col_list
}

// returns the list of known columns.
// must be called within a TkSync block.
func known_columns(tree *tk.Tablelist) []string {
	return tree.ColumnNames(tree.ColumnCount())
}

// add each column in `new_col_list` to Tablelist `tree`,
// unless column exists.
func set_tablelist_cols(new_col_list []core.UIColumn, tree *tk.Tablelist) {
	kc := known_columns(tree)
	known_cols := map[string]bool{}
	for _, title := range kc {
		known_cols[title] = true
	}

	tk_col_list := []*tk.TablelistColumn{}
	for _, col := range new_col_list {
		_, present := known_cols[col.Title]
		if present {
			slog.Debug("column exists, skipping", "col", col.Title)
			continue
		}

		slog.Debug("column not found, creating", "col", col.Title)

		tk_col := tk.NewTablelistColumn()
		tk_col.Title = col.Title

		// map any attributes :(
		tk_col.MaxWidth = col.MaxWidth

		tk_col_list = append(tk_col_list, tk_col)
		known_cols[col.Title] = true
	}

	if len(tk_col_list) > 0 {
		tree.InsertColumnsEx(len(kc), tk_col_list)
	}
}

// ---

func details_widj(gui *GUIUI, parent tk.Widget, onclosefn func(), body tk.Widget) *DetailsWidj {
	p := tk.NewPackLayout(parent, tk.SideTop)

	btn := tk.NewButton(parent, "close")
	btn.OnCommand(onclosefn)
	p.AddWidget(btn)

	if body != nil {
		p.AddWidget(body)
	}

	return &DetailsWidj{
		p,
	}
}

type DetailsWidj struct {
	*tk.PackLayout
}

func MakeSearchBar(gui *GUIUI, parent tk.Widget) *tk.PackLayout {
	layout := tk.NewHPackLayout(parent)

	txt := tk.NewLabel(layout, "Search:")
	entry := tk.NewEntry(layout)

	layout.AddWidget(txt)
	layout.AddWidget(entry)

	// global event bind for 'ctrl-f', but only apply to entry widget in current tab
	tk.BindEvent(".", "<Control-f>", func(e *tk.Event) {
		prefix := gui.mw.tabber.CurrentTab().Id()
		if strings.HasPrefix(entry.Id(), prefix) {
			slog.Debug("setting focus", "sb", entry.Id())
			entry.SetFocus()
		}
	})

	var (
		mu             sync.Mutex
		debounce_timer *time.Timer
		delay          = 300 * time.Millisecond
	)

	// widget event bind for keypresses
	entry.BindKeyEvent(func(e *tk.KeyEvent) {

		mu.Lock()
		defer mu.Unlock()

		// cancel previous timer if it exists
		if debounce_timer != nil {
			debounce_timer.Stop()
		}

		// start a new timer. runs only after 300ms of no keypresses
		debounce_timer = time.AfterFunc(delay, func() {

			ctab := gui.mw.tabber.CurrentTab()
			prefix := ctab.Id()
			if strings.HasPrefix(entry.Id(), prefix) {
				//slog.Info("got event", "entry", entry.Id(), "e", e, "r", e.KeyRune, "t", e.KeyText, "full", entry.Text())

				guitab := gui.current_tab()
				table := guitab.table_widj

				gui.TkSync(func() {
					text := entry.Text()
					cell_value_list := table.GetCells("0,1", "last,1", tk.TABLELIST_ROW_STATE_ALL)

					hide := map[string]string{"hide": "true"}
					no_hide := map[string]string{"hide": "false"}

					for i, cell_value := range cell_value_list {
						is := core.IntToString(i)
						if strings.HasPrefix(strings.ToLower(cell_value), text) {
							//slog.Info("NOT hiding row", "i", i, "text", text, "name", nom)
							table.RowConfigure(is, no_hide)
						} else {
							table.RowConfigure(is, hide)
						}

					}
				}).Wait()
			}
		})
	})

	return layout
}

func AddTab(gui *GUIUI, title string, viewfn core.ViewFilter) {

	/*
	    ___________________
	   |tab body___________|
	   |tab|_|_|_|_|     x||
	   |           |      ||
	   |  results  |detail||
	   |   view    |      ||
	   |___________|______||
	   |___________________|

	*/

	// parents of both of these is gui.mw ...
	tab_body := tk.NewVPackLayout(gui.mw.tabber)

	// ---

	search_bar := MakeSearchBar(gui, tab_body)
	tab_body.AddWidgetEx(search_bar, tk.FillX, false, 0) // fill space horizontally (not vertically), do not expand

	// ---

	paned := tk.NewTKPaned(tab_body, tk.Horizontal)

	table_widj := new_gui_tablelist(paned)
	table_id := table_widj.Id()
	gui.widget_ref[table_id] = table_widj

	d_widj := details_widj(gui, paned, func() {
		paned.HidePane(1, true)
	}, nil)

	paned.AddWidget(table_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"stretch", "always"})

	paned.AddWidget(d_widj, &tk.WidgetAttr{"minsize", "50p"}, &tk.WidgetAttr{"width", "50p"})
	paned.HidePane(1, true)

	tab_body.AddWidgetEx(paned, tk.FillBoth, true, 0)

	gui.mw.tabber.AddTab(tab_body, title)

	tab := &GUITab{
		gui:           gui,
		tab_body:      tab_body,
		paned:         paned,
		table_widj:    table_widj,
		details_widj:  d_widj,
		title:         title,
		filter:        viewfn,
		ItemFkeyIndex: map[string]string{},
		FkeyItemIndex: map[string]string{},
		expanded_rows: mapset.NewSet[string](),
	}
	gui.TabList = append(gui.TabList, tab)
	gui.tab_idx[title] = tab_body.Id()

	// ---

	// right clicking a tablelist row
	err := table_widj.BindEvent("<Button-3>", func(e *tk.Event) {

		widj := table_widj

		// select the row on right click if no rows selected
		// note: this didn't end up working, it would always be inaccurate
		//idx := widj.NearestCell(e.PosX, e.PosY)
		//widj.SelectionAnchor(idx)
		//widj.SelectionAnchor(fmt.Sprintf("@%v,%v", e.GlobalPosX, e.GlobalPosY)) //  e.PosX, e.PosY))
		//widj.SelectionAnchor(fmt.Sprintf("@%v,%v", e.PosX, e.PosY))
		//idx := widj.NearestCell(e.PosX, e.PosY)
		//idx := widj.Nearest(e.PosY)
		//widj.SelectionSet(fmt.Sprintf("%v", idx))
		//widj.SelectionSet(fmt.Sprintf("@%v,%v", e.PosX, e.PosY))

		id_list := widj.CurSelection3() // these are simple numerical indicies

		// for each row index, find the full key, find the result in state, add it to a list
		res_list := []*core.Result{}
		for _, id := range id_list {
			idstr := core.IntToString(id)
			fkey := widj.GetFullKeys2(idstr)

			item_id := tab.FkeyItemIndex[fkey]
			result := gui.App().GetResult(item_id)
			res_list = append(res_list, result)
		}

		// group the `Result` list by the type of it's `.Item`
		grp := core.GroupBy2(res_list, func(r *core.Result) reflect.Type {
			return reflect.TypeOf(r.Item)
		})

		context_menu := tk.NewMenu(widj.Tablelist)
		context_menu.SetTearoff(false)

		// for each group, find the associated services by checking the `app.TypeMap`.
		// if many of a type were selected, check for _slice_ type associations.
		// each group gets it's own header to differentiate it from other types
		for t, grouped := range grp {
			key := t
			if len(grouped) > 1 {
				key = reflect.SliceOf(t) // T => []T, File{} => []File{}
			}
			service_list, present := gui.App().TypeMap[key]

			slog.Debug("got grouped items", "len", len(grouped), "type-map-key", key, "present?", present)

			if len(service_list) == 0 {
				// no services available for this type
				continue
			}

			// differentiate between groups with a header.
			a := tk.NewAction(fmt.Sprintf("%v (%v items)", t, len(grouped))) // "bw.File (2 items)"
			context_menu.AddActionWithState(a, "disabled")

			// clicking a service calls the function directly,
			// but only if the service accepts a single argument.
			for _, service := range service_list {
				action := tk.NewAction(service.Label)
				action.OnCommand(func() {
					if service.Fn == nil {
						slog.Warn("service registered for context menu but not implemented", "service", service)
					}

					if len(service.Interface.ArgDefList) == 1 {
						// we can call the service directly
						if len(grouped) == 1 {
							// call the service with the single item rather a list of items.
							service.Fn(gui.App(), core.MakeServiceFnArgs("selected", grouped[0]))
						} else {
							service.Fn(gui.App(), core.MakeServiceFnArgs("selected", grouped))
						}
						return
					}

					// service requires more inputs
					// open a form for user to fill out
					tab := gui.current_tab()

					// every service that accepts a bundle of data
					args := core.MakeServiceFnArgs("selected", grouped).ArgList
					tab.OpenForm(service, args)
				})
				context_menu.AddAction(action)
			}
			tk.PopupMenu(context_menu, e.GlobalPosX, e.GlobalPosY)
		}
	})
	if err != nil {
		panic(fmt.Sprintf("error! %v", err))
	}

}

func sort_insertion_order(result_list []core.Result) []core.Result {

	// we have a blob of results here.
	// we need to sort results into insertion order.
	// all parents must be added before children can be added.

	// group results by their parent.ID
	// assumes the top-level results have a ParentID of "" (State.Root.ID)
	all_idx := mapset.NewSet[string]()
	child_idx := map[string][]core.Result{} // {parent.ID => [child, child, ...], ...}
	for _, r := range result_list {
		child_idx[r.ParentID] = append(child_idx[r.ParentID], r)
		all_idx.Add(r.ID)
	}

	// these are parents that were _not_ found in `result_list`.
	// children with these parents come first
	roots := []string{}
	for _, r := range result_list {
		if !all_idx.Contains(r.ParentID) {
			roots = append(roots, r.ParentID)
		}
	}

	queue := roots
	new_results_ordered := []core.Result{}
	visited := mapset.NewSet[string]() // prevent cycles/duplicates. only an issue if duplicate children exist

	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		for _, child := range child_idx[parentID] {
			if !visited.Contains(child.ID) {
				new_results_ordered = append(new_results_ordered, child)
				queue = append(queue, child.ID)
				visited.Add(child.ID)
			}
		}
	}

	return new_results_ordered
}

func AddRowToTree(gui *GUIUI, tab *GUITab, id_list ...string) {

	gui.TkSync(func() {

		tree := tab.table_widj

		// id_list is in insertion order.
		// however! the list needs to be grouped by parent_id
		// and inserted in batches
		// as the next batch may depend on the ID of a result inserted in the previous batch.

		// top-level results (no parent ID)
		// that fail the tab's view filter fn,
		// are excluded from being displayed,
		// including their children (obviously)
		excluded := map[string]bool{}

		result_list := []core.Result{}
		for _, id := range id_list {
			result := gui.App().GetResult(id)
			if result == nil {
				slog.Error("GUI, result with id not found in app", "id", id)
				panic("")
			}

			if result.ID != id {
				slog.Error("GUI, result with id != given id", "id", id, "result.ID", result.ID)
				panic("")
			}

			if !tab.filter(*result) {
				slog.Debug("row excluded and won't be present in tab", "tab", tab.title, "id", id, "result-ns", result.NS)
				excluded[result.ID] = true
				continue
			}

			result_list = append(result_list, *result)
		}

		// ignoring missing parents turns out to be a shorthand for 'no grouping'.
		// this is a bit of a wart and should be cleaned up
		if !tab.IgnoreMissingParents {
			result_list = sort_insertion_order(result_list)
		}

		no_parent := "-1"
		bunch_list := core.Bunch(result_list, func(r core.Result) any {
			return r.ParentID
		})

		// figure out which parent to insert each bunch of results under
		for _, bunch := range bunch_list {
			first_row := bunch[0]
			var parent_id string
			var present bool
			if first_row.ParentID == "" {
				// easy, no parent to begin with,
				// use top-level.
				parent_id = no_parent
			} else {
				// has a parent, but
				// parent may have been excluded during filtering of results above,
				// or we may have a code error.

				// if parent has been filtered out,
				is_excluded := excluded[first_row.ParentID] // warning: bool default value is being used here for rows not found
				if is_excluded {
					// parent has been excluded.
					// this means all children (this bunch) are also excluded,
					// unless IgnoreMissingParents is true.
					if tab.IgnoreMissingParents {
						slog.Debug("parent has been excluded and this bunch of results will become top-level")
					} else {
						slog.Debug("parent has been excluded so this bunch of results will also be excluded")
						continue
					}
				}

				parent_id, present = tab.ItemFkeyIndex[first_row.ParentID]
				if !present {
					// parent not found!
					// this is to be expected if we're excluding results, but
					// unless IgnoreMissingParents is explicitly set to true,
					// this is a programming error.
					if tab.IgnoreMissingParents {
						// all good, just set parent to the top level.
						// note: using IgnoreMissingParents may mask programming problems.
						parent_id = no_parent
					} else {
						// no good. parent not found and IgnoreMissingParents is false. die.
						msg := "parent not found in index. it hasn't been inserted yet or has been excluded without IgnoreMissingParents set to 'true'"
						id := first_row.ID
						parent := first_row.ParentID
						slog.Error(msg, "id", id, "parent", parent, "num-exclusions", len(excluded), "ignore-missing-parents", tab.IgnoreMissingParents, "parent-was-excluded", is_excluded)
						panic("programming error")
					}
				}
			}

			// ---

			row_list, col_list := build_treeview_row(bunch, tab.column_list)

			// todo: col_idx and col_list should be updated in-place.
			tab.column_list = col_list

			set_tablelist_cols(col_list, tree.Tablelist)

			child_idx := 0 // where in list of children to add this child (if is child)
			_insert_treeview_items(tree.Tablelist, parent_id, child_idx, row_list, col_list, tab.ItemFkeyIndex, tab.FkeyItemIndex)

			// expand certain children if they've been tagged
			for _, result := range bunch {
				if result.Tags.Contains(core.TAG_SHOW_CHILDREN) {

					// don't expand if the item is marked as having no children or is lazily loaded
					ii, has_ii := result.Item.(core.ItemInfo)
					if has_ii && ii.ItemHasChildren() != core.ITEM_CHILDREN_LOAD_TRUE {
						continue
					}

					full_key := tab.ItemFkeyIndex[result.ID]
					tab.expanded_rows.Add(full_key)
				}
			}
		}

		tree.CollapseAll()
		to_expand := tab.expanded_rows.ToSlice()
		slog.Debug("partly expanding", "fkey", to_expand)
		tree.ExpandPartly2(to_expand)
	})

}

// when a row is ADDED, it is because the row doesn't exist to be modified in-place.
// as such, any new columns must be added and
// the row must find all of it's parents and children and
// the row must be inserted in the right place.
func (gui *GUIUI) AddRow(id_list ...string) {
	for _, tab := range gui.TabList {
		AddRowToTree(gui, tab, id_list...)
	}
}

// add a function to be executed on the UI thread and processed _synchronously_
func (gui *GUIUI) TkSync(fn func()) *sync.WaitGroup {
	wg := sync.WaitGroup{}
	wg.Add(1)
	wrap := func() {
		defer wg.Done()
		fn()
	}
	gui.tk_chan <- wrap
	return &wg
}

// updates a single row, does not affect children
func UpdateRowInTree(gui *GUIUI, tab *GUITab, id string) {
	gui.TkSync(func() {
		slog.Debug("gui.UpdateRow UPDATING ROW", "id", id)
		if len(tab.ItemFkeyIndex) == 0 {
			// can happen when a new tab is created and an update comes in for a result present in another tab.
			slog.Debug("gui tab failed to update row, tab has no rows to update yet", "tab", tab.title, "id", id)
			return
		}

		full_key, present := tab.ItemFkeyIndex[id]
		if !present {
			// received an update for a row that isn't present in the table.
			// this is normal when the table widget doesn't contain all results.
			slog.Debug("gui failed to update row, row full key not found in row index", "id", id)
			return
		}

		result := gui.App().GetResult(id)
		if result == nil {
			// received an update for a result that no longer exists?
			slog.Error("gui tab failed to update row, result with id does not exist", "id", id)
			panic("")
		}

		tree := tab.table_widj

		// TODO: revisit, works but expensive. necessary if we want row updates to change cols
		if false {
			set_tablelist_cols(tab.column_list, tree.Tablelist)
		}

		// when a row is updated, just the row is updated, the children are not modified.
		row_list, col_list := build_treeview_row([]core.Result{*result}, tab.column_list)

		// todo: update many rows at once?
		if len(row_list) != 1 {
			slog.Error("gui failed to update row, result of building row should be precisely 1", "row-list", row_list)
			panic("")
		}
		row := row_list[0]

		// because updating rows can't introduce new columns,
		// add padding if a value for that column doesn't exist.
		single_row := []string{}
		for _, col := range col_list {
			val, present := row.Row[col.Title]
			if !present {
				single_row = append(single_row, "")
			} else {
				single_row = append(single_row, val)
			}
		}

		tree.Tablelist.RowConfigureText(full_key, single_row)

		// not sure where this is going or if it's efficient,
		// but checking for a Result.Tag and modifying a row seems ok?

		if result.Tags.Contains(core.TAG_HAS_UPDATE) {
			colour := gui.App().State.GetKeyVal(KV_GUI_ROW_MARKED_COLOUR)
			highlight_row(tab, []string{full_key}, colour)
		}

		if result.Tags.Contains(core.TAG_SHOW_CHILDREN) {
			tab.expand_row(full_key)
		}
	})
}

func (gui *GUIUI) UpdateRow(id string) {
	for _, tab := range gui.TabList {
		UpdateRowInTree(gui, tab, id)
	}
}

func delete_row_in_tree(gui *GUIUI, tab *GUITab, id string) {
	fullkey := tab.ItemFkeyIndex[id]
	if fullkey != "" {
		gui.TkSync(func() {
			tab.table_widj.Delete2(fullkey)
		})
		tab.expanded_rows.Remove(fullkey)
	}
}

func (gui *GUIUI) DeleteRow(id string) {
	for _, tab := range gui.TabList {
		delete_row_in_tree(gui, tab, id)
	}
}

// todo: hack.
// causes the menu to be rebuilt.
// if the menu is using app state and that state changes, call this to refresh menu.
func (gui *GUIUI) RebuildMenu() {
	gui.TkSync(func() {
		gui.mw.SetMenu(build_menu(gui, gui.mw))
	})
}

// ---

// creates a form for the given `service`,
// binds the given `initial_data`, if any,
// opens the details pane,
// renders a GUI version of the form.
func (tab *GUITab) OpenForm(service core.Service, initial_data []core.KeyVal) *sync.WaitGroup {
	form := core.MakeForm(service)
	form.Update(initial_data)

	return tab.gui.TkSync(func() {
		// destroy previous details before opening a new set
		children := tab.details_widj.Children()
		if len(children) > 0 {
			for _, c := range children {
				tk.DestroyWidget(c)
			}
		}

		tab.OpenDetails()
		parent := tab.details_widj
		tab.GUIForm = RenderServiceForm(tab.gui, parent, form)
	})
}

// closes the details widj, empties it's body, removes any form, etc
func (tab *GUITab) close_form() {
	tab.CloseDetails()
	tab.GUIForm = nil
}

func (tab *GUITab) CloseForm() *sync.WaitGroup {
	return tab.gui.TkSync(func() {
		tab.close_form()
	})
}

//

func NewWindow(gui *GUIUI) *Window {
	mw := &Window{}
	mw.Window = tk.RootWindow()
	mw.ResizeN(800, 600)
	mw.SetMenu(build_menu(gui, mw))
	mw.tabber = tk.NewNotebook(mw)

	vbox := tk.NewVPackLayout(mw)
	vbox.AddWidgetEx(mw.tabber, tk.FillBoth, true, 0)

	return mw
}

type GUIUI struct {
	app *core.App // reverse reference to the app this gui belongs to

	TabList    []*GUITab
	tab_idx    map[string]string
	widget_ref map[string]any

	inc     core.UIEventChan // events coming from the core app
	out     core.UIEventChan // events going to the core app
	tk_chan chan func()      // functions to be executed on the tk channel

	WG *sync.WaitGroup

	mw *Window // intended to be the gui 'root', from where we can reach all gui elements
}

func (gui *GUIUI) App() *core.App {
	return gui.app
}

var _ core.UI = (*GUIUI)(nil)

func (gui *GUIUI) SetTitle(title string) {
	panic("not implemented")
}

// blocking pull of a single gui event from the incoming stream of events.
func (gui *GUIUI) Get() []core.UIEvent {
	val := <-gui.inc
	slog.Debug("gui.GET called, fetching UI event from app", "val", val)
	return val
}

// put `event` on to the stream of gui events to process
func (gui *GUIUI) Put(event ...core.UIEvent) {
	slog.Debug("gui.PUT called, adding UI event from app", "event", event)
	gui.inc <- event
}

func (gui *GUIUI) GetTab(title string) core.UITab {
	for _, tab := range gui.TabList {
		if title == tab.title {
			return tab
		}
	}
	return nil
}

func (gui *GUIUI) AddTab(title string, filter core.ViewFilter) *sync.WaitGroup {
	return gui.TkSync(func() {
		AddTab(gui, title, filter)
	})
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

/*
# example code for adding hyperlinks into tablelist. untested

package require tablelist

# Create a tablelist widget
tablelist::tablelist .tl -columns {0 "Column 1" 0 "Column 2"} -height 10
pack .tl -fill both -expand true

# Create a hyperlink-like button
set hyperlink [button .link -text "Visit Website" \
    -relief flat -highlightthickness 0 -background white \
    -activebackground white -foreground blue \
    -activeforeground purple -cursor hand2 \
    -command {exec xdg-open https://example.com}]

# Add the button to a cell
.tl insert end [list "Row 1" $hyperlink]

# Embed the hyperlink-like button into the cell
.tl cellconfigure 0 1 -window $hyperlink
*/

func (gui *GUIUI) Stop() {
	slog.Warn("stopping gui")
	tk.Quit()
	// tk.Quit() is Async and a 5ms pause actually seems to prevent:
	//   'panic: error: script: "destroy .", error: "invalid command name \"destroy\""'
	time.Sleep(5 * time.Millisecond)
	gui.WG.Done()
}

// 'install' tablelist and the other tcl/tk scripts in to the app's XDG_DATA dir,
// then add that dir to the autopath.
// we do this because tcl/tk can't navigate a virtual FS :(
func install_scripts(src fs.FS, dst_root string) (string, error) {
	src_root := "."
	return filepath.Join(dst_root, "tcl-tk"), fs.WalkDir(src, src_root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel_path, err := filepath.Rel(src_root, path) // "/path/to/fs/." => "/path/to/fs/./tcl-tk"
		if err != nil {
			return err
		}
		target_path := filepath.Join(dst_root, rel_path)

		if d.IsDir() {
			return os.MkdirAll(target_path, 0755)
		}

		src_file, err := src.Open(path)
		if err != nil {
			return err
		}
		defer src_file.Close()

		dst_file, err := os.Create(target_path)
		if err != nil {
			return err
		}
		defer dst_file.Close()

		_, err = io.Copy(dst_file, src_file)
		return err
	})
}

func (gui *GUIUI) Start() *sync.WaitGroup {
	var init_wg sync.WaitGroup
	init_wg.Add(1)

	tcl_tk_path, err := install_scripts(TCLTK_FS, gui.App().DataDir())
	if err != nil {
		panic("failed to install tcl script")
	}

	// listen for events from the app and tie them to UI methods
	// TODO: might want to start this _after_ we've finished loading tk?
	go core.Dispatch(gui)

	// tcl/tk init
	go func() {
		err = tk.Init()
		if err != nil {
			slog.Error("failed to init tk", "error", err)
			panic("environment error")
		}
		tk.SetErrorHandle(core.PanicOnErr)

		// tablelist: https://www.nemethi.de
		// ttkthemes: https://ttkthemes.readthedocs.io/en/latest/loading.html#tcl-loading
		slog.Info("tcl/tk", "tcl", tk.TclVersion(), "tk", tk.TkVersion())

		// --- configure path
		// todo: fix environment so this isn't necessary

		// prepend a directory to the TCL `auto_path`,
		// where custom tcl/tk code can be loaded.
		tk.SetAutoPath(tcl_tk_path)

		_, err := tk.MainInterp().EvalAsString(`
# has no package
#source tcl-tk/widgettree/widgettree.tcl # disabled 2024-09-15: 'invalid command name "console"'

# $auto_path doesn't seem to work until searched

# tablelist/scaleutil is doing crazy fucking things
# like peering into running processes looking for and calling
# xfconf-query, gsettings, xrdb, xrandr etc.
# shortcircuit it's logic by giving it what it wants up front.
# we'll deal with it later.
set ::tk::scalingPct 100`)
		core.PanicOnErr(err)

		_, err = tk.MainInterp().EvalAsString(`
package require Tablelist_tile 7.6`)
		core.PanicOnErr(err) // "panic: error: NULL main window" happens here

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

			mw.SetTitle(gui.App().State.GetKeyVal("bw.app.name"))
			mw.Center(nil)
			mw.ShowNormal()
			mw.OnClose(func() bool {
				gui.Stop()
				return true
			})

			init_wg.Done() // the GUI isn't 'done', but we're done with init and ready to go.

			go func() {
				var wg sync.WaitGroup
				for {
					tk_fn := <-gui.tk_chan
					slog.Debug("--- TkSync block OPEN")
					wg.Add(1) // !important to be outside async
					tk.Async(func() {
						defer wg.Done()
						tk_fn()
					})
					wg.Wait()
					slog.Debug("--- TkSync block CLOSED")
				}
			}()
		})
	}()

	return &init_wg
}

func MakeGUI(app *core.App, wg *sync.WaitGroup) *GUIUI {
	wg.Add(1)

	// sets the colour that marked rows should be in the GUI
	app.State.SetKeyAnyVal(KV_GUI_ROW_MARKED_COLOUR, GUI_ROW_MARKED_COLOUR)

	return &GUIUI{
		tab_idx: map[string]string{},

		widget_ref: map[string]any{},
		inc:        make(chan []core.UIEvent, 5),
		out:        make(chan []core.UIEvent),
		tk_chan:    make(chan func()),
		WG:         wg,
		app:        app,
	}
}
