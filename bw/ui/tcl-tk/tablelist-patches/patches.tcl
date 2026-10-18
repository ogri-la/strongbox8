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

# --- column widths do not depend on which rows are shown
#
# tablelist sizes a dynamic-width column to its widest viewable cell, so expanding a row
# with wide children widens the column and collapsing it narrows it again, and a search
# that hides rows does the same, moving every column to the right and its header each
# time. here collapsed and hidden rows count too: a column is as wide as its widest
# cell, shown or not, and expanding or searching changes nothing about the columns. the
# bookkeeping `-elide` and `-hide` do on a row's cells is left alone: at worst it asks
# for a recount, which `computeColWidth` answers with the same width.

::bw::tablelist::replace_in_proc_body ::tablelist::computeColWidth \
    "if {\[info exists data(\$key-elide)\] || \[info exists data(\$key-hide)\]} {" \
    "if {false} {"


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
            # no narrower than the release will accept, so the guide is truthful
            set narrowest [expr {$data(X) - $data(oldStretchedColWidth) + $data(minColWidth)}]
            ::bw::tablelist::move_resize_guide $win [expr {max($X, $narrowest)}]
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

# --- hiding and showing rows in bulk
#
# tablelist's `-hide` row option does its column width bookkeeping, body height
# reconfiguration and option parsing once per row, about 70 microseconds a row, so a
# search over a catalogue-sized table takes hundreds of milliseconds on the Tk thread.
# `set_hidden` changes the hidden state of many rows at once and does the follow-up work
# once: the body height, the stripes and the view. column widths need no work, as hidden
# rows count towards them, see above. a hidden row's descendants are elided and a shown
# expanded row's are revealed, as `-hide` does. rows are named by full key, as the
# application knows them.
#
# two things keep the text widget's work short. while rows are elided, every tag change
# inside the elided block costs the widget a walk over the block, so the hidden rows are
# not elided for the duration and are elided again at the end, one relayout. and rows are
# handled in table order, so the walk a change does cost stays over rows just handled.

proc ::bw::tablelist::set_hidden {win hide_keys show_keys} {
    upvar ::tablelist::ns${win}::data data
    ::tablelist::synchronize $win
    ::tablelist::displayItems $win
    if {!$data(keyToRowMapValid)} {
        # `updateKeyToRowMap` expects to run from its own scheduling, which it clears
        if {[info exists data(mapId)]} {
            after cancel $data(mapId)
        } else {
            set data(mapId) {}
        }
        ::tablelist::updateKeyToRowMap $win
    }
    set w $data(body)
    set pu $::tablelist::pu

    set hide_pairs {}
    foreach key $hide_keys {
        if {[info exists data($key-hide)] || ![info exists data($key-parent)]} {
            continue
        }
        lappend hide_pairs [list [::tablelist::keyToRow $win $key] $key]
    }
    set show_pairs {}
    foreach key $show_keys {
        if {![info exists data($key-hide)]} {
            continue
        }
        lappend show_pairs [list [::tablelist::keyToRow $win $key] $key]
    }
    if {[llength $hide_pairs] == 0 && [llength $show_pairs] == 0} {
        return ""
    }

    set elide [$w tag cget hiddenRow -elide]
    $w tag configure hiddenRow -elide 0
    set rows {}

    foreach pair [lsort -integer -index 0 $hide_pairs] {
        lassign $pair row key
        set line [expr {$row + 1}]
        set data($key-hide) 1
        $w tag add hiddenRow $line.0 $line.end+1$pu
        if {![info exists data($key-elide)]} {
            incr data(nonViewableRowCount)
            set to_row [::tablelist::nodeRow $win $key end]
            for {set r [expr {$row + 1}]} {$r < $to_row} {incr r} {
                ::tablelist::doRowConfig $r $win -elide 1
            }
            if {$row == $data(editRow)} {
                ::tablelist::doCancelEditing $win
            }
        }
        lappend rows $row
    }

    foreach pair [lsort -integer -index 0 $show_pairs] {
        lassign $pair row key
        set line [expr {$row + 1}]
        unset data($key-hide)
        # keeps embedded windows on this line from being created before it is displayed
        $w tag add elidedWin $line.0 $line.end
        $w tag remove hiddenRow $line.0 $line.end+1$pu
        if {![info exists data($key-elide)]} {
            incr data(nonViewableRowCount) -1
            set indent_name $key,$data(treeCol)-indent
            if {[info exists data($indent_name)] && [string match "*expanded*" $data($indent_name)]} {
                ::tablelist::expandSubCmd $win [list $row -partly]
            }
        }
        lappend rows $row
    }

    $w tag configure hiddenRow -elide $elide

    ::tablelist::adjustRowIndex $win data(anchorRow) 1
    set active_row $data(activeRow)
    ::tablelist::adjustRowIndex $win active_row 1
    set data(activeRow) $active_row

    if {$data(-height) <= 0} {
        set viewable [expr {$data(itemCount) - $data(nonViewableRowCount)}]
        $w configure -height $viewable
        $data(lb) configure -height $viewable
    }
    set data(viewableRowList) {-1}
    ::tablelist::makeStripesWhenIdle $win
    ::tablelist::showLineNumbersWhenIdle $win
    ::tablelist::updateViewWhenIdle $win
    event generate $win <<TablelistRowHiddenStateChanged>> -data [lsort -integer $rows]
    return ""
}

# --- a resized column's rows off screen are redisplayed in the background
#
# when a column resize ends, tablelist redisplays every cell of the column, snipping
# each text to the new width, before anything else happens: about 60 microseconds a row,
# a second or more for a catalogue, during which the window is frozen behind a busy
# cursor. the rows on screen were already redisplayed by the motion handler the release
# runs, see above, so here the rows above and below them are redisplayed in chunks of
# 200 rows at idle time, between whatever else the user is doing. a row off screen shows
# its newly snipped text by the time it scrolls into view, as the chunks run in table
# order from the top and finish within a second even for a catalogue.
#
# `redisplayCol` only includes collapsed and hidden rows when asked for every row, and
# the chunks need them included too, so it is told so through `redisplay_all_rows`.

::bw::tablelist::replace_in_proc_body ::tablelist::redisplayCol \
    "set allRows \[expr {\$first == 0 && \$last eq \"last\"}\]" \
    "set allRows \[expr {(\$first == 0 && \$last eq \"last\") || \[info exists ::bw::tablelist::redisplay_all_rows\]}\]"

::bw::tablelist::replace_in_proc_body ::tablelist::labelB1Up__live \
    "redisplayCol \$win \$col 0 last" \
    "::bw::tablelist::redisplay_col_visible_then_rest \$win \$col"

proc ::bw::tablelist::redisplay_col_visible_then_rest {win col} {
    upvar ::tablelist::ns${win}::data data
    set w $data(body)
    set top [expr {int([$w index @0,0]) - 1}]
    set bottom [expr {min(int([$w index @0,[expr {[winfo height $w] - 1}]]) - 1, $data(lastRow))}]
    cancel_col_chunks $win $col
    set data($col-bwRedispId) [after idle \
        [list ::bw::tablelist::redisplay_col_chunk $win $col 0 $top $bottom]]
}

proc ::bw::tablelist::cancel_col_chunks {win col} {
    upvar ::tablelist::ns${win}::data data
    if {[info exists data($col-bwRedispId)]} {
        after cancel $data($col-bwRedispId)
        unset data($col-bwRedispId)
    }
}

# redisplays up to 200 rows of column `col` from row `from`, collapsed and hidden rows
# included but the rows `done_from` to `done_to` skipped, and schedules the next chunk
# until the last row.
proc ::bw::tablelist::redisplay_col_chunk {win col from done_from done_to} {
    if {![winfo exists $win]} {
        return
    }
    upvar ::tablelist::ns${win}::data data
    unset -nocomplain data($col-bwRedispId)
    if {$col > $data(lastCol) || $from > $data(lastRow)} {
        return
    }
    if {$from >= $done_from && $from <= $done_to} {
        set from [expr {$done_to + 1}]
    }
    set to [expr {min($from + 199, $data(lastRow))}]
    if {$to >= $done_from && $from < $done_from} {
        set to [expr {$done_from - 1}]
    }
    if {$from <= $to} {
        set ::bw::tablelist::redisplay_all_rows 1
        try {
            ::tablelist::redisplayCol $win $col $from $to
        } finally {
            unset ::bw::tablelist::redisplay_all_rows
        }
    }
    if {$to < $data(lastRow)} {
        set data($col-bwRedispId) [after idle \
            [list ::bw::tablelist::redisplay_col_chunk $win $col [expr {$to + 1}] $done_from $done_to]]
    }
}
