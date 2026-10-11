#!/usr/bin/env tclsh
# unit tests for `::bw::tablelist::stripe_flags`

package require tcltest
namespace import tcltest::*

source [file join [file dirname [info script]] .. tablelist-patches stripes.tcl]

test stripe-flags-empty-1.0 {no rows, no flags} -body {
    ::bw::tablelist::stripe_flags {} 1
} -result {}

test stripe-flags-top-level-1.0 {top-level rows alternate} -body {
    set given {{a root 1} {b root 1} {c root 1} {d root 1}}
    ::bw::tablelist::stripe_flags $given 1
} -result {0 1 0 1}

test stripe-flags-top-level-1.1 {a stripe height of two alternates every two rows} -body {
    set given {{a root 1} {b root 1} {c root 1} {d root 1} {e root 1}}
    ::bw::tablelist::stripe_flags $given 2
} -result {0 0 1 1 0}

test stripe-flags-top-level-1.2 {a stripe height below one is treated as one} -body {
    set given {{a root 1} {b root 1}}
    ::bw::tablelist::stripe_flags $given 0
} -result {0 1}

test stripe-flags-children-2.0 {children take their parent's stripe and do not advance it} -body {
    set given {{a root 1} {b root 1} {b1 b 1} {b2 b 1} {c root 1}}
    ::bw::tablelist::stripe_flags $given 1
} -result {0 1 1 1 0}

test stripe-flags-children-2.1 {grandchildren inherit through their parent} -body {
    set given {{a root 1} {b root 1} {b1 b 1} {b1x b1 1} {c root 1}}
    ::bw::tablelist::stripe_flags $given 1
} -result {0 1 1 1 0}

test stripe-flags-children-2.2 {expanding a row leaves the rows below as they were} -body {
    set collapsed {{a root 1} {b root 1} {b1 b 0} {c root 1} {d root 1}}
    set expanded  {{a root 1} {b root 1} {b1 b 1} {c root 1} {d root 1}}
    set before [::bw::tablelist::stripe_flags $collapsed 1]
    set after  [::bw::tablelist::stripe_flags $expanded 1]
    list [lrange $before 3 4] [lrange $after 3 4]
} -result {{0 1} {0 1}}

test stripe-flags-hidden-3.0 {a row that is not viewable gets no stripe and takes no space} -body {
    set given {{a root 1} {b root 0} {c root 1} {d root 1}}
    ::bw::tablelist::stripe_flags $given 1
} -result {0 0 1 0}

test stripe-flags-hidden-3.1 {a viewable child of an unknown parent is plain} -body {
    set given {{a root 1} {x ghost 1} {b root 1}}
    ::bw::tablelist::stripe_flags $given 1
} -result {0 0 1}

cleanupTests
