#!/usr/bin/env tclsh
# Test runner for all Tcl/Tk tests

package require tcltest
namespace import tcltest::*

# Configure test output
configure -verbose {body error skip start}

# Set the test directory
set testDir [file dirname [info script]]

# Run all test files
foreach testFile [glob -directory $testDir *_test.tcl] {
    puts "\n=== Running [file tail $testFile] ==="
    source $testFile
}

# Exit with appropriate code
if {$::tcltest::numTests(Failed) > 0} {
    exit 1
}
exit 0
