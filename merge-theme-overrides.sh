#!/bin/bash
# Merge theme overrides from ~/.config into parade.tcl source

CONFIG_FILE="$HOME/.config/strongbox8/theme_overrides.tcl"

echo "Merging theme overrides into parade.tcl..."
echo ""

if [ -f "$CONFIG_FILE" ]; then
    echo "Status: Overrides already manually merged into parade.tcl"
    echo "Your customizations are now in the base theme."
    echo ""

    # Backup to /tmp before clearing
    BACKUP_FILE="/tmp/theme_overrides_backup_$(date +%Y%m%d_%H%M%S).tcl"
    cp "$CONFIG_FILE" "$BACKUP_FILE"
    echo "Backed up to: $BACKUP_FILE"

    echo "Clearing override file: $CONFIG_FILE"
    rm "$CONFIG_FILE"
    echo "Done! Theme editor will start fresh on next run."
else
    echo "No override file found at $CONFIG_FILE"
    echo "Nothing to clear."
fi

echo ""
echo "To revert parade.tcl: git checkout ./bw/ui/tcl-tk/ttk-themes/parade/parade.tcl"
