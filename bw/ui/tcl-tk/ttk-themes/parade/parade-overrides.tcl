# Parade Theme Overrides
# This file contains user customizations that override the base parade theme
# Migrated from config directory

namespace eval ttk::theme::parade::overrides {

    # Override storage - maps style.option to custom value
    variable overrides
    array set overrides {}

    # Apply all stored overrides to the current theme
    proc apply_overrides {} {
        variable overrides

        # Define known non-TTK widget classes
        set non_ttk_widgets {Tablelist Button Label Entry Listbox Text Canvas Frame Toplevel Menu Menubutton Scale Scrollbar Checkbutton Radiobutton}

        foreach {key value} [array get overrides] {
            set parts [split $key "."]
            if {[llength $parts] == 2} {
                set style [lindex $parts 0]
                set option [lindex $parts 1]

                # Apply based on widget type
                if {$style in $non_ttk_widgets} {
                    # For non-TTK widgets, use option database AND configure existing widgets
                    set pattern "*${style}.${option}"
                    catch {option add $pattern $value widgetDefault}

                    # Also configure all existing widgets of this class
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
                    # For TTK widgets
                    catch {ttk::style configure $style $option $value}
                }
            }
        }
    }

    # Set an override value for a style option
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
            # TTK widgets - use ttk::style configure
            catch {ttk::style configure $style $option $value}
        }
    }

    # Remove an override (revert to base theme default)
    proc remove_override {style option} {
        variable overrides
        set key "$style.$option"

        if {[info exists overrides($key)]} {
            unset overrides($key)

            # To properly revert, we need to reload the theme
            set current_theme [ttk::style theme use]

            # Temporarily switch to another theme and back to reset styles
            if {$current_theme eq "parade"} {
                ttk::style theme use clam
                ttk::style theme use parade

                # Reapply remaining overrides
                apply_overrides
            }
        }
    }

    # Clear all overrides and revert to base theme
    proc clear_all_overrides {} {
        variable overrides
        array unset overrides

        # Reload the theme to reset to defaults
        set current_theme [ttk::style theme use]
        if {$current_theme eq "parade"} {
            ttk::style theme use clam
            ttk::style theme use parade
        }
    }

    # Get current override value, or empty string if not overridden
    proc get_override {style option} {
        variable overrides
        set key "$style.$option"

        if {[info exists overrides($key)]} {
            return $overrides($key)
        }
        return ""
    }

    # Load overrides from saved values (called when file is sourced)
    proc load_overrides {} {
        variable overrides
        apply_overrides
    }

    # Saved override data
    array set overrides {
        Menu.-background {#ddd}
        TNotebook.-tabmargins {10 10 0 0}
        Parade.Theme.-tabbg {#f0f0f0}
        Tablelist.-resizablecolumns {}
        TLabelframe.-relief raised
        Treeview.-foreground {#008080}
        Tablelist.-labelforeground {#222}
        Menu.-foreground {#222}
        Tablelist.-spacing 5
        Tablelist.-selectborderwidth {}
        Tablelist.-labelpady 5
        Tablelist.-borderwidth 0
        Menu.-borderwidth 1
        Tablelist.-stripebackground {#efefef}
        Tablelist.-font {TkDefaultFont 11 \{\}}
        TLabel.-padding {5 10 5 10}
        TPanedwindow.-padding {0 0 0 0}
        Menu.-padding {0 0 0 0}
        Menu.-relief raised
        Tablelist.-background {#ffffff}
        Tablelist.-selectbackground lightsteelblue
        Parade.Theme.-padding {0 0 0 0}
        Parade.Theme.-font {TkDefaultFont 12 \{\}}
        Tablelist.-labelheight 1
        TEntry.-padding {5 5 1 5}
        Tablelist.-foreground {#222222}
        TButton.-padding {10 5 10 5}
        TNotebook.Tab {5 0 0 0}
        Treeview.Heading.-padding {0 0 0 0}
        Tablelist.-showseparators 1
        TNotebook.Tab.-font {TkDefaultFont 12 \{\}}
        TNotebook.Tab.-padding {15 5 15 5}
        Tablelist.-showhorizseparator 0
        Tablelist.-labelborderwidth 1
        Parade.Theme.-tabborder {#d0d0d0}
        Tablelist.-fullseparators 1
        Tablelist.-labelfont {TkDefaultFont 11 bold}
        Menu.-font {TkDefaultFont 11 \{\}}
        Tablelist.-labelbackground {#eee}
    }
}

# Auto-apply when sourced
ttk::theme::parade::overrides::load_overrides