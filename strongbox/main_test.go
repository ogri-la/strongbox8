package main

import "testing"

func Test_main(t *testing.T) {
	gui := main_gui()
	gui.Stop()
}
