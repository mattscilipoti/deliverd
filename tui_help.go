package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// helpSections drives the ? screen; keep it in step with the key handlers.
var helpSections = []struct {
	title string
	lines []string
}{
	{"Usage", []string{
		"Enter item sets (L, W, H in inches, Qty) to see if a delivery fits the car.",
		"Qty is optional and defaults to 1. Identical sets merge into one row.",
		"Checks: total volume, each item's size in any rotation, and a 3D packing",
		"heuristic: it puts heavier pieces low and forward and never on lighter ones,",
		"but it can miss layouts, so treat it as a guide.",
	}},
	{"Adding items", []string{
		"tab / shift+tab   next / previous field",
		"enter             add the set (Wt = lb per item, optional; empty Qty = 1)",
		"esc               switch to command mode",
	}},
	{"Command mode (after esc)", []string{
		"↑/↓ or k/j        select a set",
		"e                 edit the selected set (enter saves, esc cancels)",
		"d                 delete the selected set",
		"c                 clear all sets",
		"i or enter        go back to adding items",
		"m                 merge identical sets on/off",
		"r                 rear seats up/down",
		"v                 next vehicle",
		"s                 type custom cargo dimensions",
		"p                 set the weight limit and occupant weight (lb)",
		"L                 show the packing layout (D there cycles the dolly,",
		"                  i saves it as an SVG image and opens it in your viewer)",
		"D                 dolly: off → behind seats (not counted) → with items (48 × 24 × 6 in)",
		"q                 quit",
	}},
	{"Saved loads (command mode)", []string{
		"w                 save; overwrites the opened load, or asks for a name",
		"W                 save as a new name (asks before replacing another load)",
		"o                 open a saved load (asks before discarding changes)",
	}},
	{"Help and quitting", []string{
		"?                 this help (in command mode and the item / cargo forms)",
		"ctrl+c            quit from anywhere",
	}},
	{"Flags", []string{
		"-seats up|down    rear seats (default down)",
		"-length -width -height   override the cargo size, in inches",
		"-dolly off|seats|items   where the dolly goes (default off)",
		"-payload LB       override the combined occupant + cargo weight limit",
		"-occupants LB     occupant weight (default 175)",
		"-merge=false      keep identical sets separate",
		"-dir PATH         where saved loads live (default: your config dir)",
		"-image-dir PATH   where layout images are saved (default: temp dir)",
		"-version          print the build time and exit",
	}},
}

func helpText() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("deliverd — help") + "  " + dimStyle.Render(buildLabel()) + "\n")
	for _, s := range helpSections {
		b.WriteString("\n" + titleStyle.Render(s.title) + "\n")
		for _, l := range s.lines {
			b.WriteString("  " + l + "\n")
		}
	}
	b.WriteString("\n" + dimStyle.Render("Press any key to return.") + "\n")
	return b.String()
}

func (m *model) showHelp() {
	m.helpReturn = m.mode
	m.mode = modeHelp
}

func (m model) updateHelp(tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = m.helpReturn
	return m, nil
}

// showLayout packs the current load and opens the layout screen. Packing runs
// here, not on every keystroke, because it can be slow for overloaded loads.
func (m *model) showLayout() {
	m.layoutPack = m.load.Pack()
	m.helpReturn = m.mode
	m.mode = modeLayout
}

func (m model) layoutWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m model) layoutHeader() string {
	c := m.load.Space
	name := "Custom cargo space"
	if !m.custom {
		name = Vehicles[m.vehicle].Name + ", " + seatsLabel(m.seatsDown)
	}
	return fmt.Sprintf("%s (%s × %s × %s in)", name, fmtNum(c.L), fmtNum(c.W), fmtNum(c.H))
}

// updateLayout lets D cycle the dolly and i export an image; any other key returns.
func (m model) updateLayout(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "D":
		m.load.Dolly = m.load.Dolly.Next()
		m.layoutPack = m.load.Pack()
		return m, nil
	case "i":
		return m.exportImage()
	}
	return m.updateHelp(key)
}

// imageOpenedMsg reports the result of launching the image viewer.
type imageOpenedMsg struct {
	path string
	err  error
}

func (o imageOpenedMsg) status() string {
	if o.err != nil {
		return fmt.Sprintf("Saved %s, but couldn't open an image viewer (%v). Open the file yourself.", o.path, o.err)
	}
	return "Saved and opened " + o.path
}

// exportImage writes the current layout as an SVG and opens it in the OS viewer.
// The viewer is launched in a command so the screen doesn't wait on it.
func (m model) exportImage() (tea.Model, tea.Cmd) {
	if len(m.layoutPack.Placed) == 0 {
		m.status = "Nothing to draw yet: no pieces were placed."
		return m, nil
	}
	dir := m.imageDir
	if dir == "" {
		dir = os.TempDir()
	}
	now := time.Now()
	path := filepath.Join(dir, "deliverd-layout-"+now.Format("20060102-150405")+".svg")
	stamp := "Exported " + now.Format("2006-01-02 15:04:05 MST") + " · deliverd " + buildLabel()
	svg := renderSVG(m.load, m.layoutPack, m.layoutHeader(), stamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.status = fmt.Sprintf("Couldn't save the image: %v", err)
		return m, nil
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		m.status = fmt.Sprintf("Couldn't save the image: %v", err)
		return m, nil
	}
	m.status = "Saved " + path + " — opening it…"
	open := m.openImage
	if open == nil {
		open = osOpen
	}
	return m, func() tea.Msg { return imageOpenedMsg{path, open(path)} }
}

// osOpen opens path with the desktop's default viewer. A viewer that fails
// right away (for example xdg-open with no handler) is reported; one that is
// still running after a moment is assumed to have opened.
func osOpen(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}
