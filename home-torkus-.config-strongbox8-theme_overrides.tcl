# Strongbox Theme Overrides
# Generated automatically by theme editor
# File created: Tue Sep 30 22:54:45 ACST 2025

# Ensure override namespace exists
if {![namespace exists ttk::theme::parade::overrides]} {
    namespace eval ttk::theme::parade::overrides {
        variable overrides
        array set overrides {}
        proc apply_overrides {} {
            variable overrides
            foreach {key value} [array get overrides] {
                set parts [split $key "."]
                if {[llength $parts] == 2} {
                    set style [lindex $parts 0]
                    set option [lindex $parts 1]
                    ttk::style configure $style $option $value
                }
            }
        }
        proc set_override {style option value} {
            variable overrides
            set key "$style.$option"
            set overrides($key) $value

            # Define known non-TTK widget classes
            set non_ttk_widgets {Tablelist Button Label Entry Listbox Text Canvas Frame Toplevel Menu Menubutton Scale Scrollbar Checkbutton Radiobutton}

            if {$style in $non_ttk_widgets} {
                # For non-TTK widgets, use option database and direct widget configuration
                set pattern "*${style}.${option}"
                catch {option add $pattern $value widgetDefault}

                # Also configure existing widgets of this class
                set all_widgets [list "."]
                set checked {}
                while {[llength $all_widgets] > 0} {
                    set widget [lindex $all_widgets 0]
                    set all_widgets [lrange $all_widgets 1 end]
                    if {[lsearch $checked $widget] != -1} continue
                    lappend checked $widget
                    if {[winfo exists $widget]} {
                        if {[winfo class $widget] eq $style} {
                            catch {$widget configure $option $value}
                        }
                        if {[catch {set children [winfo children $widget]} err] == 0} {
                            set all_widgets [concat $all_widgets $children]
                        }
                    }
                }
            } else {
                # Try TTK style configuration for TTK widgets
                if {[catch {ttk::style configure $style $option $value} err] == 0} {
                    # TTK style configuration succeeded
                    # Special handling for TNotebook.Tab padding to override mapped states
                    if {$style eq "TNotebook.Tab" && $option eq "-padding"} {
                        catch {ttk::style map $style $option {}}
                        catch {ttk::style map $style $option [list selected $value active $value pressed $value {} $value]}
                    }
                } else {
                    # TTK failed, might be a non-TTK widget not in our list
                    # Fall back to option database approach
                    set pattern "*${style}.${option}"
                    catch {option add $pattern $value widgetDefault}

                    # Also configure existing widgets of this class
                    set all_widgets [list "."]
                    set checked {}
                    while {[llength $all_widgets] > 0} {
                        set widget [lindex $all_widgets 0]
                        set all_widgets [lrange $all_widgets 1 end]
                        if {[lsearch $checked $widget] != -1} continue
                        lappend checked $widget
                        if {[winfo exists $widget]} {
                            if {[winfo class $widget] eq $style} {
                                catch {$widget configure $option $value}
                            }
                            if {[catch {set children [winfo children $widget]} err] == 0} {
                                set all_widgets [concat $all_widgets $children]
                            }
                        }
                    }
                }
            }
        }
        proc remove_override {style option} {
            variable overrides
            set key "$style.$option"
            if {[info exists overrides($key)]} {
                unset overrides($key)
                # Force theme refresh
                set current_theme [ttk::style theme use]
                if {$current_theme eq "parade"} {
                    ttk::style theme use clam
                    ttk::style theme use parade
                    apply_overrides
                }
            }
        }
        proc clear_all_overrides {} {
            variable overrides
            array unset overrides
            set current_theme [ttk::style theme use]
            if {$current_theme eq "parade"} {
                ttk::style theme use clam
                ttk::style theme use parade
            }
        }
    }
}

# Apply saved overrides
ttk::theme::parade::overrides::set_override "Menu" "-background" "#ddd"
ttk::theme::parade::overrides::set_override "TNotebook" "-tabmargins" "10 10 0 0"
ttk::theme::parade::overrides::set_override "Parade.Theme" "-tabbg" "#f0f0f0"
ttk::theme::parade::overrides::set_override "Tablelist" "-resizablecolumns" ""
ttk::theme::parade::overrides::set_override "TLabelframe" "-relief" "raised"
ttk::theme::parade::overrides::set_override "Treeview" "-foreground" "#008080"
ttk::theme::parade::overrides::set_override "Tablelist" "-labelforeground" "#222"
ttk::theme::parade::overrides::set_override "Menu" "-foreground" "#222"
ttk::theme::parade::overrides::set_override "Tablelist" "-spacing" "5"
ttk::theme::parade::overrides::set_override "Tablelist" "-selectborderwidth" ""
ttk::theme::parade::overrides::set_override "Tablelist" "-labelpady" "5"
ttk::theme::parade::overrides::set_override "Tablelist" "-borderwidth" "0"
ttk::theme::parade::overrides::set_override "TPanedwindow" "-padding" "0 0 0 0"
ttk::theme::parade::overrides::set_override "TLabel" "-padding" "5 10 5 10"
ttk::theme::parade::overrides::set_override "Tablelist" "-font" "TkDefaultFont 11 \{\}"
ttk::theme::parade::overrides::set_override "Tablelist" "-stripebackground" "#efefef"
ttk::theme::parade::overrides::set_override "Menu" "-borderwidth" "1"
ttk::theme::parade::overrides::set_override "Tablelist" "-selectbackground" "lightsteelblue"
ttk::theme::parade::overrides::set_override "Tablelist" "-background" "#ffffff"
ttk::theme::parade::overrides::set_override "Menu" "-relief" "raised"
ttk::theme::parade::overrides::set_override "Menu" "-padding" "0 0 0 0"
ttk::theme::parade::overrides::set_override "TEntry" "-padding" "5 5 1 5"
ttk::theme::parade::overrides::set_override "Tablelist" "-labelheight" "1"
ttk::theme::parade::overrides::set_override "Parade.Theme" "-font" "TkDefaultFont 12 \{\}"
ttk::theme::parade::overrides::set_override "Parade.Theme" "-padding" "0 0 0 0"
ttk::theme::parade::overrides::set_override "TNotebook" "Tab" "5 0 0 0"
ttk::theme::parade::overrides::set_override "TButton" "-padding" "10 5 10 5"
ttk::theme::parade::overrides::set_override "Tablelist" "-foreground" "#222222"
ttk::theme::parade::overrides::set_override "Treeview.Heading" "-padding" "0 0 0 0"
ttk::theme::parade::overrides::set_override "TNotebook.Tab" "-padding" "15 5 15 5"
ttk::theme::parade::overrides::set_override "TNotebook.Tab" "-font" "TkDefaultFont 12 \{\}"
ttk::theme::parade::overrides::set_override "Tablelist" "-showseparators" "1"
ttk::theme::parade::overrides::set_override "Tablelist" "-labelborderwidth" "1"
ttk::theme::parade::overrides::set_override "Tablelist" "-showhorizseparator" "0"
ttk::theme::parade::overrides::set_override "Tablelist" "-labelfont" "TkDefaultFont 11 bold"
ttk::theme::parade::overrides::set_override "Tablelist" "-fullseparators" "1"
ttk::theme::parade::overrides::set_override "Parade.Theme" "-tabborder" "#d0d0d0"
ttk::theme::parade::overrides::set_override "Tablelist" "-labelbackground" "#eee"
ttk::theme::parade::overrides::set_override "Menu" "-font" "TkDefaultFont 11 \{\}"

# Apply all overrides
ttk::theme::parade::overrides::apply_overrides
