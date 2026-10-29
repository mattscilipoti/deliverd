package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// buildTime is an RFC 3339 UTC timestamp set at build time by build.sh
// (-ldflags "-X main.buildTime=...").
var buildTime string

// buildLabel is shown on the main screen and in help, in the local time zone.
func buildLabel() string { return buildLabelIn(buildTime, time.Local) }

// buildLabelIn formats raw in loc, adding the UTC time of day when loc isn't UTC.
// A value that isn't RFC 3339 is shown as is.
func buildLabelIn(raw string, loc *time.Location) string {
	if raw == "" {
		return "dev build (not stamped)"
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "built " + raw
	}
	local := t.In(loc)
	if _, offset := local.Zone(); offset == 0 {
		return "built " + t.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return "built " + local.Format("2006-01-02 15:04:05 MST") + " (" + t.UTC().Format("15:04 UTC") + ")"
}

func main() {
	var o options
	flag.Float64Var(&o.override.L, "length", 0, "override cargo length in inches")
	flag.Float64Var(&o.override.W, "width", 0, "override cargo width in inches")
	flag.Float64Var(&o.override.H, "height", 0, "override cargo height in inches")
	flag.BoolVar(&o.merge, "merge", true, "combine item sets with identical dimensions")
	seats := flag.String("seats", "down", "rear seats: up or down")
	dollyFlag := flag.String("dolly", "off", `dolly: "off", "seats" (stowed behind the seats, not counted) or "items" (packed with the items)`)
	version := flag.Bool("version", false, "print the build time and exit")
	dir := flag.String("dir", "", "directory for saved loads (default: user config dir)")
	flag.Parse()

	var err error
	if o.dolly, err = ParseDollyMode(*dollyFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
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
