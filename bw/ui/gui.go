// the Tcl/Tk user interface for boardwalk.
// a `GUIUI` observes an app's state and renders its results as rows in a table, one tab
// per view.
// widget calls must happen on the Tk thread: use `GUIUI.TkSync` from anywhere else.
package ui

// todo: capture the collapsed/expanded state of rows as gui state, as is already done for
// the selection state.

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
	clone "github.com/huandu/go-clone/generic"
)

//go:embed tcl-tk/*
var TCLTK_FS embed.FS

// ---

type UIColumn struct {
	Title       string
	HiddenTitle bool
	Hidden      bool
	MaxWidth    int
}

// called for each row when the user types in the search box.
// `input` is the raw user input.
// `row` maps column title to cell value, including hidden columns.
// return `true` to show the row, `false` to hide it.
type SearchFilter func(input string, row map[string]string) bool

const (
	key_gui_state          = "bw.ui.gui"
	key_details_pane_state = "bw.gui.details-pane"
	key_selected_results   = "bw.gui.selected-rows"
)

var NS_KEYVAL = core.MakeNS("bw", "ui", "keyval")
var NS_VIEW = core.MakeNS("bw", "ui", "view")

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
	theme_editor_frame *tk.Frame
	tabber             *tk.Notebook
}

// --- tablelist

type GUITablelist struct {
	tk.TablelistEx
	OnExpandFnList   []func(full_key string)
	OnCollapseFnList []func(full_key string)
}

// returns a tablelist with sorting, multi-column sorting, extended selection and
// draggable columns enabled.
// panics if the widget cannot be created.
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

	// one callback dispatching to many allows individual callbacks to be toggled,
	// which matters because 'CollapseAll' fires once per row that has children.
	// the expand/collapse virtual events are generated on the tablelist widget itself,
	// not its body, so they are bound here directly: `Tablelist.OnItemExpanded` binds
	// to the body tag and never sees them.
	tk.BindEvent(widj.Tablelist.Id(), "<<TablelistRowExpand>>", func(e *tk.Event) {
		full_key := widj.GetFullKeys2(e.UserData)
		slog.Debug("item expanded", "full-key", full_key)
		for _, fn := range widj.OnExpandFnList {
			fn(full_key)
		}
	})

	tk.BindEvent(widj.Tablelist.Id(), "<<TablelistRowCollapse>>", func(e *tk.Event) {
		full_key := widj.GetFullKeys2(e.UserData)
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
	column_list          []UIColumn             // available columns and their properties for this tab
	ItemFkeyIndex        map[string]string      // a mapping of app item IDs => tablelist 'full key'
	FkeyItemIndex        map[string]string      // a mapping of tablelist 'full key' => app item IDs
	IgnoreMissingParents bool                   // results with a parent that are missing get a parent_id of '-1' (top-level)
	expanded_rows        mapset.Set[string]     // 'open' rows
	placeholder_rows     map[string]string      // result ID => full key of its placeholder child row
	search_fn            SearchFilter           // if set, the search box on this tab uses this fn to decide which rows to show
	search_entry         *tk.Entry              // search box widget, kept so `SetSearchFilter` can enable it
}

// returns true when the row for `result_id` currently has a placeholder child row.
// must be called on the Tk thread.
func (tab *GUITab) HasPlaceholderRow(result_id string) bool {
	_, present := tab.placeholder_rows[result_id]
	return present
}

// returns the number of child rows, placeholders included, of the row for `result_id`.
// returns -1 when the result has no row in this tab.
// must be called on the Tk thread.
func (tab *GUITab) RowChildCount(result_id string) int {
	fkey, present := tab.ItemFkeyIndex[result_id]
	if !present {
		return -1
	}
	return tab.table_widj.ChildCount(fkey)
}

func (tab *GUITab) OpenDetails() {
	tab.paned.HidePane(1, false)
}

func (tab *GUITab) CloseDetails() {
	tab.paned.HidePane(1, true)
}

// expands the row at `index`, revealing its immediate children only.
// a failure to expand is logged and swallowed.
// must be called on the Tk thread, see `ExpandRow`.
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

// collapses the row at `index` and its descendants.
func (tab *GUITab) CollapseRow(index string) {
	tab.gui.TkSync(func() {
		tab.table_widj.CollapseFully1(index)
	})
}

// sets the background of every row in `index_list` to `colour`.
// a failure to highlight an individual row is logged and the rest still change.
// must be called on the Tk thread, see `GUITab.HighlightManyRows`.
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

// highlights all rows in `index_list` with the app's configured 'marked' colour.
// falls back to a default colour, with a warning, when the app has none set.
func (tab *GUITab) MarkRows(index_list []string) {
	val := tab.gui.App().State.GetKeyVal(KV_GUI_ROW_MARKED_COLOUR)
	if val == "" {
		// todo: consider putting KV_GUI_ROW_MARKED_COLOUR into kvstore on app start and making this a panic
		slog.Warn("keyval missing, using default", "keyval", KV_GUI_ROW_MARKED_COLOUR, "default", GUI_ROW_MARKED_COLOUR)
		val = GUI_ROW_MARKED_COLOUR
	}
	tab.HighlightManyRows(index_list, val)
}

// installs `fn` as this tab's search callback and enables its search box.
// a nil `fn` disables the search box, which is the state a tab starts in.
func (tab *GUITab) SetSearchFilter(fn SearchFilter) {
	tab.gui.TkSync(func() {
		tab.search_fn = fn
		if tab.search_entry != nil {
			if fn == nil {
				tab.search_entry.SetState(tk.StateDisable)
			} else {
				tab.search_entry.SetState(tk.StateNormal)
			}
		}
	})
}

func (tab *GUITab) SetTitle(title string) {
	tab.gui.TkSync(func() {
		tab.title = title
		tab.gui.mw.tabber.SetTab(tab.tab_body, title)
	})
}

// declares the columns of this tab, replacing whatever was declared before.
// a column not in `column_list`, or marked hidden in it, is hidden.
// the columns are re-ordered to match the order given.
// columns are otherwise created as rows are added to the table.
func (tab *GUITab) SetColumnAttrs(column_list []UIColumn) {
	tab.gui.TkSync(func() {
		// hide
		old_col_titles := mapset.NewSet[string]()
		for _, col := range tab.column_list {
			old_col_titles.Add(col.Title)
		}

		new_col_idx := map[string]UIColumn{}
		for _, col := range column_list {
			new_col_idx[col.Title] = col
		}

		// columns in old that are not in new
		new_col_titles := mapset.NewSetFromMapKeys(new_col_idx)
		cols_to_hide := old_col_titles.Difference(new_col_titles)

		// a column's position is needed to hide it, and a new column may not exist yet
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

		// order
		// - https://www.nemethi.de/tablelist/tablelistWidget.html#movecolumn
		new_col_pos_idx := map[string]int{}
		for i, col := range column_list {
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

		// attrs
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

// returns true when `result` awaits an on-demand load of its children.
func has_unrealised_lazy_children(result core.Result) bool {
	if result.ChildrenRealised || result.Item == nil {
		return false
	}
	item, is_item := result.Item.(core.ItemInfo)
	return is_item && item.ItemHasChildren() == core.ITEM_CHILDREN_LOAD_LAZY
}

// inserts a placeholder child row under `parent_fkey` so a row with unrealised lazy
// children shows an expand affordance before its children exist.
// the placeholder is a bare table row: it never enters application state.
// a row that already has a placeholder is a no-op.
// returns true when a placeholder was inserted. the parent row is NOT collapsed here:
// tablelist marks a parent expanded when it gains its first child, and each `collapse`
// call forces a full repaint, so the caller must collapse the parent, batched where
// possible.
// must be called on the Tk thread.
func insert_placeholder_row(tab *GUITab, result_id string, parent_fkey string) bool {
	if _, present := tab.placeholder_rows[result_id]; present {
		return false
	}

	cells := make([]string, len(tab.column_list))
	if len(cells) == 0 {
		cells = []string{""}
	}
	cells[0] = "..."

	fkey_list := tab.table_widj.InsertChildList(parent_fkey, 0, [][]string{cells})
	if len(fkey_list) != 1 {
		slog.Error("expected one full key inserting placeholder row", "result-id", result_id, "full-keys", fkey_list)
		return false
	}
	tab.placeholder_rows[result_id] = fkey_list[0]
	return true
}

// removes the placeholder child row of the result `result_id`, if it has one.
// must be called on the Tk thread.
func remove_placeholder_row(tab *GUITab, result_id string) {
	fkey, present := tab.placeholder_rows[result_id]
	if !present {
		return
	}
	tab.table_widj.Delete2(fkey)
	delete(tab.placeholder_rows, result_id)
}

func donothing() {}

// returns the items of the 'Edit' menu.
// currently only the theme editor, and only in a development build: theme switching is
// disabled while 'parade' is the sole supported theme.
func build_edit_menu() []core.MenuItem {
	menuitem_list := []core.MenuItem{}

	// disabled. theme switching returns once more than one theme is supported.
	/*
		// bw/ui/tcl-tk/ttk-themes
		theme_list := mapset.NewSet("black", "clearlooks", "parade", "plastik")

		for _, theme := range tk.TtkTheme.ThemeIdList() {
			if theme == "scid" {
				// something wrong with this one
				continue
			}
			if theme_list.Contains(theme) {
				menuitem_list = append(menuitem_list, core.MenuItem{Name: theme, Fn: func(app *core.App) {
					tk.TtkTheme.SetThemeId(theme)
				}})
			}
		}
	*/

	if core.Debug() {
		menuitem_list = append(menuitem_list, core.MENU_SEP)
		menuitem_list = append(menuitem_list, core.MenuItem{Name: "Theme Editor", Fn: func(app *core.App) {
			_, err := tk.MainInterp().EvalAsString(`theme_editor::toggle_embedded_editor`)
			if err != nil {
				slog.Error("failed to toggle theme editor", "error", err)
			}
		}})
	}

	return menuitem_list
}

// returns the currently selected tab.
// must be called on the Tk thread, see `GetCurrentTab`.
func (gui *GUIUI) current_tab() *GUITab {
	idx := gui.mw.tabber.CurrentTabIndex()
	return gui.TabList[idx]
}

func (gui *GUIUI) GetCurrentTab() *GUITab {
	var tab *GUITab
	gui.TkSync(func() {
		tab = gui.current_tab()
	})
	return tab
}

// returns a menu item per registered service, each opening that service's form.
// the gui is built before the providers start, so this returns nothing until
// `GUIUI.RebuildMenu` is called after they have.
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

// calls the given `service` with `args`, opening a form on the current tab for any
// further input.
// todo: a form is opened even when the service needs no further input.
func (gui *GUIUI) CallService(service core.Service, args core.ServiceFnArgs) {
	tab := gui.current_tab()
	tab.OpenForm(service, args.ArgList)
	return
}

// builds the menu bar from the app's menus, merged with the GUI's own entries.
// must be called on the Tk thread.
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
		{Name: "Edit", MenuItemList: build_edit_menu()},
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
					// menu commands run on the Tk thread: the current tab must be
					// read here, but opening the service's form synchronises with
					// the Tk thread and deadlocks unless it leaves it first.
					tab := gui.current_tab()
					go tab.OpenForm(service, args.ArgList)
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

// inserts `row_list` into `tree` and returns the number of rows inserted.
// `item_idx` and `fkey_idx` are updated in place with the mapping between result IDs and
// the tablelist keys of the new rows.
// panics if `row_list` is empty or `parent` is an empty string.
//
// Parameters:
//   - parent: the parent to insert under. "-1" means the invisible root, so the rows
//     appear top-level. any other value is the index of the parent row.
//   - cidx: where among the parent's children to insert. 0 inserts at the beginning, and
//     a value equal to the parent's child count inserts at the end.
//
// - https://www.nemethi.de/tablelist/tablelistWidget.html#insertchildlist
func _insert_treeview_items(tree *tk.Tablelist, parent string, cidx int, row_list []Row, col_list []UIColumn, item_idx map[string]string, fkey_idx map[string]string) int {

	if len(row_list) == 0 {
		panic("row list is empty")
	}

	if parent == "" {
		panic("parent_id is empty")
	}

	var parent_idx string
	if parent == "-1" {
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

// returns the rows and columns for the given `result_list`.
// does not consider children, does not recurse.
// a non-empty `col_list` fixes the columns: values outside it are dropped and no new
// columns are added. an empty one grows the column list to cover every field found.
// results with no item are skipped, as are results whose item is not a `core.ItemInfo`.
func build_treeview_row(result_list []core.Result, col_list []UIColumn) ([]Row, []UIColumn) {
	fixed := len(col_list) > 0

	if !fixed {
		col_list = []UIColumn{
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
				for _, col := range item.ItemKeys() {
					if !col_idx.Contains(col) {
						col_list = append(col_list, UIColumn{Title: col})
						col_idx.Add(col)
					}
				}
			}

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

// adds each column in `new_col_list` to the tablelist `tree`, skipping those that already
// exist.
// an existing column's attributes are not updated.
// must be called on the Tk thread.
func set_tablelist_cols(new_col_list []UIColumn, tree *tk.Tablelist) {
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

// builds a text entry widget with a "Search:" label.
// The entry is created disabled and enabled once the owning tab has a `SearchFilter` fn set
// via `SetSearchFilter`.
// On keypress, the tab's `SearchFilter` is invoked once per row with the user's input and a `{column-title: cell-value}` map.
// When the fn returns `false` the row is hidden.
func MakeSearchBar(gui *GUIUI, parent tk.Widget) (*tk.PackLayout, *tk.Entry) {
	layout := tk.NewHPackLayout(parent)

	txt := tk.NewLabel(layout, "Search:")
	entry := tk.NewEntry(layout)
	entry.SetState(tk.StateDisable) // enabled by GUITab.SetSearchFilter

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
		delay          = 200 * time.Millisecond
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
			gui.TkSync(func() {
				ctab := gui.mw.tabber.CurrentTab()

				// during the debounce period it is possible the current tab has changed!
				// check that the text entry still belongs to the current tab.
				// if not, just exit early.
				entry_id := entry.Id() // ".notebook.tabframe3.hpacklayout5.entry7"
				prefix := ctab.Id()    // ".notebook.tabframe3"
				if !strings.HasPrefix(entry_id, prefix) {
					return
				}

				// no search fn, cannot search.
				// we have to do this because this is a _global_ handler for _all_ tabs.
				guitab := gui.current_tab()
				if guitab.search_fn == nil {
					return
				}

				table := guitab.table_widj
				text := entry.Text()
				column_count := len(guitab.column_list)
				if column_count == 0 {
					return
				}

				column_titles := make([]string, column_count)
				for i, col := range guitab.column_list {
					column_titles[i] = col.Title
				}

				// one round-trip: `GetCells` returns the rectangle in row-major order,
				// `column_count` cells per row.
				last_col := core.IntToString(column_count - 1)
				flat := table.GetCells("0,0", "last,"+last_col, tk.TABLELIST_ROW_STATE_ALL)
				row_count := len(flat) / column_count

				specs := make([]tk.ConfigRowListSpec, row_count)
				for r := range row_count {
					row := make(map[string]string, column_count)
					base := r * column_count
					for c, title := range column_titles {
						row[title] = flat[base+c]
					}
					hide := "true"
					if guitab.search_fn(text, row) {
						hide = "false"
					}
					specs[r] = tk.ConfigRowListSpec{Index: core.IntToString(r), Option: "hide", Value: hide}
				}
				if err := table.ConfigRowList(specs); err != nil {
					slog.Error("search row hide failed", "error", err)
				}
			})
		})
	})

	return layout, entry
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

	search_bar, search_entry := MakeSearchBar(gui, tab_body)
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
		gui:              gui,
		tab_body:         tab_body,
		paned:            paned,
		table_widj:       table_widj,
		details_widj:     d_widj,
		search_entry:     search_entry,
		title:            title,
		filter:           viewfn,
		ItemFkeyIndex:    map[string]string{},
		FkeyItemIndex:    map[string]string{},
		expanded_rows:    mapset.NewSet[string](),
		placeholder_rows: map[string]string{},
	}
	gui.TabList = append(gui.TabList, tab)
	gui.tab_idx[title] = tab_body.Id()

	// ---

	// expanding a row with unrealised lazy children triggers an on-demand load.
	// the load runs off the Tk thread and the loaded children arrive through the
	// usual state-change notifications.
	table_widj.OnExpandFnList = append(table_widj.OnExpandFnList, func(full_key string) {
		tab.expanded_rows.Add(full_key)
		result_id, present := tab.FkeyItemIndex[full_key]
		if !present {
			// not a result row, e.g. a placeholder
			return
		}
		gui.realise_lazy_children(result_id)
	})

	table_widj.OnCollapseFnList = append(table_widj.OnCollapseFnList, func(full_key string) {
		tab.expanded_rows.Remove(full_key)
	})

	// ---

	// double clicking a tablelist row toggles expand/collapse
	table_widj.BindEvent("<Double-1>", func(e *tk.Event) {
		row_idx := "active"
		if table_widj.ChildCount(row_idx) == 0 {
			return
		}
		fkey := table_widj.GetFullKeys2(row_idx)
		if table_widj.IsExpanded(row_idx) {
			table_widj.CollapseFully1(fkey)
		} else {
			table_widj.ExpandPartly1(fkey)
		}
	})

	// ---

	// right clicking a tablelist row.
	err := table_widj.BindEvent("<ButtonRelease-3>", func(e *tk.Event) {

		widj := table_widj

		// `ActivateAtGlobal` moves the "active" cursor to the row under the right-click
		// using _global_ (screen) coordinates since the event fires on the tablelist mega-widget.
		widj.ActivateAtGlobal(e.GlobalPosX, e.GlobalPosY)

		// check if newly clicked row is part of the current selection.
		// if so, nothing happens and selected rows are preserved.
		// some subtle behaviour here: you must right-click _within_ the multiple selected rows to preserve them.
		// you can't select multiple rows then right-click a row that isn't selected. that deselects them.
		if !widj.SelectionIncludes("active") {
			// current row not selected, clears all selected from start (0) to end
			widj.SelectionClear("0 end")
			// then select just the right-clicked row
			widj.SelectionSet("active")
		}

		id_list := widj.CurSelection3()

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
		has_services := false
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

			has_services = true

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
						if len(grouped) == 1 {
							gui.RunService(service, core.MakeServiceFnArgs("selected", grouped[0]), nil)
						} else {
							gui.RunService(service, core.MakeServiceFnArgs("selected", grouped), nil)
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
		}

		if has_services {
			context_menu.AddSeparator()
		}

		props_action := tk.NewAction("Properties")
		props_action.OnCommand(func() {
			slog.Info("properties", "num-results", len(res_list))
		})
		context_menu.AddAction(props_action)

		tk.PopupMenu(context_menu, e.GlobalPosX, e.GlobalPosY)
	})
	if err != nil {
		panic(fmt.Sprintf("error! %v", err))
	}

}

// returns `result_list` sorted so that every parent comes before its children, which is
// the order the table requires for insertion.
// results whose parent is not in `result_list` are treated as top-level and come first.
// a result is returned at most once, so a cycle cannot loop forever.
// results unreachable from any root are dropped.
func sort_insertion_order(result_list []core.Result) []core.Result {
	all_idx := mapset.NewSet[string]()
	child_idx := map[string][]core.Result{} // {parent.ID => [child, child, ...], ...}
	for _, r := range result_list {
		child_idx[r.ParentID] = append(child_idx[r.ParentID], r)
		all_idx.Add(r.ID)
	}

	// parents not found in `result_list`. their children come first
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

// adds the results named in `id_list`, read from `snapshot`, to the given `tab`'s table.
// a result failing the tab's view filter is not shown, and neither are its children
// unless the tab ignores missing parents, in which case they become top-level.
// rows are inserted in batches grouped by parent, because a batch may need the table key
// of a row inserted by the previous batch.
// must be called on the Tk thread, see `AddRowToTree`.
// panics if an ID in `id_list` is not in `snapshot`.
func add_row_to_tree(gui *GUIUI, tab *GUITab, snapshot map[string]core.Result, id_list ...string) {
	tree := tab.table_widj

	excluded := map[string]bool{}

	result_list := []core.Result{}
	for _, id := range id_list {
		result, present := snapshot[id]
		if !present {
			slog.Error("GUI, result with id not found in snapshot", "id", id)
			panic("programming error")
		}

		if !tab.filter(result) {
			slog.Debug("row excluded and won't be present in tab", "tab", tab.title, "id", id, "result-ns", result.NS)
			excluded[result.ID] = true
			continue
		}

		result_list = append(result_list, result)
	}

	if len(result_list) == 0 {
		return
	}

	// todo: ignoring missing parents doubles as shorthand for 'no grouping'. separate the two.
	if !tab.IgnoreMissingParents {
		result_list = sort_insertion_order(result_list)
	}

	no_parent := "-1"
	bunch_list := core.Bunch(result_list, func(r core.Result) any {
		return r.ParentID
	})

	rows_inserted := 0
	placeholder_parents := mapset.NewSet[string]() // full keys of rows that gained a placeholder

	// figure out which parent to insert each bunch of results under
	for _, bunch := range bunch_list {
		first_row := bunch[0]
		var parent_id string
		var present bool
		if first_row.ParentID == "" {
			parent_id = no_parent
		} else {
			is_excluded := excluded[first_row.ParentID] // warning: bool default value is being used here for rows not found
			if is_excluded {
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
					// the parent, or one of its ancestors, may have been excluded by
					// this tab's filter, possibly in an earlier update: children
					// realised on demand arrive well after their parent. a result
					// whose parent has no row in this tab has no row either.
					// a parent missing from state entirely is still a programming error.
					if gui.App().HasResult(first_row.ParentID) {
						slog.Debug("parent has no row in this tab, excluding this bunch of results", "tab", tab.title, "parent", first_row.ParentID)
						continue
					}

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
		rows_inserted += len(row_list)

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

		// rows awaiting an on-demand load get a placeholder child so they can be expanded.
		// inserting a first child marks its parent expanded, so these parents join the
		// collapse batch below.
		for _, result := range bunch {
			if has_unrealised_lazy_children(result) {
				fkey := tab.ItemFkeyIndex[result.ID]
				if insert_placeholder_row(tab, result.ID, fkey) {
					placeholder_parents.Add(fkey)
				}
			}
		}
	}

	if rows_inserted > 0 {
		parents := mapset.NewSet[string]()
		for _, result := range result_list {
			if result.ParentID != "" {
				parents.Add(result.ParentID)
			}
		}

		// one batched collapse: each `collapse` call forces a full repaint, so
		// collapsing row by row makes large insertions visibly slow.
		to_collapse := placeholder_parents.ToSlice()
		for _, parent_id := range parents.ToSlice() {
			fkey, present := tab.ItemFkeyIndex[parent_id]
			if present && !tab.expanded_rows.Contains(fkey) && !placeholder_parents.Contains(fkey) {
				to_collapse = append(to_collapse, fkey)
			}
		}
		if len(to_collapse) > 0 {
			tree.CollapseFully2(to_collapse)
		}
	}

}

// calls `add_row_to_tree` on the Tk thread, blocking until it has run.
func AddRowToTree(gui *GUIUI, tab *GUITab, snapshot map[string]core.Result, id_list ...string) {
	gui.TkSync(func() {
		add_row_to_tree(gui, tab, snapshot, id_list...)
	})
}

// runs `fn` on the Tk thread and blocks until it has finished.
// every widget call must go through here unless it is already on the Tk thread.
// calling it from the Tk thread deadlocks.
func (gui *GUIUI) TkSync(fn func()) {
	var wg sync.WaitGroup
	wg.Add(1)
	tk.Async(func() {
		defer wg.Done()
		fn()
	})
	wg.Wait()
}

// updates the row for the result `id` in the given `tab`, re-highlighting and expanding it
// according to its tags.
// does nothing when the tab has no rows yet, or when the result has no row in this tab.
// `snapshot` is captured at notification time, so no result data is read from mutable app
// state.
// must be called on the Tk thread.
// panics if `id` is not in `snapshot`.
func update_row_in_tree(gui *GUIUI, tab *GUITab, snapshot map[string]core.Result, id string) {
	slog.Debug("gui.UpdateRow UPDATING ROW", "id", id)
	if len(tab.ItemFkeyIndex) == 0 {
		slog.Debug("gui tab failed to update row, tab has no rows to update yet", "tab", tab.title, "id", id)
		return
	}

	full_key, present := tab.ItemFkeyIndex[id]
	if !present {
		slog.Debug("gui failed to update row, row full key not found in row index", "id", id)
		return
	}

	result, in_snapshot := snapshot[id]
	if !in_snapshot {
		slog.Error("gui tab failed to update row, result with id not found in snapshot", "id", id)
		panic("programming error")
	}

	tree := tab.table_widj

	if false {
		set_tablelist_cols(tab.column_list, tree.Tablelist)
	}

	row_list, col_list := build_treeview_row([]core.Result{result}, tab.column_list)

	if len(row_list) != 1 {
		slog.Error("gui failed to update row, result of building row should be precisely 1", "row-list", row_list)
		panic("")
	}
	row := row_list[0]

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

	if result.Tags.Contains(core.TAG_HAS_UPDATE) {
		colour := gui.App().State.GetKeyVal(KV_GUI_ROW_MARKED_COLOUR)
		highlight_row(tab, []string{full_key}, colour)
	}

	if result.Tags.Contains(core.TAG_SHOW_CHILDREN) {
		tab.expand_row(full_key)
	}

	// placeholder upkeep: a row realised on demand loses its placeholder, one still
	// awaiting realisation keeps its expand affordance.
	if result.ChildrenRealised {
		remove_placeholder_row(tab, id)
	} else if has_unrealised_lazy_children(result) {
		if insert_placeholder_row(tab, id, full_key) {
			// a single row: no batching to be had
			tab.table_widj.CollapseFully1(full_key)
		}
	}
}

// removes the row for the result `id` from the given `tab`.
// a result with no row in this tab is a no-op.
// must be called on the Tk thread.
func delete_row_in_tree_sync(tab *GUITab, id string) {
	fullkey := tab.ItemFkeyIndex[id]
	if fullkey != "" {
		tab.table_widj.Delete2(fullkey)
		tab.expanded_rows.Remove(fullkey)
		delete(tab.placeholder_rows, id) // the placeholder row is deleted with its parent
	}
}

// rebuilds the menu bar from scratch.
// the menu is built from app state, so call this after that state changes.
// todo: the menu should update itself rather than needing to be rebuilt.
func (gui *GUIUI) RebuildMenu() {
	gui.TkSync(func() {
		gui.mw.SetMenu(build_menu(gui, gui.mw))
	})
}

// applies the theme's styling to every Tablelist widget, and installs desktop-standard
// keybindings on every Entry widget.
// call it once, after all tabs have been created: widgets created later are not styled.
// failures are logged, not returned.
//
// Tablelist is a megawidget that builds itself from internal sub-widgets, and unlike TTK
// widgets it does not reliably honour the option database: the theme sets its entries
// before the widgets exist, the patterns do not reach dynamically created sub-widgets, and
// Tablelist's own high-priority defaults override them. configuring each widget directly
// after it exists takes priority over all of that.
//
// the Entry bindings are set at the class level, so they apply to current and future
// widgets. Tk's defaults follow Emacs conventions, where Ctrl+A means 'home' and
// Ctrl+Backspace does nothing.
func (gui *GUIUI) ApplyTablelistStyling() {
	gui.TkSync(func() {
		_, err := tk.MainInterp().EvalAsString("ttk::theme::parade::apply_tablelist_styling")
		if err != nil {
			slog.Warn("Failed to apply tablelist styling", "error", err)
		}

		_, err = tk.MainInterp().EvalAsString(`
			foreach class {Entry TEntry} {
				bind $class <Control-Key-a> {%W selection range 0 end; break}
				bind $class <Control-BackSpace> {
					if {[%W selection present]} {
						%W delete sel.first sel.last
					} else {
						set prev [tcl_wordBreakBefore [%W get] [%W index insert]]
						%W delete $prev insert
					}
					break
				}
				bind $class <Control-Delete> {
					if {[%W selection present]} {
						%W delete sel.first sel.last
					} else {
						%W delete insert [tcl_wordBreakAfter [%W get] [%W index insert]]
					}
					break
				}
			}
		`)
		if err != nil {
			slog.Warn("failed to apply Entry keybindings", "error", err)
		}
	})
}

// ---

// creates a form for the given `service`,
// binds the given `initial_data`, if any,
// opens the details pane,
// renders a GUI version of the form.
func (tab *GUITab) OpenForm(service core.Service, initial_data []core.KeyVal) {
	form := core.MakeForm(service)
	form.Update(initial_data)

	tab.gui.TkSync(func() {
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

func (tab *GUITab) CloseForm() {
	tab.gui.TkSync(func() {
		tab.close_form()
	})
}

//

func NewWindow(gui *GUIUI) *Window {
	mw := &Window{}
	mw.Window = tk.RootWindow()
	mw.SetSizeN(1200, 800)
	mw.SetMenu(build_menu(gui, mw))

	// paned window holds main content and theme editor
	paned := tk.NewPaned(mw, tk.Horizontal)

	// the notebook is considered the 'main' content area
	mw.tabber = tk.NewNotebook(paned)

	// will hold the theme editor when/if we init it
	mw.theme_editor_frame = tk.NewFrame(paned)

	main_content_weight := 4
	paned.AddWidget(mw.tabber, main_content_weight)

	vbox := tk.NewVPackLayout(mw)
	vbox.AddWidgetEx(paned, tk.FillBoth, true, 0)

	return mw
}

func (gui *GUIUI) configure_embedded_theme_editor() {
	slog.Info("configuring embedded theme editorig")

	// the app is essentially split into two areas, the 'main content' and the 'theme editor'.
	// this is the path to where we display the theme editor.
	parent_path := gui.mw.theme_editor_frame.Id()

	tcl_tk_path := filepath.Join(gui.app.DataDir(), "tcl-tk")
	parade_theme_path := filepath.Join(tcl_tk_path, "ttk-themes", "parade")
	theme_editor_fs_path := filepath.Join(tcl_tk_path, "theme-editor.tcl")
	theme_editor_content, err := os.ReadFile(theme_editor_fs_path)
	if err != nil {
		slog.Error("cannot find theme editor", "path", theme_editor_fs_path, "error", err)
		panic("programming error")
	}

	// eval that source file
	_, err = tk.MainInterp().EvalAsString(string(theme_editor_content))
	if err != nil {
		slog.Error("failed to source theme editor", "error", err)
		panic("programming error")
	}

	// create the editor and tell it who it's parent is
	// pass the repository path so theme editor can save to source files
	cwd, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get working directory", "error", err)
		panic(err)
	}
	// Go up one level from ./strongbox to repository root (./strongbox2)
	repo_path := filepath.Dir(cwd)

	tcl_command := fmt.Sprintf(`
		# when executed the cwd for parade.tcl code will be whatever dir strongbox was executed from,
		# preventing 'source parade-overrides.tcl'. this hack ensures that the theme editor can find the file.
		cd "%s"
		if {[namespace exists theme_editor]} {
			if {[info commands theme_editor::create_embedded_editor] ne ""} {
				set ::theme_editor::tcl_source_dir {%s}
				theme_editor::create_embedded_editor %s
			} else {
				puts "ERROR: create_embedded_editor command not found"
				exit 1
			}
		} else {
			puts "ERROR: theme_editor namespace not found"
			exit 1
		}
	`, parade_theme_path, repo_path, parent_path)

	result, err := tk.MainInterp().EvalAsString(tcl_command)
	if err != nil {
		slog.Error("failed to launch embedded theme editor", "error", err, "result", result)
		panic("programming error")
	}
}

// a unit of work for the service worker: a fn to run and a `done` callback to handle its
// result, such as releasing a `WaitGroup`.
// these go on the `service_chan` channel, which has a size of 1, making a serial queue so
// services run one at a time.
type service_work struct {
	fn   func() core.ServiceResult
	done func(core.ServiceResult)
}

// the GUI itself.
// observes an app's state and renders its results across one or more tabs.
type GUIUI struct {
	app *core.App

	TabList      []*GUITab
	tab_idx      map[string]string
	widget_ref   map[string]any
	service_chan chan service_work

	lazy_loads_in_flight mapset.Set[string] // result IDs whose children are being loaded

	WG *sync.WaitGroup

	mw *Window // 'main window', intended to be the gui 'root' from where we can reach all gui elements
}

var _ core.StateObserver = (*GUIUI)(nil)

func (gui *GUIUI) App() *core.App {
	return gui.app
}

// started at GUI init, it pulls work off the `gui.service_chan` channel,
// runs `service_work.fn()`,
// then dispatches `done()` back onto the Tk thread via tk.Async so any UI updates happen safely.
func (gui *GUIUI) service_worker() {
	for work := range gui.service_chan {
		result := work.fn()
		if work.done != nil {
			tk.Async(func() {
				work.done(result)
			})
		}
	}
}

// realises the lazy children of the result with `result_id` off the Tk thread.
// the loaded children arrive through the usual state-change notifications.
// a request for a result that is gone, already realised, not lazy, or already
// loading is ignored.
func (gui *GUIUI) realise_lazy_children(result_id string) {
	result := gui.App().GetResult(result_id)
	if result == nil || !has_unrealised_lazy_children(*result) {
		return
	}

	if !gui.lazy_loads_in_flight.Add(result_id) {
		// a load for this result is already in flight
		return
	}

	go func() {
		defer gui.lazy_loads_in_flight.Remove(result_id)
		_, err := core.Children(gui.App(), *result)
		if err != nil {
			slog.Error("failed to realise children", "id", result_id, "error", err)
		}
	}()
}

// queues `service` to be called with `args` and returns immediately.
// `done` is called with the result on the Tk thread, so it may touch widgets.
// services run one at a time, in the order they are queued.
func (gui *GUIUI) RunService(service core.Service, args core.ServiceFnArgs, done func(core.ServiceResult)) {
	gui.service_chan <- service_work{
		fn: func() core.ServiceResult {
			return core.CallServiceFnWithArgs(gui.App(), service, args)
		},
		done: done,
	}
}

// pushes a no-op job onto the service worker channel and blocks until it completes,
// guaranteeing all previously queued work has finished.
func (gui *GUIUI) WaitForServices() {
	var wg sync.WaitGroup
	wg.Add(1)
	gui.service_chan <- service_work{
		fn:   func() core.ServiceResult { return core.ServiceResult{} },
		done: func(core.ServiceResult) { wg.Done() },
	}
	wg.Wait()
}

// updates every tab to match the change between the two snapshots.
// the changed results are deep-copied, so this observer is isolated from later state
// modifications made by other observers or by the time the Tk callback runs.
// the table is updated asynchronously and this returns before it has happened: updating
// synchronously would deadlock when a Tk callback triggers the state update that lands
// here.
func (gui *GUIUI) OnResultsChanged(old_snapshot, new_snapshot *core.Snapshot) {
	diff := core.DiffResults(old_snapshot, new_snapshot)

	if len(diff.Added) == 0 && len(diff.Modified) == 0 && len(diff.Deleted) == 0 {
		return
	}

	// a partial snapshot of just the changed results
	snapshot := make(map[string]core.Result, len(diff.Added)+len(diff.Modified))
	for _, id := range diff.Added {
		if r := new_snapshot.GetResult(id); r != nil {
			snapshot[id] = clone.Clone(*r)
		}
	}
	for _, id := range diff.Modified {
		if r := new_snapshot.GetResult(id); r != nil {
			snapshot[id] = clone.Clone(*r)
		}
	}

	tk.Async(func() {
		for _, tab := range gui.TabList {
			if len(diff.Added) > 0 {
				add_row_to_tree(gui, tab, snapshot, diff.Added...)
			}
			for _, id := range diff.Modified {
				update_row_in_tree(gui, tab, snapshot, id)
			}
			for _, id := range diff.Deleted {
				delete_row_in_tree_sync(tab, id)
			}
		}
	})
}

// handles an action dispatched by a provider.
// panics on an action type the GUI does not handle.
func (gui *GUIUI) OnAction(action core.Action) {
	switch action.Type {
	case core.ACTION_SWITCH_TAB:
		gui.SetActiveTab(action.Payload.(string))
	default:
		panic(fmt.Sprintf("unhandled action type: %s", action.Type))
	}
}

// returns the tab with the given `title`, or nil when there is none.
func (gui *GUIUI) GetTab(title string) *GUITab {
	for _, tab := range gui.TabList {
		if title == tab.title {
			return tab
		}
	}
	return nil
}

func (gui *GUIUI) AddTab(title string, filter core.ViewFilter) {
	gui.TkSync(func() {
		AddTab(gui, title, filter)
	})
}

// brings the tab with the given `title` to the front.
// an unknown title is logged and nothing changes.
func (gui *GUIUI) SetActiveTab(title string) {
	slog.Info("setting active tab", "title", title)
	tab_id, exists := gui.tab_idx[title]
	if !exists {
		slog.Error("tab not found in index, cannot set active tab", "title", title)
		return
	}
	widj, exists := tk.LookupWidget(tab_id)
	if !exists {
		slog.Error("widget with id not found. cannot set active tab", "title", title, "id", tab_id)
		return
	}
	gui.TkSync(func() {
		gui.mw.tabber.SetCurrentTab(widj)
	})
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

func (gui *GUIUI) Show() {
	gui.TkSync(func() {
		gui.mw.ShowNormal()
	})
}

// quits Tk and releases the GUI's wait group, unblocking whoever is waiting on it.
func (gui *GUIUI) Stop() {
	slog.Warn("stopping gui")
	tk.Quit()
	// tk.Quit() is async. a short pause prevents:
	//   'panic: error: script: "destroy .", error: "invalid command name \"destroy\""'
	time.Sleep(5 * time.Millisecond)
	gui.WG.Done()
}

// copies tablelist and the other tcl/tk scripts from `src` into `dst_root`, replacing
// whatever was there, and returns the directory they were written to.
// the scripts are embedded in the binary but tcl/tk cannot read a virtual filesystem, so
// they must exist on disk.
func install_scripts(src fs.FS, dst_root string) (string, error) {
	src_root := "."

	dst := filepath.Join(dst_root, "tcl-tk")
	err := os.RemoveAll(dst)
	if err != nil {
		slog.Error("failed to replace application tcl-tk directory", "path", dst)
		return dst, err
	}

	return dst, fs.WalkDir(src, src_root, func(path string, d fs.DirEntry, err error) error {
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

// installs the tcl/tk scripts, starts Tk on its own thread and builds the main window.
// returns a `WaitGroup` that is done once the GUI is ready to use.
// panics if the scripts cannot be installed.
func (gui *GUIUI) Start() *sync.WaitGroup {
	var init_wg sync.WaitGroup
	init_wg.Add(1)

	tcl_tk_path, err := install_scripts(TCLTK_FS, gui.App().DataDir())
	if err != nil {
		panic("failed to install tcl scripts")
	}

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
package require Tablelist 7.6`)
		core.PanicOnErr(err) // "panic: error: NULL main window" happens here

		// --- configure theme
		// todo: set as bw preference
		// todo: limit available themes
		// todo: dark theme
		// todo: main menu seems to resist styling

		default_theme := "parade"
		tk.TtkTheme.SetThemeId(default_theme)

		// ---

		tk.MainLoop(func() {

			mw := NewWindow(gui)
			gui.mw = mw

			gui.configure_embedded_theme_editor()

			mw.SetTitle(gui.App().State.GetKeyVal("bw.app.name"))
			mw.Center(nil)
			mw.OnClose(func() bool {
				gui.Stop()
				return true
			})

			init_wg.Done()
		})
	}()

	return &init_wg
}

func MakeGUI(app *core.App, wg *sync.WaitGroup) *GUIUI {
	wg.Add(1)

	// sets the colour that marked rows should be in the GUI
	app.State.SetKeyAnyVal(KV_GUI_ROW_MARKED_COLOUR, GUI_ROW_MARKED_COLOUR)

	gui := &GUIUI{
		tab_idx:              map[string]string{},
		widget_ref:           map[string]any{},
		service_chan:         make(chan service_work, 1),
		lazy_loads_in_flight: mapset.NewSet[string](),
		WG:                   wg,
		app:                  app,
	}
	go gui.service_worker()
	return gui
}
