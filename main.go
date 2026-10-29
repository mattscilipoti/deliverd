package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	space := DefaultSpace
	flag.Float64Var(&space.L, "length", space.L, "cargo length in inches")
	flag.Float64Var(&space.W, "width", space.W, "cargo width in inches")
	flag.Float64Var(&space.H, "height", space.H, "cargo height in inches")
	flag.Parse()

	if space.L <= 0 || space.W <= 0 || space.H <= 0 {
		fmt.Fprintln(os.Stderr, "cargo dimensions must be positive")
		os.Exit(2)
	}

	if _, err := tea.NewProgram(newModel(space)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
