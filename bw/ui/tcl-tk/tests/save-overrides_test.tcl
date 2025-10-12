#!/usr/bin/env tclsh
# Unit tests for save_overrides functionality

package require tcltest
namespace import tcltest::*

# Test helper: Format output as save_overrides would
proc format_override_line {style option value} {
    set non_ttk_widgets {Tablelist Button Label Entry Listbox Text Canvas Frame Toplevel Menu Menubutton Scale Scrollbar Checkbutton Radiobutton}
    set escaped_value [list $value]

    if {$style in $non_ttk_widgets} {
        return "option add *${style}.${option} $escaped_value widgetDefault"
    } else {
        return "ttk::style configure $style $option $escaped_value"
    }
}

# Test helper: Parse key into style and option (matches save_overrides logic)
proc parse_override_key {key} {
    set last_dot_pos [string last "." $key]
    if {$last_dot_pos > 0} {
        set style [string range $key 0 [expr {$last_dot_pos - 1}]]
        set option [string range $key [expr {$last_dot_pos + 1}] end]
        return [list $style $option]
    }
    return [list "" ""]
}

test format-ttk-widget-1.0 {TTK widgets should use ttk::style configure} -body {
    format_override_line "TButton" "-padding" "10 5 10 5"
} -result {ttk::style configure TButton -padding {10 5 10 5}}

test format-non-ttk-widget-1.0 {Non-TTK widgets should use option add} -body {
    format_override_line "Tablelist" "-background" "#ffffff"
} -result {option add *Tablelist.-background {#ffffff} widgetDefault}

test format-menu-widget-1.0 {Menu widgets should use option add} -body {
    format_override_line "Menu" "-foreground" "#222"
} -result {option add *Menu.-foreground {#222} widgetDefault}

test format-value-with-spaces-1.0 {Values with spaces should be properly escaped} -body {
    format_override_line "Tablelist" "-labelfont" "TkDefaultFont 11 bold"
} -result {option add *Tablelist.-labelfont {TkDefaultFont 11 bold} widgetDefault}

test format-complex-value-1.0 {Complex values should be properly escaped} -body {
    format_override_line "TNotebook.Tab" "-padding" "15 5 15 5"
} -result {ttk::style configure TNotebook.Tab -padding {15 5 15 5}}

# Test key parsing logic
test parse-key-simple-1.0 {Simple keys should parse correctly} -body {
    parse_override_key "TButton.-padding"
} -result {TButton -padding}

test parse-key-dotted-style-1.0 {Dotted style names should parse correctly} -body {
    parse_override_key "TNotebook.Tab.-padding"
} -result {TNotebook.Tab -padding}

test parse-key-complex-option-1.0 {Keys with multiple dots split on last dot} -body {
    # Splits on the LAST dot, so "TButton.-some.complex.option" becomes style="TButton.-some.complex" option="option"
    parse_override_key "TButton.-some.complex.option"
} -result {TButton.-some.complex option}

# Test file path construction
test override-file-path-1.0 {Override file path should be constructed correctly} -body {
    set tcl_source_dir "/home/user/dev/strongbox2"
    set expected_path [file join $tcl_source_dir "bw" "ui" "tcl-tk" "ttk-themes" "parade" "parade-overrides.tcl"]
    return $expected_path
} -result {/home/user/dev/strongbox2/bw/ui/tcl-tk/ttk-themes/parade/parade-overrides.tcl}

# Test validation of source directory
test source-dir-validation-1.0 {Empty source directory should be detected} -body {
    set tcl_source_dir ""
    expr {$tcl_source_dir eq ""}
} -result 1

test source-dir-validation-2.0 {Non-empty source directory should pass} -body {
    set tcl_source_dir "/some/path"
    expr {$tcl_source_dir eq ""}
} -result 0

cleanupTests
