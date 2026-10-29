package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// buildTime is set at build time by build.sh (-ldflags "-X main.buildTime=...").
var buildTime string

// buildLabel is shown on the main screen and in help.
func buildLabel() string {
	if buildTime == "" {
		return "dev build (not stamped)"
	}
	return "built " + buildTime
}

func main() {
	var o options
	flag.Float64Var(&o.override.L, "length", 0, "override cargo length in inches")
	flag.Float64Var(&o.override.W, "width", 0, "override cargo width in inches")
	flag.Float64Var(&o.override.H, "height", 0, "override cargo height in inches")
	flag.BoolVar(&o.merge, "merge", true, "combine item sets with identical dimensions")
	seats := flag.String("seats", "down", "rear seats: up or down")
	flag.BoolVar(&o.dolly, "dolly", false, "include the 48x24x6 in dolly in the load")
	version := flag.Bool("version", false, "print the build time and exit")
	dir := flag.String("dir", "", "directory for saved loads (default: user config dir)")
	flag.Parse()

	if *version {
		fmt.Println("deliverd", buildLabel())
		return
	}
	if o.override.L < 0 || o.override.W < 0 || o.override.H < 0 {
		fmt.Fprintln(os.Stderr, "cargo dimensions must be positive")
		os.Exit(2)
	}
	switch *seats {
	case "up":
	case "down":
		o.seatsDown = true
	default:
		fmt.Fprintln(os.Stderr, `-seats must be "up" or "down"`)
		os.Exit(2)
	}

	o.storeDir = *dir
	if o.storeDir == "" {
		// An error leaves storeDir empty, which disables saving with a message in the TUI.
		o.storeDir, _ = DefaultStoreDir()
	}

	if _, err := tea.NewProgram(newModel(o)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
