package main

import (
	"fmt"
	"strings"

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
		"L                 show the packing layout (D there cycles the dolly)",
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

// updateLayout lets D cycle the dolly and redraws; any other key returns.
func (m model) updateLayout(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "D" {
		m.load.Dolly = m.load.Dolly.Next()
		m.layoutPack = m.load.Pack()
		return m, nil
	}
	return m.updateHelp(key)
}
