if {[file isdirectory [file join $dir parade]]} {
    if {![catch {package require Ttk}]} {
        package ifneeded ttk::theme::parade 0.1 \
            [list source [file join $dir parade.tcl]]
    } elseif {![catch {package require tile}]} {
        package ifneeded tile::theme::parade 0.1 \
            [list source [file join $dir parade.tcl]]
    } else {
	return
    }
}

