# stripe assignment for tablelist rows used as a tree.
#
# tablelist stripes rows by their position on screen: expanding a row slides every row
# below it onto a different stripe, so each of them repaints. here a stripe belongs to a
# top-level row and its descendants inherit it: expanding a row changes nothing about the
# rows around it, and a parent and its children read as one block.
#
# pure tcl, no Tk: sourced by `patches.tcl` and by the tests.

namespace eval ::bw::tablelist {}

# returns one 0/1 flag per row in `rows`, 1 for a row in a stripe.
#
# `rows` is a list of `{key parent viewable}` triples in display order, every parent
# before its descendants, `parent` being "root" for a top-level row and `viewable` being
# 1 for a row that is neither elided nor hidden.
#
# viewable top-level rows are grouped into bands of `stripe_height` rows that alternate
# between plain and striped, as tablelist's `-stripeheight` option does. a descendant
# takes the flag of its parent. a row that is not viewable gets 0 and does not advance
# the bands, as it takes no space on screen.
proc ::bw::tablelist::stripe_flags {rows stripe_height} {
    if {$stripe_height < 1} {
        set stripe_height 1
    }
    set count 0
    set in_stripe 0
    set flags {}
    array set flag_of {}
    foreach row $rows {
        lassign $row key parent viewable
        if {!$viewable} {
            lappend flags 0
            continue
        }
        if {$parent eq "root"} {
            set flag $in_stripe
            if {[incr count] == $stripe_height} {
                set count 0
                set in_stripe [expr {!$in_stripe}]
            }
        } elseif {[info exists flag_of($parent)]} {
            set flag $flag_of($parent)
        } else {
            set flag 0
        }
        set flag_of($key) $flag
        lappend flags $flag
    }
    return $flags
}
