# changes to tablelist's behaviour, applied after `package require Tablelist`.
#
# each patch replaces or edits a tablelist proc in place, so the vendored tablelist stays
# untouched and the patches are listed in one place. a patch that edits a proc body
# checks the text it edits is still there, so a tablelist upgrade that changes the proc
# fails loudly here rather than silently losing the patch.
#
# written against tablelist 7.6.

package require Tablelist

source [file join [file dirname [info script]] stripes.tcl]

# --- helpers

# tablelist sources its script files on first use, through `auto_load`, and sourcing a
# file defines every proc in it. a proc replaced before its file is loaded is lost when
# the file loads later, so each patched proc has its file loaded first.
proc ::bw::tablelist::load_original {name} {
    if {[info procs $name] eq ""} {
        auto_load $name
    }
    if {[info procs $name] eq ""} {
        error "tablelist patch failed: no proc `$name`"
    }
}

# redefines the proc `name` with `from` replaced by `to` in its body, arguments and
# their defaults kept. errors when `from` is not in the body.
proc ::bw::tablelist::replace_in_proc_body {name from to} {
    load_original $name
    set body [info body $name]
    if {[string first $from $body] < 0} {
        error "tablelist patch failed: `$name` no longer contains `[string trim $from]`"
    }
    set arg_list {}
    foreach arg [info args $name] {
        if {[info default $name $arg default]} {
            lappend arg_list [list $arg $default]
        } else {
            lappend arg_list $arg
        }
    }
    proc $name $arg_list [string map [list $from $to] $body]
}

# redefines the proc `name` without `text` in its body.
proc ::bw::tablelist::remove_from_proc_body {name text} {
    replace_in_proc_body $name $text ""
}

# --- stripes belong to rows, not to positions on screen
#
# tablelist's `makeStripes` removes the "stripe" tag from every row and adds it back by
# position. every row below an expanded one changes stripe and repaints, and removing
# and re-adding a tag marks a row changed even when it ends up as it was.
#
# this version assigns stripes by top-level row, descendants inheriting their parent's,
# see `::bw::tablelist::stripe_flags`, and only changes the tag on rows whose stripe
# actually changes. the rows that only moved keep their tags, and Tk's text widget
# copies a moved line on screen rather than repainting it.

::bw::tablelist::load_original ::tablelist::makeStripes

proc tablelist::makeStripes win {
    upvar ::tablelist::ns${win}::data data
    if {[info exists data(stripesId)]} {
        after cancel $data(stripesId)
        unset data(stripesId)
    }

    if {[info exists data(dispId)]} {
        return ""
    }

    set w $data(body)
    if {$data(-stripebackground) eq "" && $data(-stripeforeground) eq ""} {
        $w tag remove stripe 1.0 end
        updateColors $win
        return ""
    }

    set rows {}
    foreach key $data(keyList) {
        set viewable [expr {![info exists data($key-elide)] &&
                            ![info exists data($key-hide)]}]
        lappend rows [list $key $data($key-parent) $viewable]
    }

    set line 1
    foreach row $rows flag [::bw::tablelist::stripe_flags $rows $data(-stripeheight)] {
        set striped [expr {[lsearch -exact [$w tag names $line.0] stripe] >= 0}]
        if {$flag && !$striped} {
            $w tag add stripe $line.0 $line.end
        } elseif {!$flag && $striped} {
            $w tag remove stripe $line.0 $line.end
        }
        incr line
    }

    updateColors $win
}

# --- collapsing no longer forces a repaint half way through
#
# `collapseSubCmd` calls `update idletasks` after eliding the collapsed rows and before
# restoring the stripes and separators. that paints a frame with the rows gone but the
# stripes stale, then the real one. the text widget lays itself out on demand for the
# `@x,y` index lookups that follow, so the forced repaint is not needed.

::bw::tablelist::remove_from_proc_body ::tablelist::collapseSubCmd "\tupdate idletasks\n"

# --- column widths do not depend on which rows are expanded
#
# tablelist sizes a dynamic-width column to its widest viewable cell, so expanding a row
# with wide children widens the column and collapsing it narrows it again, moving every
# column to the right and its header each time. here collapsed rows count too: a column
# is as wide as its widest cell, shown or not, and expanding changes nothing about the
# columns. the bookkeeping `-elide` does on a row's cells is left alone: at worst it
# asks for a recount, which `computeColWidth` answers with the same width.

::bw::tablelist::replace_in_proc_body ::tablelist::computeColWidth \
    "if {\[info exists data(\$key-elide)\] || \[info exists data(\$key-hide)\]} {" \
    "if {\[info exists data(\$key-hide)\]} {"


# --- resizing a column moves a guide line, the column follows on release
#
# tablelist resizes a column on every pointer motion, rewriting the visible cells of the
# column and laying the table out again each time. with a large table that costs more
# than the time between two motions, the drag falls behind the pointer and the cell
# contents jostle about as the column changes under them. here a motion only moves a
# thin guide line over the table to the pointer's position, as a window border drag does
# with "show window contents while dragging" off, and the release runs tablelist's own
# motion handler once, with the final position, before its release handler. an escape
# cancels the drag as before and takes the guide with it.
#
# the guide is a child of the table's parent, not of the table: a table painted off
# screen reaches the screen once per copy, and the guide must follow the pointer at once.
# `::bw::tablelist::outline_resize` set to 0 restores resizing as the pointer moves.

namespace eval ::bw::tablelist {
    variable outline_resize 1
}

foreach name {labelB1Motion labelB1Up escape} {
    ::bw::tablelist::load_original ::tablelist::$name
    rename ::tablelist::$name ::tablelist::${name}__live
}

proc ::bw::tablelist::resize_guide {win} {
    set parent [winfo parent $win]
    if {$parent eq "."} {
        return .bwResizeGuide
    }
    return $parent.bwResizeGuide
}

# places the guide of `win` at the screen x coordinate `X`, creating it on first use.
proc ::bw::tablelist::move_resize_guide {win X} {
    set guide [resize_guide $win]
    if {![winfo exists $guide]} {
        frame $guide -width 2 -borderwidth 0 -highlightthickness 0 \
            -background [::$win cget -foreground]
    }
    place $guide -in $win -x [expr {$X - [winfo rootx $win]}] -y 0 -relheight 1
    raise $guide
}

proc ::tablelist::labelB1Motion {w X x y} {
    if {$::bw::tablelist::outline_resize && [parseLabelPath $w win col]} {
        upvar ::tablelist::ns${win}::data data
        if {[info exists data(colBeingResized)] && $data(labelClicked)} {
            ::bw::tablelist::move_resize_guide $win $X
            return ""
        }
    }
    tailcall ::tablelist::labelB1Motion__live $w $X $x $y
}

proc ::tablelist::labelB1Up {w X} {
    if {[parseLabelPath $w win col]} {
        upvar ::tablelist::ns${win}::data data
        set dragged [winfo exists [::bw::tablelist::resize_guide $win]]
        destroy [::bw::tablelist::resize_guide $win]
        if {$dragged && [info exists data(colBeingResized)]} {
            # the one resize, through tablelist's own code. the motion handler's x and
            # y are not used when resizing.
            ::tablelist::labelB1Motion__live $w $X 0 0
        }
    }
    tailcall ::tablelist::labelB1Up__live $w $X
}

proc ::tablelist::escape {win col} {
    destroy [::bw::tablelist::resize_guide $win]
    tailcall ::tablelist::escape__live $win $col
}
