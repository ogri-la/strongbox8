# Live Theme Property Editor
# Provides real-time editing of TTK widget styles with override persistence

namespace eval theme_editor {

    # Widget references
    variable editor_window ""
    variable widget_path_entry ""
    variable property_frame ""
    variable preview_frame ""
    variable current_widget_class ""
    variable current_style ""

    # Property editing widgets storage
    variable property_widgets
    array set property_widgets {}

    # Widget filtering (kept for compatibility)
    variable filter_var ""




    # Configure canvas scroll region when content changes
    proc configure_scroll_region {canvas} {
        # Use after idle to prevent too frequent updates
        after cancel [info globals canvas_update_id]
        set canvas_update_id [after idle [list $canvas configure -scrollregion [$canvas bbox all]]]
    }

    # Handle mouse wheel scrolling
    proc mouse_wheel_scroll {canvas delta} {
        set units [expr {-($delta / 120) * 3}]
        $canvas yview scroll $units units
    }

    # Configure canvas width to match content frame
    proc configure_canvas_width {canvas content_frame} {
        # Use after idle to prevent too frequent updates during resize
        after cancel [info globals width_update_id]
        set width_update_id [after idle [list theme_editor::do_canvas_width_update $canvas $content_frame]]
    }

    # Helper to actually update canvas width
    proc do_canvas_width_update {canvas content_frame} {
        set canvas_width [winfo width $canvas]
        if {$canvas_width > 1} {
            # Find the window item containing our content frame and set its width
            foreach item [$canvas find all] {
                if {[$canvas type $item] eq "window"} {
                    set widget [$canvas itemcget $item -window]
                    if {$widget eq $content_frame} {
                        $canvas itemconfig $item -width $canvas_width
                        break
                    }
                }
            }
        }
    }

    # Load saved theme overrides from file
    proc load_saved_overrides {} {
        # Determine load location - use XDG config directory
        set config_dir ""
        if {[info exists ::env(HOME)]} {
            set config_dir [file join $::env(HOME) ".config" "strongbox8"]
        } else {
            set config_dir [file join [pwd] "config"]
        }

        set override_file [file join $config_dir "theme_overrides.tcl"]

        # Load overrides if file exists
        if {[file exists $override_file]} {
            if {[catch {
                source $override_file
                puts "Theme overrides loaded from: $override_file"
            } err]} {
                puts "Warning: Failed to load theme overrides: $err"
            }
        }
    }

    # Ensure override namespace exists and has all required functions
    proc ensure_override_namespace {} {
        if {![namespace exists ttk::theme::parade::overrides]} {
            # Create the override namespace if it doesn't exist
            namespace eval ttk::theme::parade::overrides {
                # Override storage - maps style.option to custom value
                variable overrides
                array set overrides {}

                # Apply all stored overrides to the current theme
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

                # Set an override value for a style option
                proc set_override {style option value} {
                    variable overrides
                    set key "$style.$option"
                    set overrides($key) $value

                    # Try TTK style configuration first
                    if {[catch {ttk::style configure $style $option $value} err] == 0} {
                        # TTK style configuration succeeded
                        puts "DEBUG: Applied TTK style override: $style $option $value"

                        # Special handling for TNotebook.Tab padding to override mapped states
                        if {$style eq "TNotebook.Tab" && $option eq "-padding"} {
                            # Clear any existing map for this option and set our override
                            catch {ttk::style map $style $option {}}
                            # Set a new map that uses our override value for all states
                            catch {ttk::style map $style $option [list selected $value active $value pressed $value {} $value]}
                        }
                    } else {
                        # TTK failed, use option database for regular widget classes
                        puts "DEBUG: TTK style configure failed, using option database for class $style"

                        # Use option database to set default for all future widgets of this class
                        set pattern "*${style}${option}"
                        if {[catch {option add $pattern $value} opt_err] == 0} {
                            puts "DEBUG: Added option database entry: $pattern = $value"

                            # Also configure existing widgets of this class
                            set all_widgets [list "."]
                            set checked {}
                            set configured_count 0

                            while {[llength $all_widgets] > 0} {
                                set widget [lindex $all_widgets 0]
                                set all_widgets [lrange $all_widgets 1 end]

                                if {[lsearch $checked $widget] != -1} continue
                                lappend checked $widget

                                if {[winfo exists $widget]} {
                                    if {[winfo class $widget] eq $style} {
                                        # Found a widget of this class, configure it
                                        if {[catch {$widget configure $option $value} config_err] == 0} {
                                            incr configured_count
                                            puts "DEBUG: Applied widget override to $widget: $option $value"
                                        } else {
                                            puts "DEBUG: Failed to configure $widget $option: $config_err"
                                        }
                                    }

                                    # Add children to search
                                    if {[catch {set children [winfo children $widget]} err] == 0} {
                                        set all_widgets [concat $all_widgets $children]
                                    }
                                }
                            }

                            puts "DEBUG: Applied widget-based override to $configured_count widgets of class $style"
                        } else {
                            puts "DEBUG: Failed to add option database entry: $opt_err"
                        }
                    }
                }

                # Remove an override (revert to base theme default)
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

                # Clear all overrides and revert to base theme
                proc clear_all_overrides {} {
                    variable overrides
                    array unset overrides
                    set current_theme [ttk::style theme use]
                    if {$current_theme eq "parade"} {
                        ttk::style theme use clam
                        ttk::style theme use parade
                    }
                }

                # Basic save function (simplified)
                proc save_overrides {} {
                    # For now, just show a message that overrides are active
                    # Full persistence would require file writing
                }
            }
        }

        # Ensure all required functions exist (in case loaded from incomplete saved file)
        if {![catch {namespace eval ttk::theme::parade::overrides {info procs remove_override}} result] && $result eq ""} {
            # Add missing remove_override function
            namespace eval ttk::theme::parade::overrides {
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
            }
        }

        if {![catch {namespace eval ttk::theme::parade::overrides {info procs clear_all_overrides}} result] && $result eq ""} {
            # Add missing clear_all_overrides function
            namespace eval ttk::theme::parade::overrides {
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
    }

    # Helper function to apply style overrides safely
    proc apply_style_override {style_name option value} {
        ensure_override_namespace
        ttk::theme::parade::overrides::set_override $style_name $option $value

        # Force theme refresh to ensure changes are visible
        refresh_theme
    }

    # Force refresh of the current theme to apply changes
    proc refresh_theme {} {
        set current_theme [ttk::style theme use]
        if {$current_theme ne ""} {
            # Method 1: Try to force widget updates by temporarily switching themes
            if {[catch {
                set temp_theme "clam"
                if {$current_theme eq "clam"} {
                    set temp_theme "alt"
                }
                ttk::style theme use $temp_theme
                ttk::style theme use $current_theme
            }]} {
                # Method 2: If theme switching fails, force display update
                catch {
                    update idletasks
                }
            }
        }
    }

    # Debug function: Change notebook tab font size directly
    proc debug_change_tab_font_size {value} {
        variable editor_window

        # Round the value to integer
        set font_size [expr {int($value)}]

        # Update the value label - check both embedded and standalone paths
        set value_widget ""
        if {[winfo exists $editor_window.main.debug_frame.content.value]} {
            set value_widget $editor_window.main.debug_frame.content.value
        } elseif {[winfo exists $editor_window.main.nb.debug.debug_frame.content.value]} {
            set value_widget $editor_window.main.nb.debug.debug_frame.content.value
        }

        if {$value_widget ne "" && [winfo exists $value_widget]} {
            $value_widget configure -text $font_size
        }

        # Apply the font change using the override system so it gets tracked
        ensure_override_namespace
        set font_spec [list TkDefaultFont $font_size normal]
        ttk::theme::parade::overrides::set_override "TNotebook.Tab" "-font" $font_spec

        # Force theme refresh and update to ensure the change is visible
        refresh_theme
        update idletasks
    }

    # Debug function to list all discoverable widgets
    proc debug_list_all_widgets {} {
        puts "=== DEBUG: Widget Discovery ==="

        puts "Root children: [winfo children .]"
        puts "Window stack order: [wm stackorder .]"

        # Try to find all toplevel windows
        foreach w [winfo children .] {
            if {[winfo exists $w]} {
                puts "Child: $w ([winfo class $w])"
                if {[winfo class $w] eq "Toplevel"} {
                    puts "  Toplevel children: [winfo children $w]"
                }
            }
        }

        # Check if there are any notebooks anywhere
        set all_widgets [list "."]
        set checked {}
        while {[llength $all_widgets] > 0} {
            set widget [lindex $all_widgets 0]
            set all_widgets [lrange $all_widgets 1 end]

            if {[lsearch $checked $widget] != -1} continue
            lappend checked $widget

            if {[winfo exists $widget]} {
                set class [winfo class $widget]
                if {$class eq "TNotebook"} {
                    puts "Found notebook: $widget"
                    if {[catch {set tabs [$widget tabs]} err] == 0} {
                        puts "  Tabs: $tabs"
                    }
                }

                # Add children to search
                if {[catch {set children [winfo children $widget]} err] == 0} {
                    set all_widgets [concat $all_widgets $children]
                }
            }
        }
        puts "=== END DEBUG ==="
    }


    # Load properties for the selected TTK class
    proc load_class_properties {} {
        variable editor_window

        puts "DEBUG: load_class_properties called"

        # Get selected class from the combo box
        set selected_class ""

        # Get from the combo box (updated path)
        if {[winfo exists $editor_window.main.class_frame.inner.combo]} {
            set selected_class [$editor_window.main.class_frame.inner.combo get]
            puts "DEBUG: Found selected class: '$selected_class'"
        } else {
            puts "DEBUG: Combo box does not exist at $editor_window.main.class_frame.inner.combo"
        }

        if {$selected_class eq ""} {
            puts "DEBUG: No class selected, returning"
            return
        }

        # Set the widget path variable to the class name for compatibility
        set ::widget_path_var $selected_class

        # Load properties for this style class
        puts "DEBUG: Calling inspect_style_class for '$selected_class'"
        inspect_style_class $selected_class
    }

    # Inspect a TTK style class instead of individual widgets
    proc inspect_style_class {style_class} {
        variable property_frame

        puts "DEBUG: inspect_style_class called with style_class: '$style_class'"

        if {$property_frame eq ""} return

        # Clear existing properties
        foreach child [winfo children $property_frame] {
            destroy $child
        }

        # Create property editors for this style class
        create_style_property_editors $style_class
    }

    # Discover actually used TTK style classes by scanning the widget tree
    proc discover_available_styles {} {
        set discovered_styles {}
        set widget_classes {}

        puts "DEBUG: Scanning widget tree for actual TTK widgets and regular Tk widgets in use..."

        # Walk the entire widget tree to find actual widgets
        set all_widgets [list "."]
        set checked {}

        while {[llength $all_widgets] > 0} {
            set widget [lindex $all_widgets 0]
            set all_widgets [lrange $all_widgets 1 end]

            if {[lsearch $checked $widget] != -1} continue
            lappend checked $widget

            if {[winfo exists $widget]} {
                set widget_class [winfo class $widget]

                # Check if this is a TTK widget (they usually start with T or are special cases)
                if {[string match "T*" $widget_class] ||
                    $widget_class eq "Treeview" ||
                    $widget_class eq "Progressbar" ||
                    $widget_class eq "Scale" ||
                    $widget_class eq "Scrollbar"} {

                    # Try to get the actual style being used
                    set style_name ""
                    if {[catch {$widget cget -style} custom_style] == 0 && $custom_style ne ""} {
                        set style_name $custom_style
                    } else {
                        # Use the default style name for this widget class
                        set style_name $widget_class
                    }

                    if {$style_name ne "" && [lsearch $discovered_styles $style_name] == -1} {
                        lappend discovered_styles $style_name
                        puts "DEBUG: Found TTK widget: $widget (class: $widget_class, style: $style_name)"
                    }
                } else {
                    # For non-TTK widgets, use the widget class name as the "style"
                    # Skip basic container widgets that aren't usually styled
                    if {$widget_class ni {"Tk" "Frame" "Toplevel" "Canvas" "Text"}} {
                        if {[lsearch $discovered_styles $widget_class] == -1} {
                            lappend discovered_styles $widget_class
                            puts "DEBUG: Found regular Tk widget: $widget (class: $widget_class)"
                        }
                    }
                }

                # Add children to search
                if {[catch {set children [winfo children $widget]} err] == 0} {
                    set all_widgets [concat $all_widgets $children]
                }
            }
        }

        # Also add some common styles that might not be instantiated yet but are available
        set common_fallbacks {
            "TButton" "TCheckbutton" "TCombobox" "TEntry" "TFrame" "TLabel"
            "TLabelframe" "TNotebook" "TNotebook.Tab" "TPanedwindow"
            "TRadiobutton" "TScale" "TScrollbar" "Treeview" "Treeview.Heading"
        }

        foreach style $common_fallbacks {
            if {[lsearch $discovered_styles $style] == -1} {
                # Test if this style actually exists in the theme
                if {[catch {ttk::style configure $style} result] == 0 && [llength $result] > 0} {
                    lappend discovered_styles $style
                    puts "DEBUG: Added available style: $style"
                }
            }
        }

        # Sort the styles for better organization
        set discovered_styles [lsort $discovered_styles]

        puts "DEBUG: Discovered [llength $discovered_styles] total styles: $discovered_styles"
        return $discovered_styles
    }

    # Refresh the style class list by re-scanning the widget tree
    proc refresh_style_classes {} {
        variable editor_window

        puts "DEBUG: Refreshing style classes..."

        # Re-discover available styles
        set ttk_classes [discover_available_styles]

        # Update the combo box values
        if {[winfo exists $editor_window.main.class_frame.inner.combo]} {
            $editor_window.main.class_frame.inner.combo configure -values $ttk_classes
            puts "DEBUG: Updated combo box with [llength $ttk_classes] style classes"
        } else {
            puts "DEBUG: Combo box not found for refresh"
        }
    }

    # Discover actual available properties for a TTK style or widget class
    proc discover_style_properties {style_class} {
        set discovered_properties {}

        # First try TTK style configuration
        if {[catch {ttk::style configure $style_class} config_result] == 0 && [llength $config_result] > 0} {
            puts "DEBUG: TTK style configure for $style_class returned: $config_result"

            # Parse TTK style configuration
            for {set i 0} {$i < [llength $config_result]} {incr i 2} {
                set option [lindex $config_result $i]
                set value [lindex $config_result [expr {$i + 1}]]

                if {$option ne "" && [string match "-*" $option]} {
                    set type [determine_property_type $option $value]
                    set label [format_option_label $option]
                    lappend discovered_properties [list $option $type $label]
                    puts "DEBUG: Discovered TTK property: $option ($type) - $label"
                }
            }

            # Also try to discover TTK mapped properties
            if {[catch {ttk::style map $style_class} map_result] == 0} {
                puts "DEBUG: TTK style map for $style_class returned: $map_result"

                for {set i 0} {$i < [llength $map_result]} {incr i 2} {
                    set option [lindex $map_result $i]

                    if {$option ne "" && [string match "-*" $option]} {
                        # Check if we already have this option from configure
                        set found 0
                        foreach prop $discovered_properties {
                            if {[lindex $prop 0] eq $option} {
                                set found 1
                                break
                            }
                        }

                        if {!$found} {
                            set type [determine_property_type $option ""]
                            set label [format_option_label $option]
                            lappend discovered_properties [list $option $type $label]
                            puts "DEBUG: Discovered TTK mapped property: $option ($type) - $label"
                        }
                    }
                }
            }
        } else {
            puts "DEBUG: $style_class is not a TTK style or returned empty results, trying widget-based discovery"

            # Try to find actual widgets of this class and discover their properties
            set discovered_properties [discover_widget_class_properties $style_class]
        }

        return $discovered_properties
    }

    # Discover properties for non-TTK widget classes by examining actual widgets
    proc discover_widget_class_properties {widget_class} {
        set discovered_properties {}

        puts "DEBUG: Searching for widgets of class '$widget_class'"

        # Walk the widget tree to find widgets of this class
        set all_widgets [list "."]
        set checked {}
        set sample_widget ""

        while {[llength $all_widgets] > 0} {
            set widget [lindex $all_widgets 0]
            set all_widgets [lrange $all_widgets 1 end]

            if {[lsearch $checked $widget] != -1} continue
            lappend checked $widget

            if {[winfo exists $widget]} {
                if {[winfo class $widget] eq $widget_class} {
                    set sample_widget $widget
                    break
                }

                # Add children to search
                if {[catch {set children [winfo children $widget]} err] == 0} {
                    set all_widgets [concat $all_widgets $children]
                }
            }
        }

        if {$sample_widget ne ""} {
            puts "DEBUG: Found sample widget: $sample_widget"

            # Try to get the widget's configuration options
            if {[catch {$sample_widget configure} config_result] == 0} {
                puts "DEBUG: Widget configure returned [llength $config_result] options"

                foreach config_line $config_result {
                    if {[llength $config_line] >= 2} {
                        set option [lindex $config_line 0]
                        set current_value [lindex $config_line end]

                        if {$option ne "" && [string match "-*" $option]} {
                            set type [determine_property_type $option $current_value]
                            set label [format_option_label $option]
                            lappend discovered_properties [list $option $type $label]
                            puts "DEBUG: Discovered widget property: $option ($type) - $label"
                        }
                    }
                }
            } else {
                puts "DEBUG: Cannot get configuration for $sample_widget"
            }
        } else {
            puts "DEBUG: No widgets of class '$widget_class' found in widget tree"
        }

        return $discovered_properties
    }

    # Original TTK-only discovery (kept for reference but replaced above)
    proc discover_ttk_style_properties_old {style_class} {
        set discovered_properties {}

        # Try to get the style configuration
        if {[catch {ttk::style configure $style_class} config_result]} {
            puts "DEBUG: Cannot get style configuration for $style_class"
            return {}
        }

        puts "DEBUG: Style configure for $style_class returned: $config_result"

        # Parse the configuration result - it should be a list of option-value pairs
        for {set i 0} {$i < [llength $config_result]} {incr i 2} {
            set option [lindex $config_result $i]
            set value [lindex $config_result [expr {$i + 1}]]

            if {$option ne "" && [string match "-*" $option]} {
                # Determine property type and label from the option name
                set type [determine_property_type $option $value]
                set label [format_option_label $option]

                lappend discovered_properties [list $option $type $label]
                puts "DEBUG: Discovered property: $option ($type) - $label"
            }
        }

        # Also try to discover mapped properties
        if {[catch {ttk::style map $style_class} map_result] == 0} {
            puts "DEBUG: Style map for $style_class returned: $map_result"

            # Parse mapped properties - these are option followed by state-value lists
            for {set i 0} {$i < [llength $map_result]} {incr i 2} {
                set option [lindex $map_result $i]

                if {$option ne "" && [string match "-*" $option]} {
                    # Check if we already have this option from configure
                    set found 0
                    foreach prop $discovered_properties {
                        if {[lindex $prop 0] eq $option} {
                            set found 1
                            break
                        }
                    }

                    # If not found in configure, add it
                    if {!$found} {
                        set type [determine_property_type $option ""]
                        set label [format_option_label $option]

                        lappend discovered_properties [list $option $type $label]
                        puts "DEBUG: Discovered mapped property: $option ($type) - $label"
                    }
                }
            }
        }

        return $discovered_properties
    }

    # Format an option name into a human-readable label
    proc format_option_label {option} {
        # Remove the leading dash and convert to title case
        set base_name [string range $option 1 end]

        # Handle common cases
        switch $base_name {
            "background" { return "Background Color" }
            "foreground" { return "Text Color" }
            "font" { return "Font" }
            "relief" { return "Border Relief" }
            "borderwidth" { return "Border Width" }
            "bordercolor" { return "Border Color" }
            "padding" { return "Padding" }
            "lightcolor" { return "Light Color" }
            "darkcolor" { return "Dark Color" }
            "troughcolor" { return "Trough Color" }
            "focuscolor" { return "Focus Color" }
            "fieldbackground" { return "Field Background" }
            "insertcolor" { return "Cursor Color" }
            "selectbackground" { return "Selection Background" }
            "selectforeground" { return "Selection Text" }
            "tabmargins" { return "Tab Margins" }
            "gripcount" { return "Grip Count" }
            "sashthickness" { return "Sash Thickness" }
            default {
                # Convert camelCase or underscore_case to spaced Title Case
                set words [regexp -all -inline {[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\b)} $base_name]
                if {[llength $words] == 0} {
                    # Fallback: split on underscores or just capitalize
                    set words [split $base_name "_"]
                }

                set formatted_words {}
                foreach word $words {
                    lappend formatted_words [string totitle $word]
                }
                return [join $formatted_words " "]
            }
        }
    }

    # Create property editors specifically for TTK style classes
    proc create_style_property_editors {style_class} {
        variable property_frame

        puts "DEBUG: Discovering properties for style class: $style_class"

        # Discover actual available properties for this style
        set style_properties [discover_style_properties $style_class]

        if {[llength $style_properties] == 0} {
            puts "DEBUG: No properties discovered for $style_class - style may not support configuration"
            # Don't provide fallback properties - if a style has no properties, it's not configurable
            return
        }

        # Get actual current values for the discovered properties
        set properties_with_values {}
        foreach prop_def $style_properties {
            set option [lindex $prop_def 0]
            set type [lindex $prop_def 1]
            set label [lindex $prop_def 2]

            # Get the current value using our override-aware function
            set current_value [get_style_option_value $style_class $option]

            puts "DEBUG: Property $option for $style_class has value: '$current_value'"
            puts "DEBUG: Type: '$type', Label: '$label'"

            lappend properties_with_values [list $option $current_value $type $label]
        }

        # Create section for style properties
        create_property_section "Style Properties" $properties_with_values $style_class "style" 0
    }

    # Create and show the theme editor window
    proc create_editor {} {
        variable editor_window
        variable widget_path_entry
        variable property_frame
        variable preview_frame

        if {$editor_window ne "" && [winfo exists $editor_window]} {
            # Editor already exists, just raise it
            wm deiconify $editor_window
            raise $editor_window
            return
        }

        # Create main editor window
        set editor_window .theme_editor
        toplevel $editor_window
        wm title $editor_window "Live Theme Property Editor"
        wm geometry $editor_window "800x600"

        # Create main frame structure
        ttk::frame $editor_window.main
        pack $editor_window.main -fill both -expand 1 -padx 10 -pady 10

        # TTK Style Class selector
        ttk::labelframe $editor_window.main.class_frame -text "TTK Style Class"
        pack $editor_window.main.class_frame -fill x -pady {0 10}

        # Create inner frame for combo box and refresh button
        ttk::frame $editor_window.main.class_frame.inner
        pack $editor_window.main.class_frame.inner -fill x -padx 10 -pady 5

        ttk::combobox $editor_window.main.class_frame.inner.combo -state readonly -width 25

        # Discover available TTK style classes dynamically
        set ttk_classes [discover_available_styles]
        $editor_window.main.class_frame.inner.combo configure -values $ttk_classes
        $editor_window.main.class_frame.inner.combo set "TNotebook.Tab"

        # Refresh button to re-scan widget tree
        ttk::button $editor_window.main.class_frame.inner.refresh -text "Refresh" -width 10 \
            -command {theme_editor::refresh_style_classes}

        # Bind selection to load properties
        bind $editor_window.main.class_frame.inner.combo <<ComboboxSelected>> {theme_editor::load_class_properties}

        pack $editor_window.main.class_frame.inner.combo -side left -fill x -expand 1
        pack $editor_window.main.class_frame.inner.refresh -side right -padx {10 0}

        # Load initial properties for default selection
        after idle {theme_editor::load_class_properties}

        # Properties section
        ttk::labelframe $editor_window.main.prop_frame -text "Style Properties"
        set property_frame $editor_window.main.prop_frame.content

        # Create frame for properties (scrolling removed for now)
        ttk::frame $property_frame
        pack $editor_window.main.prop_frame -fill both -expand 1 -pady {10 0}
        pack $property_frame -fill both -expand 1 -padx 10 -pady 10

        # Debug section - guaranteed to work controls
        ttk::labelframe $editor_window.main.debug_frame -text "Debug: Notebook Tab Font Size"
        pack $editor_window.main.debug_frame -fill x -pady {10 0}

        ttk::frame $editor_window.main.debug_frame.content
        pack $editor_window.main.debug_frame.content -fill x -padx 10 -pady 5

        ttk::label $editor_window.main.debug_frame.content.label -text "Tab Font Size:"
        ttk::scale $editor_window.main.debug_frame.content.scale -from 8 -to 20 -orient horizontal \
            -command {theme_editor::debug_change_tab_font_size}
        ttk::label $editor_window.main.debug_frame.content.value -text "12" -width 3

        # Set initial value
        $editor_window.main.debug_frame.content.scale set 12

        pack $editor_window.main.debug_frame.content.label -side left -padx {0 10}
        pack $editor_window.main.debug_frame.content.scale -side left -fill x -expand 1 -padx {0 10}
        pack $editor_window.main.debug_frame.content.value -side left

        # Control buttons
        ttk::frame $editor_window.main.controls
        pack $editor_window.main.controls -fill x -pady {10 0}

        ttk::button $editor_window.main.controls.save_btn -text "Save Overrides" \
            -command {theme_editor::save_overrides}
        ttk::button $editor_window.main.controls.revert_btn -text "Revert All" \
            -command {theme_editor::revert_all}
        ttk::button $editor_window.main.controls.close_btn -text "Close" \
            -command {theme_editor::close_editor}

        pack $editor_window.main.controls.save_btn -side left -padx {0 10}
        pack $editor_window.main.controls.revert_btn -side left -padx {0 10}
        pack $editor_window.main.controls.close_btn -side right

        # Initial message
        ttk::label $property_frame.msg -text "Select a TTK style class above to edit its properties. Changes apply immediately to all widgets of that type."
        pack $property_frame.msg -pady 20
    }

















    # Create preview widgets to show live changes
    proc create_preview_widgets {parent} {
        ttk::frame $parent.widgets
        pack $parent.widgets -fill both -expand 1 -padx 10 -pady 10

        # Notebook with tabs (common target for styling)
        ttk::notebook $parent.widgets.nb
        ttk::frame $parent.widgets.nb.tab1
        ttk::frame $parent.widgets.nb.tab2
        $parent.widgets.nb add $parent.widgets.nb.tab1 -text "Tab One"
        $parent.widgets.nb add $parent.widgets.nb.tab2 -text "Tab Two"
        pack $parent.widgets.nb -fill x -pady {0 10}

        # Buttons
        ttk::frame $parent.widgets.buttons
        ttk::button $parent.widgets.buttons.btn1 -text "Normal Button"
        ttk::button $parent.widgets.buttons.btn2 -text "Another Button"
        pack $parent.widgets.buttons.btn1 -side left -padx {0 10}
        pack $parent.widgets.buttons.btn2 -side left
        pack $parent.widgets.buttons -fill x -pady {0 10}

        # Text entry
        ttk::frame $parent.widgets.entries
        ttk::label $parent.widgets.entries.lbl -text "Text Entry:"
        ttk::entry $parent.widgets.entries.ent
        $parent.widgets.entries.ent insert 0 "Sample text"
        pack $parent.widgets.entries.lbl -side left -padx {0 10}
        pack $parent.widgets.entries.ent -side left -fill x -expand 1
        pack $parent.widgets.entries -fill x -pady {0 10}

        # Scrollbars and listbox
        ttk::frame $parent.widgets.scroll_frame
        listbox $parent.widgets.scroll_frame.list -height 6
        ttk::scrollbar $parent.widgets.scroll_frame.scroll -command [list $parent.widgets.scroll_frame.list yview]
        $parent.widgets.scroll_frame.list configure -yscrollcommand [list $parent.widgets.scroll_frame.scroll set]

        for {set i 1} {$i <= 10} {incr i} {
            $parent.widgets.scroll_frame.list insert end "List Item $i"
        }

        pack $parent.widgets.scroll_frame.scroll -side right -fill y
        pack $parent.widgets.scroll_frame.list -side left -fill both -expand 1
        pack $parent.widgets.scroll_frame -fill both -expand 1
    }

    # Inspect a widget and show its editable properties
    proc inspect_widget {} {
        variable widget_path_entry
        variable property_frame
        variable current_widget_class
        variable current_style

        # Get widget path from entry
        set widget_path [set ::widget_path_var]

        if {$widget_path eq ""} {
            tk_messageBox -title "Error" -message "Please enter a widget path" -type ok -icon error
            return
        }

        # Check if widget exists
        if {![winfo exists $widget_path]} {
            tk_messageBox -title "Error" -message "Widget '$widget_path' does not exist" -type ok -icon error
            return
        }

        # Get widget class and style
        set current_widget_class [winfo class $widget_path]

        # For TTK widgets, get the style name
        if {[catch {$widget_path cget -style} style_result]} {
            # Not a TTK widget or no custom style - use class name as style
            set current_style $current_widget_class
        } else {
            if {$style_result eq ""} {
                # Default style - use class name with T prefix if it's a TTK widget
                if {[string match "Ttk*" $current_widget_class]} {
                    set current_style "T[string range $current_widget_class 3 end]"
                } else {
                    set current_style $current_widget_class
                }
            } else {
                set current_style $style_result
            }
        }

        # Widget path set for class-based styling compatibility

        # Clear existing property widgets
        foreach child [winfo children $property_frame] {
            destroy $child
        }

        # Show widget info
        create_widget_info_display $current_widget_class $current_style

        # Create property editors for common style options
        create_property_editors $current_style
    }

    # Display basic widget information
    proc create_widget_info_display {widget_class style_name} {
        variable property_frame

        ttk::frame $property_frame.info
        pack $property_frame.info -fill x -pady {0 10}

        set widget_path [set ::widget_path_var]

        ttk::label $property_frame.info.class_lbl -text "Widget Class:"
        ttk::label $property_frame.info.class_val -text $widget_class -font {TkDefaultFont 0 bold}
        ttk::label $property_frame.info.style_lbl -text "Style Name:"
        ttk::label $property_frame.info.style_val -text $style_name -font {TkDefaultFont 0 bold}
        ttk::label $property_frame.info.path_lbl -text "Widget Path:"
        ttk::label $property_frame.info.path_val -text $widget_path -font {TkDefaultFont 0 bold} -foreground "#FF8C00"

        grid $property_frame.info.class_lbl -row 0 -column 0 -sticky w -padx {0 10}
        grid $property_frame.info.class_val -row 0 -column 1 -sticky w
        grid $property_frame.info.style_lbl -row 1 -column 0 -sticky w -padx {0 10}
        grid $property_frame.info.style_val -row 1 -column 1 -sticky w
        grid $property_frame.info.path_lbl -row 2 -column 0 -sticky w -padx {0 10}
        grid $property_frame.info.path_val -row 2 -column 1 -sticky w
    }

    # Discover and create property editors for actual widget properties
    proc create_property_editors {style_name} {
        variable property_frame

        # Get widget path from global variable
        set widget_path [set ::widget_path_var]

        if {![winfo exists $widget_path]} {
            ttk::label $property_frame.error -text "Widget no longer exists"
            pack $property_frame.error -pady 10
            return
        }

        # Get actual widget configuration options
        set widget_options {}
        if {[catch {$widget_path configure} config_result]} {
            # Not a configurable widget, show style properties instead
            set widget_options [get_style_properties $style_name]
        } else {
            # Parse widget configuration options
            foreach config_line $config_result {
                if {[llength $config_line] >= 2} {
                    set option [lindex $config_line 0]
                    set current_value [lindex $config_line end]
                    lappend widget_options [list $option $current_value "widget"]
                }
            }
        }

        # Also get style properties
        set style_options [get_style_properties $style_name]

        # Create sections for widget vs style properties
        if {[llength $widget_options] > 0} {
            create_property_section "Widget Properties" $widget_options $widget_path "widget" 0
        }

        if {[llength $style_options] > 0} {
            create_property_section "Style Properties" $style_options $style_name "style" [llength $widget_options]
        }

        # Add custom property entry
        create_custom_property_editor $style_name [expr {[llength $widget_options] + [llength $style_options]}]
    }

    # Get style properties that are actually configured
    proc get_style_properties {style_name} {
        puts "DEBUG: get_style_properties called for style_name: '$style_name'"
        set properties {}

        # Common TTK style options to check
        set common_options {-padding -font -foreground -background -borderwidth -relief -anchor -justify}
        puts "DEBUG: Will check these options: $common_options"

        foreach option $common_options {
            set current_value [get_style_option_value $style_name $option]
            if {$current_value ne ""} {
                lappend properties [list $option $current_value "style"]
            }
        }

        return $properties
    }

    # Create a section of property editors
    proc create_property_section {section_title properties target target_type start_row} {
        variable property_frame

        if {[llength $properties] == 0} return

        # Section header
        ttk::labelframe $property_frame.section_[string tolower [string map {" " "_"} $section_title]] -text $section_title
        pack $property_frame.section_[string tolower [string map {" " "_"} $section_title]] -fill x -pady {5 10}

        set section_frame $property_frame.section_[string tolower [string map {" " "_"} $section_title]].content
        ttk::frame $section_frame
        pack $section_frame -fill x -padx 10 -pady 5

        set row 0
        foreach prop_info $properties {
            set option [lindex $prop_info 0]
            set current_value [lindex $prop_info 1]

            puts "DEBUG: Processing property: $prop_info"
            puts "DEBUG: Property has [llength $prop_info] elements"

            # Check if we have a full property definition with type and label
            if {[llength $prop_info] >= 4} {
                set type [lindex $prop_info 2]
                set label [lindex $prop_info 3]
                puts "DEBUG: Using provided label: '$label'"
            } else {
                # Fall back to old behavior for compatibility
                set label [string range $option 1 end]
                set type [determine_property_type $option $current_value]
                puts "DEBUG: Generated label from option: '$label'"
            }

            puts "DEBUG: Final label for $option: '$label'"
            create_property_editor_in_section $section_frame $target $option $label $type $current_value $row $target_type
            incr row
        }
    }

    # Determine the appropriate editor type for a property
    proc determine_property_type {option current_value} {
        switch -glob $option {
            "-padding" { return "padding" }
            "-font" { return "font" }
            "*color*" -
            "*foreground*" -
            "*background*" { return "color" }
            "*width*" -
            "*size*" { return "integer" }
            "-relief" { return "relief" }
            "-state" { return "state" }
            "-anchor" { return "anchor" }
            "-justify" { return "justify" }
            default {
                # Try to guess from value
                if {[string is integer -strict $current_value]} {
                    return "integer"
                } elseif {[string match "#*" $current_value]} {
                    return "color"
                } else {
                    return "text"
                }
            }
        }
    }

    # Get current value of a style option
    proc get_style_option_value {style_name option} {
        # First check if there's an override for this style/option
        ensure_override_namespace
        if {[namespace exists ttk::theme::parade::overrides]} {
            variable ttk::theme::parade::overrides::overrides
            set key "$style_name.$option"

            # Debug output
            puts "DEBUG: Checking override for key: '$key'"
            puts "DEBUG: Available override keys: [array names overrides]"

            if {[info exists overrides($key)]} {
                puts "DEBUG: Found override value: $overrides($key)"
                return $overrides($key)
            } else {
                puts "DEBUG: No override found for key: '$key'"
            }
        } else {
            puts "DEBUG: Override namespace does not exist"
        }

        # Check if this is a TTK style or regular widget class
        if {[catch {ttk::style lookup $style_name $option} result] == 0} {
            # TTK style - use style lookup
            puts "DEBUG: TTK style lookup result for $style_name $option: '$result'"
            return $result
        } else {
            # Regular widget class - check option database first, then widget-based lookup
            puts "DEBUG: TTK style lookup failed, trying option database for $style_name $option"

            # First try to get value from option database
            set pattern "*${style_name}${option}"
            if {[catch {option get . $style_name $option} option_value] == 0 && $option_value ne ""} {
                puts "DEBUG: Option database lookup result for $style_name $option: '$option_value'"
                return $option_value
            }

            # If not in option database, find a sample widget and get its current value
            puts "DEBUG: Option database lookup failed, trying widget-based lookup for $style_name $option"

            # Walk the widget tree to find a widget of this class
            set all_widgets [list "."]
            set checked {}

            while {[llength $all_widgets] > 0} {
                set widget [lindex $all_widgets 0]
                set all_widgets [lrange $all_widgets 1 end]

                if {[lsearch $checked $widget] != -1} continue
                lappend checked $widget

                if {[winfo exists $widget]} {
                    if {[winfo class $widget] eq $style_name} {
                        # Found a widget of this class, get its current value
                        if {[catch {$widget cget $option} current_value] == 0} {
                            puts "DEBUG: Widget-based lookup result for $style_name $option: '$current_value'"
                            return $current_value
                        }
                    }

                    # Add children to search
                    if {[catch {set children [winfo children $widget]} err] == 0} {
                        set all_widgets [concat $all_widgets $children]
                    }
                }
            }

            puts "DEBUG: No widget found for class $style_name or option $option not supported"
            return ""
        }
    }

    # Create an editor for a specific property (legacy function)
    proc create_property_editor {style_name option label type current_value row} {
        variable property_frame
        set frame_name "$property_frame.prop_$row"
        ttk::frame $frame_name
        pack $frame_name -fill x -pady 2
        create_property_editor_in_section $frame_name $style_name $option $label $type $current_value $row "style"
    }

    # Create an editor for a specific property within a section
    proc create_property_editor_in_section {parent_frame target option label type current_value row target_type} {
        variable property_widgets

        set frame_name "$parent_frame.prop_$row"
        ttk::frame $frame_name
        pack $frame_name -fill x -pady 2

        ttk::label $frame_name.label -text "$label:" -width 15 -anchor w
        pack $frame_name.label -side left -padx {0 10}

        # Create appropriate editor based on type
        switch $type {
            "padding" {
                create_padding_editor $frame_name $target $option $current_value $target_type
            }
            "font" {
                create_font_editor $frame_name $target $option $current_value $target_type
            }
            "color" {
                create_color_editor $frame_name $target $option $current_value $target_type
            }
            "integer" {
                create_integer_editor $frame_name $target $option $current_value $target_type
            }
            "relief" {
                create_relief_editor $frame_name $target $option $current_value $target_type
            }
            "state" {
                create_state_editor $frame_name $target $option $current_value $target_type
            }
            "anchor" {
                create_anchor_editor $frame_name $target $option $current_value $target_type
            }
            "justify" {
                create_justify_editor $frame_name $target $option $current_value $target_type
            }
            default {
                create_text_editor $frame_name $target $option $current_value $target_type
            }
        }

        # Add revert button (only for style properties)
        if {$target_type eq "style"} {
            ttk::button $frame_name.revert -text "Revert" -width 8 \
                -command [list theme_editor::revert_property $target $option]
            pack $frame_name.revert -side right -padx {10 0}
        }
    }

    # Create font editor with family, size, and style options
    proc create_font_editor {parent target option current_value {target_type "style"}} {
        # Parse current font
        set font_family "TkDefaultFont"
        set font_size "0"
        set font_style ""

        if {$current_value ne ""} {
            set font_parts [split $current_value]
            if {[llength $font_parts] >= 1} {
                set font_family [lindex $font_parts 0]
            }
            if {[llength $font_parts] >= 2} {
                set font_size [lindex $font_parts 1]
            }
            if {[llength $font_parts] >= 3} {
                set font_style [lindex $font_parts 2]
            }
        }

        # If font size is 0, use a reasonable default for the editor
        if {$font_size eq "0"} {
            # Use 12 as a reasonable default for font size editing
            # (0 means "use default" but in the editor we want to show an actual number)
            set font_size "12"
        }

        ttk::frame $parent.font
        pack $parent.font -side left -fill x -expand 1

        # Font family (simplified list)
        set families {"TkDefaultFont" "Arial" "Helvetica" "Times" "Courier" "Verdana"}
        ttk::combobox $parent.font.family -values $families -width 12 -state readonly
        $parent.font.family set $font_family

        # Font size
        ttk::spinbox $parent.font.size -from 6 -to 24 -width 6 \
            -command [list theme_editor::update_font $target $option $parent.font $target_type]
        $parent.font.size set $font_size

        # Font style
        ttk::checkbutton $parent.font.bold -text "Bold"
        ttk::checkbutton $parent.font.italic -text "Italic"

        # Set checkbuttons based on current style
        if {[string match "*bold*" $font_style]} {
            $parent.font.bold state !alternate
            $parent.font.bold state selected
        }
        if {[string match "*italic*" $font_style]} {
            $parent.font.italic state !alternate
            $parent.font.italic state selected
        }

        pack $parent.font.family -side left -padx {0 5}
        pack $parent.font.size -side left -padx {0 5}
        pack $parent.font.bold -side left -padx {0 5}
        pack $parent.font.italic -side left

        # Bind update events
        bind $parent.font.family <<ComboboxSelected>> [list theme_editor::update_font $target $option $parent.font $target_type]
        bind $parent.font.size <Return> [list theme_editor::update_font $target $option $parent.font $target_type]
        bind $parent.font.size <FocusOut> [list theme_editor::update_font $target $option $parent.font $target_type]
        bind $parent.font.size <KeyRelease> [list theme_editor::update_font $target $option $parent.font $target_type]
        $parent.font.bold configure -command [list theme_editor::update_font $target $option $parent.font $target_type]
        $parent.font.italic configure -command [list theme_editor::update_font $target $option $parent.font $target_type]
    }

    # Generic function to apply property changes
    proc apply_property_change {target option value target_type} {
        # Ensure override namespace exists first
        ensure_override_namespace

        if {$target_type eq "widget"} {
            # For widget properties, apply directly but also track in override system
            if {[winfo exists $target]} {
                if {[catch {$target configure $option $value} err]} {
                    tk_messageBox -title "Error" -message "Cannot set $option to $value: $err" -type ok -icon error
                    return
                }
                # Also track widget changes in override system for save functionality
                set widget_class [winfo class $target]
                set override_key "widget:$target:$option"
                ttk::theme::parade::overrides::set_override $override_key $option $value
            }
        } else {
            # Apply to style using the override system
            apply_style_override $target $option $value
        }

        # Force immediate update to make changes visible
        refresh_theme
        update idletasks
    }

    # Update font and apply to target
    proc update_font {target option parent_frame target_type} {
        set family [$parent_frame.family get]
        set size [$parent_frame.size get]

        set style_parts {}
        if {[$parent_frame.bold instate selected]} {
            lappend style_parts "bold"
        }
        if {[$parent_frame.italic instate selected]} {
            lappend style_parts "italic"
        }

        set font_style [join $style_parts " "]
        set font_spec [list $family $size $font_style]

        # Apply change with immediate theme refresh
        apply_property_change $target $option $font_spec $target_type

        # Force an additional theme refresh for font changes
        after idle {theme_editor::refresh_theme}
    }

    # Create color editor with color preview and manual entry
    proc create_color_editor {parent target option current_value {target_type "style"}} {
        ttk::frame $parent.color
        pack $parent.color -side left -fill x -expand 1

        # Color preview
        frame $parent.color.preview -width 30 -height 20 -relief sunken -bd 1
        if {$current_value ne ""} {
            $parent.color.preview configure -bg $current_value
        }

        # Color entry
        ttk::entry $parent.color.entry -width 15
        $parent.color.entry insert 0 $current_value

        # Color picker button (simplified)
        ttk::button $parent.color.pick -text "Pick..." -width 8 \
            -command [list theme_editor::pick_color $target $option $parent.color $target_type]

        pack $parent.color.preview -side left -padx {0 5}
        pack $parent.color.entry -side left -padx {0 5}
        pack $parent.color.pick -side left

        # Bind update events
        bind $parent.color.entry <Return> [list theme_editor::update_color $target $option $parent.color $target_type]
        bind $parent.color.entry <FocusOut> [list theme_editor::update_color $target $option $parent.color $target_type]
    }

    # Update color and apply to target
    proc update_color {target option parent_frame target_type} {
        set color [$parent_frame.entry get]

        # Update preview
        if {[catch {$parent_frame.preview configure -bg $color}]} {
            # Invalid color, reset to white
            $parent_frame.preview configure -bg white
            return
        }

        # Apply change
        apply_property_change $target $option $color $target_type
    }

    # Simple color picker (basic color selection)
    proc pick_color {target option parent_frame target_type} {
        set colors {"#ffffff" "#000000" "#ff0000" "#00ff00" "#0000ff" "#ffff00" "#ff00ff" "#00ffff"
                   "#c0c0c0" "#808080" "#800000" "#008000" "#000080" "#808000" "#800080" "#008080"}

        set picker_win ".color_picker"
        if {[winfo exists $picker_win]} {
            destroy $picker_win
        }

        toplevel $picker_win
        wm title $picker_win "Choose Color"
        wm geometry $picker_win "400x200"

        set row 0
        set col 0
        foreach color $colors {
            button $picker_win.btn_${row}_${col} -bg $color -width 4 -height 2 \
                -command [list theme_editor::select_color $target $option $parent_frame $color $picker_win $target_type]
            grid $picker_win.btn_${row}_${col} -row $row -column $col -padx 1 -pady 1

            incr col
            if {$col >= 8} {
                set col 0
                incr row
            }
        }

        # Manual entry
        ttk::frame $picker_win.manual
        grid $picker_win.manual -row [expr $row + 1] -column 0 -columnspan 8 -pady 10

        ttk::label $picker_win.manual.lbl -text "Or enter hex color:"
        ttk::entry $picker_win.manual.entry -width 10
        ttk::button $picker_win.manual.ok -text "OK" \
            -command [list theme_editor::select_manual_color $target $option $parent_frame $picker_win $target_type]

        pack $picker_win.manual.lbl -side left -padx 5
        pack $picker_win.manual.entry -side left -padx 5
        pack $picker_win.manual.ok -side left -padx 5
    }

    # Select color from picker
    proc select_color {target option parent_frame color picker_win target_type} {
        $parent_frame.entry delete 0 end
        $parent_frame.entry insert 0 $color
        update_color $target $option $parent_frame $target_type
        destroy $picker_win
    }

    # Select manual color from picker
    proc select_manual_color {target option parent_frame picker_win target_type} {
        set color [$picker_win.manual.entry get]
        if {$color ne ""} {
            $parent_frame.entry delete 0 end
            $parent_frame.entry insert 0 $color
            update_color $target $option $parent_frame $target_type
        }
        destroy $picker_win
    }

    # Create integer editor with spinbox
    proc create_integer_editor {parent target option current_value {target_type "style"}} {
        ttk::spinbox $parent.spin -from 0 -to 20 -width 10
        $parent.spin set $current_value
        pack $parent.spin -side left -fill x -expand 1 -padx {0 10}

        bind $parent.spin <Return> [list theme_editor::apply_integer_value $target $option $parent.spin $target_type]
        bind $parent.spin <FocusOut> [list theme_editor::apply_integer_value $target $option $parent.spin $target_type]
    }

    # Apply integer value
    proc apply_integer_value {target option spinbox_widget target_type} {
        set value [$spinbox_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create relief editor with combobox
    proc create_relief_editor {parent target option current_value {target_type "style"}} {
        set reliefs {"flat" "raised" "sunken" "ridge" "groove" "solid"}
        ttk::combobox $parent.combo -values $reliefs -width 10 -state readonly
        $parent.combo set $current_value
        pack $parent.combo -side left -fill x -expand 1 -padx {0 10}

        bind $parent.combo <<ComboboxSelected>> [list theme_editor::apply_relief_value $target $option $parent.combo $target_type]
    }

    # Apply relief value
    proc apply_relief_value {target option combo_widget target_type} {
        set value [$combo_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create padding editor (4 values: left top right bottom)
    proc create_padding_editor {parent target option current_value {target_type "style"}} {
        # Parse current padding
        set padding_values [split $current_value]
        if {[llength $padding_values] == 1} {
            set padding_values [list $current_value $current_value $current_value $current_value]
        } elseif {[llength $padding_values] == 2} {
            set h [lindex $padding_values 0]
            set v [lindex $padding_values 1]
            set padding_values [list $h $v $h $v]
        } elseif {[llength $padding_values] == 4} {
            # Already correct format
        } else {
            set padding_values {0 0 0 0}
        }

        ttk::frame $parent.padding
        pack $parent.padding -side left -fill x -expand 1

        # Create 4 spinboxes for left, top, right, bottom
        set labels {L T R B}
        for {set i 0} {$i < 4} {incr i} {
            set lbl [lindex $labels $i]
            set val [lindex $padding_values $i]

            ttk::label $parent.padding.lbl$i -text "$lbl:" -width 3
            ttk::spinbox $parent.padding.spin$i -from 0 -to 50 -width 5 \
                -command [list theme_editor::update_padding $target $option $parent.padding $target_type]
            $parent.padding.spin$i set $val

            pack $parent.padding.lbl$i -side left
            pack $parent.padding.spin$i -side left -padx {0 10}
        }
    }

    # Update padding values and apply to target
    proc update_padding {target option parent_frame target_type} {
        set values {}
        for {set i 0} {$i < 4} {incr i} {
            lappend values [$parent_frame.spin$i get]
        }
        set padding_str [join $values " "]

        # Apply change - don't add extra braces, let the property system handle it
        apply_property_change $target $option $padding_str $target_type
    }

    # Create simple text editor
    proc create_text_editor {parent target option current_value {target_type "style"}} {
        ttk::entry $parent.entry -width 20
        $parent.entry insert 0 $current_value
        pack $parent.entry -side left -fill x -expand 1 -padx {0 10}

        bind $parent.entry <Return> [list theme_editor::apply_text_value $target $option $parent.entry $target_type]
        bind $parent.entry <FocusOut> [list theme_editor::apply_text_value $target $option $parent.entry $target_type]
    }

    # Apply text value from entry
    proc apply_text_value {target option entry_widget target_type} {
        set value [$entry_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create state editor with combobox
    proc create_state_editor {parent target option current_value {target_type "style"}} {
        set states {"normal" "disabled" "active" "pressed" "selected" "readonly"}
        ttk::combobox $parent.combo -values $states -width 10 -state readonly
        $parent.combo set $current_value
        pack $parent.combo -side left -fill x -expand 1 -padx {0 10}

        bind $parent.combo <<ComboboxSelected>> [list theme_editor::apply_state_value $target $option $parent.combo $target_type]
    }

    # Apply state value
    proc apply_state_value {target option combo_widget target_type} {
        set value [$combo_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create anchor editor with combobox
    proc create_anchor_editor {parent target option current_value {target_type "style"}} {
        set anchors {"n" "ne" "e" "se" "s" "sw" "w" "nw" "center"}
        ttk::combobox $parent.combo -values $anchors -width 10 -state readonly
        $parent.combo set $current_value
        pack $parent.combo -side left -fill x -expand 1 -padx {0 10}

        bind $parent.combo <<ComboboxSelected>> [list theme_editor::apply_anchor_value $target $option $parent.combo $target_type]
    }

    # Apply anchor value
    proc apply_anchor_value {target option combo_widget target_type} {
        set value [$combo_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create justify editor with combobox
    proc create_justify_editor {parent target option current_value {target_type "style"}} {
        set justifications {"left" "center" "right"}
        ttk::combobox $parent.combo -values $justifications -width 10 -state readonly
        $parent.combo set $current_value
        pack $parent.combo -side left -fill x -expand 1 -padx {0 10}

        bind $parent.combo <<ComboboxSelected>> [list theme_editor::apply_justify_value $target $option $parent.combo $target_type]
    }

    # Apply justify value
    proc apply_justify_value {target option combo_widget target_type} {
        set value [$combo_widget get]
        apply_property_change $target $option $value $target_type
    }

    # Create custom property editor for user-defined properties
    proc create_custom_property_editor {style_name row} {
        variable property_frame

        set frame_name "$property_frame.custom"
        ttk::labelframe $frame_name -text "Custom Property"
        pack $frame_name -fill x -pady {10 0}

        ttk::frame $frame_name.content
        pack $frame_name.content -fill x -padx 5 -pady 5

        ttk::label $frame_name.content.opt_lbl -text "Option:"
        ttk::entry $frame_name.content.opt_entry -width 15
        ttk::label $frame_name.content.val_lbl -text "Value:"
        ttk::entry $frame_name.content.val_entry -width 20
        ttk::button $frame_name.content.apply_btn -text "Apply" \
            -command [list theme_editor::apply_custom_property $style_name $frame_name.content]

        grid $frame_name.content.opt_lbl -row 0 -column 0 -sticky w -padx {0 5}
        grid $frame_name.content.opt_entry -row 0 -column 1 -padx {0 10}
        grid $frame_name.content.val_lbl -row 0 -column 2 -sticky w -padx {0 5}
        grid $frame_name.content.val_entry -row 0 -column 3 -padx {0 10}
        grid $frame_name.content.apply_btn -row 0 -column 4

        grid columnconfigure $frame_name.content 3 -weight 1
    }

    # Apply custom property
    proc apply_custom_property {style_name parent_frame} {
        set option [$parent_frame.opt_entry get]
        set value [$parent_frame.val_entry get]

        if {$option ne "" && $value ne ""} {
            if {![string match "-*" $option]} {
                set option "-$option"
            }
            apply_style_override $style_name $option $value

            # Clear entries
            $parent_frame.opt_entry delete 0 end
            $parent_frame.val_entry delete 0 end
        }
    }

    # Revert a single property to default
    proc revert_property {style_name option} {
        ensure_override_namespace
        ttk::theme::parade::overrides::remove_override $style_name $option

        # Refresh the current display by reloading class properties
        set ::widget_path_var $style_name
        inspect_style_class $style_name
    }

    # Save all overrides to file
    proc save_overrides {} {
        ensure_override_namespace

        # Get the override array
        set override_count 0
        array set overrides_data {}
        if {[namespace exists ttk::theme::parade::overrides]} {
            variable ttk::theme::parade::overrides::overrides
            set override_count [array size overrides]
            array set overrides_data [array get overrides]
        }

        if {$override_count > 0} {
            # Determine save location - use XDG config directory
            set config_dir ""
            if {[info exists ::env(HOME)]} {
                set config_dir [file join $::env(HOME) ".config" "strongbox8"]
            } else {
                set config_dir [file join [pwd] "config"]
            }

            # Create config directory if it doesn't exist
            if {![file exists $config_dir]} {
                file mkdir $config_dir
            }

            set override_file [file join $config_dir "theme_overrides.tcl"]

            # Write overrides to file
            if {[catch {
                set fp [open $override_file w]
                puts $fp "# Strongbox Theme Overrides"
                puts $fp "# Generated automatically by theme editor"
                puts $fp "# File created: [clock format [clock seconds]]"
                puts $fp ""
                puts $fp "# Ensure override namespace exists"
                puts $fp "if {!\[namespace exists ttk::theme::parade::overrides\]} {"
                puts $fp "    namespace eval ttk::theme::parade::overrides {"
                puts $fp "        variable overrides"
                puts $fp "        array set overrides {}"
                puts $fp "        proc apply_overrides {} {"
                puts $fp "            variable overrides"
                puts $fp "            foreach {key value} \[array get overrides\] {"
                puts $fp "                set parts \[split \$key \".\"\]"
                puts $fp "                if {\[llength \$parts\] == 2} {"
                puts $fp "                    set style \[lindex \$parts 0\]"
                puts $fp "                    set option \[lindex \$parts 1\]"
                puts $fp "                    ttk::style configure \$style \$option \$value"
                puts $fp "                }"
                puts $fp "            }"
                puts $fp "        }"
                puts $fp "        proc set_override {style option value} {"
                puts $fp "            variable overrides"
                puts $fp "            set key \"\$style.\$option\""
                puts $fp "            set overrides(\$key) \$value"
                puts $fp ""
                puts $fp "            # Try TTK style configuration first"
                puts $fp "            if {\[catch {ttk::style configure \$style \$option \$value} err\] == 0} {"
                puts $fp "                # TTK style configuration succeeded"
                puts $fp "                # Special handling for TNotebook.Tab padding to override mapped states"
                puts $fp "                if {\$style eq \"TNotebook.Tab\" && \$option eq \"-padding\"} {"
                puts $fp "                    catch {ttk::style map \$style \$option {}}"
                puts $fp "                    catch {ttk::style map \$style \$option \[list selected \$value active \$value pressed \$value {} \$value\]}"
                puts $fp "                }"
                puts $fp "            } else {"
                puts $fp "                # TTK failed, use option database for regular widget classes"
                puts $fp "                set pattern \"*\$\{style\}\$\{option\}\""
                puts $fp "                catch {option add \$pattern \$value}"
                puts $fp ""
                puts $fp "                # Also configure existing widgets of this class"
                puts $fp "                set all_widgets \[list \".\"\]"
                puts $fp "                set checked {}"
                puts $fp "                while {\[llength \$all_widgets\] > 0} {"
                puts $fp "                    set widget \[lindex \$all_widgets 0\]"
                puts $fp "                    set all_widgets \[lrange \$all_widgets 1 end\]"
                puts $fp "                    if {\[lsearch \$checked \$widget\] != -1} continue"
                puts $fp "                    lappend checked \$widget"
                puts $fp "                    if {\[winfo exists \$widget\]} {"
                puts $fp "                        if {\[winfo class \$widget\] eq \$style} {"
                puts $fp "                            catch {\$widget configure \$option \$value}"
                puts $fp "                        }"
                puts $fp "                        if {\[catch {set children \[winfo children \$widget\]} err\] == 0} {"
                puts $fp "                            set all_widgets \[concat \$all_widgets \$children\]"
                puts $fp "                        }"
                puts $fp "                    }"
                puts $fp "                }"
                puts $fp "            }"
                puts $fp "        }"
                puts $fp "        proc remove_override {style option} {"
                puts $fp "            variable overrides"
                puts $fp "            set key \"\$style.\$option\""
                puts $fp "            if {\[info exists overrides(\$key)\]} {"
                puts $fp "                unset overrides(\$key)"
                puts $fp "                # Force theme refresh"
                puts $fp "                set current_theme \[ttk::style theme use\]"
                puts $fp "                if {\$current_theme eq \"parade\"} {"
                puts $fp "                    ttk::style theme use clam"
                puts $fp "                    ttk::style theme use parade"
                puts $fp "                    apply_overrides"
                puts $fp "                }"
                puts $fp "            }"
                puts $fp "        }"
                puts $fp "        proc clear_all_overrides {} {"
                puts $fp "            variable overrides"
                puts $fp "            array unset overrides"
                puts $fp "            set current_theme \[ttk::style theme use\]"
                puts $fp "            if {\$current_theme eq \"parade\"} {"
                puts $fp "                ttk::style theme use clam"
                puts $fp "                ttk::style theme use parade"
                puts $fp "            }"
                puts $fp "        }"
                puts $fp "    }"
                puts $fp "}"
                puts $fp ""
                puts $fp "# Apply saved overrides"
                foreach {key value} [array get overrides_data] {
                    # Split the key properly: style.option -> style and option
                    set parts [split $key "."]
                    if {[llength $parts] >= 2} {
                        # Find the last part as option, everything before as style
                        set option [lindex $parts end]
                        set style [join [lrange $parts 0 end-1] "."]

                        # Escape special characters in the value
                        set escaped_value [string map {\" \\\" \\ \\\\ \{ \\\{ \} \\\}} $value]
                        puts $fp "ttk::theme::parade::overrides::set_override \"$style\" \"$option\" \"$escaped_value\""
                    }
                }
                puts $fp ""
                puts $fp "# Apply all overrides"
                puts $fp "ttk::theme::parade::overrides::apply_overrides"
                close $fp

                tk_messageBox -title "Saved" -message "Theme overrides saved successfully to:\n$override_file\n\n($override_count properties saved)" -type ok -icon info
            } err]} {
                tk_messageBox -title "Error" -message "Failed to save theme overrides:\n$err" -type ok -icon error
            }
        } else {
            tk_messageBox -title "Info" -message "No theme overrides to save." -type ok -icon info
        }
    }

    # Revert all overrides
    proc revert_all {} {
        set answer [tk_messageBox -title "Confirm" -message "This will remove all theme overrides. Continue?" \
            -type yesno -icon question]
        if {$answer eq "yes"} {
            ensure_override_namespace
            ttk::theme::parade::overrides::clear_all_overrides

            # Refresh display if a class is currently selected
            if {[info exists ::widget_path_var] && $::widget_path_var ne ""} {
                inspect_style_class $::widget_path_var
            }
        }
    }

    # Use current selection (if any)
    proc use_current_selection {} {
        # This would need integration with the main app to get current selection
        # For now, just show a message
        tk_messageBox -title "Info" -message "This feature would use the currently selected widget in the main application" \
            -type ok -icon info
    }

    # Close the editor
    proc close_editor {} {
        variable editor_window

        if {[winfo exists $editor_window]} {
            destroy $editor_window
        }
        set editor_window ""
    }

    # Main entry point to launch the editor in a separate window
    proc launch {} {
        create_editor
    }

    # Create the theme editor embedded in an existing frame
    proc create_embedded_editor {parent_frame} {
        variable editor_window
        variable property_frame

        # Use the provided frame as our container
        set editor_window $parent_frame

        # Create main frame structure within the parent
        ttk::frame $editor_window.main
        pack $editor_window.main -fill both -expand 1 -padx 5 -pady 5

        # TTK Style Class selector
        ttk::labelframe $editor_window.main.class_frame -text "TTK Style Class"
        pack $editor_window.main.class_frame -fill x -pady {0 10}

        # Create inner frame for combo box and refresh button
        ttk::frame $editor_window.main.class_frame.inner
        pack $editor_window.main.class_frame.inner -fill x -padx 10 -pady 5

        ttk::combobox $editor_window.main.class_frame.inner.combo -state readonly -width 25

        # Discover available TTK style classes dynamically
        set ttk_classes [discover_available_styles]
        $editor_window.main.class_frame.inner.combo configure -values $ttk_classes
        $editor_window.main.class_frame.inner.combo set "TNotebook.Tab"

        # Refresh button to re-scan widget tree
        ttk::button $editor_window.main.class_frame.inner.refresh -text "Refresh" -width 10 \
            -command {theme_editor::refresh_style_classes}

        # Bind selection to load properties
        bind $editor_window.main.class_frame.inner.combo <<ComboboxSelected>> {theme_editor::load_class_properties}

        pack $editor_window.main.class_frame.inner.combo -side left -fill x -expand 1
        pack $editor_window.main.class_frame.inner.refresh -side right -padx {10 0}

        # Properties section (directly under class selector)
        ttk::frame $editor_window.main.prop_frame
        pack $editor_window.main.prop_frame -fill both -expand 1 -pady {0 10}

        # Create scrollable canvas for properties
        canvas $editor_window.main.prop_frame.canvas -highlightthickness 0 \
            -yscrollcommand [list $editor_window.main.prop_frame.scroll set]
        ttk::scrollbar $editor_window.main.prop_frame.scroll -orient vertical \
            -command [list $editor_window.main.prop_frame.canvas yview]

        # Create the properties frame
        set property_frame $editor_window.main.prop_frame.content
        ttk::frame $property_frame

        # Add the frame to the canvas
        $editor_window.main.prop_frame.canvas create window 0 0 -anchor nw -window $property_frame

        pack $editor_window.main.prop_frame.scroll -side right -fill y
        pack $editor_window.main.prop_frame.canvas -side left -fill both -expand 1

        # Configure scrolling
        bind $property_frame <Configure> [list theme_editor::configure_scroll_region $editor_window.main.prop_frame.canvas]
        bind $editor_window.main.prop_frame.canvas <Configure> [list theme_editor::configure_canvas_width $editor_window.main.prop_frame.canvas $property_frame]

        # Enable mouse wheel scrolling
        bind $editor_window.main.prop_frame.canvas <MouseWheel> [list theme_editor::mouse_wheel_scroll $editor_window.main.prop_frame.canvas %D]
        bind $editor_window.main.prop_frame.canvas <Button-4> [list $editor_window.main.prop_frame.canvas yview scroll -3 units]
        bind $editor_window.main.prop_frame.canvas <Button-5> [list $editor_window.main.prop_frame.canvas yview scroll 3 units]

        # Control buttons
        ttk::frame $editor_window.main.controls
        pack $editor_window.main.controls -fill x

        ttk::button $editor_window.main.controls.save_btn -text "Save Overrides" \
            -command {theme_editor::save_overrides}
        ttk::button $editor_window.main.controls.revert_btn -text "Revert All" \
            -command {theme_editor::revert_all}

        pack $editor_window.main.controls.save_btn -side left -padx {0 10}
        pack $editor_window.main.controls.revert_btn -side left

        # Initial message
        ttk::label $property_frame.msg -text "Select a TTK style class above to edit its properties"
        pack $property_frame.msg -pady 20

        # Load saved overrides and initial class properties
        after idle {
            theme_editor::load_saved_overrides
            theme_editor::load_class_properties
        }
    }
}

# Global variables
set widget_path_var ""
set theme_editor::filter_var ""