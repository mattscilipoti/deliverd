package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeItem    mode = iota // typing a new item set
	modeCommand             // item inputs blurred; single-key commands active
	modeSpace               // editing cargo dimensions
	modeSave                // typing a name to save the items under
	modeOpen                // choosing a saved load to open
	modeConfirm             // waiting for y/n
	modeHelp                // showing the ? screen
	modeLayout              // showing the packing layout views
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	badStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

type model struct {
	load       Load
	mode       mode
	itemInputs []textinput.Model // L, W, H, Qty
	spcInputs  []textinput.Model // L, W, H
	focus      int
	err        error
	merge      bool // combine sets with identical dimensions
	vehicle    int  // index into Vehicles
	seatsDown  bool
	custom     bool // cargo space was overridden by hand or by flags
	pack       Packing
	packKey    string // load the cached pack was computed for
	sel        int    // selected set in command mode
	editing    int    // index of the set being edited, or -1

	store         Store
	loadName      string // saved load currently open, or ""
	savedKey      string // setsKey of the items as last saved or opened
	nameInput     textinput.Model
	openNames     []string
	openSel       int
	status        string
	confirmPrompt string
	confirmAction func(*model)
	helpReturn    mode // mode to go back to when help or the layout closes

	width      int     // terminal width from the last WindowSizeMsg; 0 until known
	layoutPack Packing // packing computed when the layout was opened
}

func newInputs(labels []string) []textinput.Model {
	in := make([]textinput.Model, len(labels))
	for i, l := range labels {
		t := textinput.New()
		t.Prompt = l + " "
		t.CharLimit = 8
		t.Width = 8
		in[i] = t
	}
	return in
}

// options are the startup settings. Zero override fields keep the vehicle preset.
type options struct {
	vehicle   int
	seatsDown bool
	merge     bool
	override  Dims
	dolly     bool
	storeDir  string // where saved loads live; empty disables saving
}

func newModel(o options) model {
	m := model{
		vehicle:    o.vehicle,
		seatsDown:  o.seatsDown,
		itemInputs: newInputs([]string{"L", "W", "H", "Qty"}),
		spcInputs:  newInputs([]string{"L", "W", "H"}),
		merge:      o.merge,
		editing:    -1,
		store:      Store{Dir: o.storeDir},
		savedKey:   setsKey(nil),
	}
	m.nameInput = textinput.New()
	m.nameInput.Prompt = "Name: "
	m.nameInput.CharLimit = 60
	m.nameInput.Width = 30
	m.load.Dolly = o.dolly
	m.usePreset()
	for _, f := range []struct {
		v   float64
		dst *float64
	}{{o.override.L, &m.load.Space.L}, {o.override.W, &m.load.Space.W}, {o.override.H, &m.load.Space.H}} {
		if f.v > 0 {
			*f.dst = f.v
			m.custom = true
		}
	}
	m.itemInputs[3].Placeholder = "1"
	m.setFocus(m.itemInputs, 0)
	return m
}

// usePreset loads the selected vehicle's cargo box for the current seat configuration.
func (m *model) usePreset() {
	m.load.Space = Vehicles[m.vehicle].Cargo(m.seatsDown).Dims
	m.custom = false
}

func (m model) Init() tea.Cmd { return textinput.Blink }

// active returns the input group for the current mode, or nil in command mode.
func (m *model) active() []textinput.Model {
	switch m.mode {
	case modeItem:
		return m.itemInputs
	case modeSpace:
		return m.spcInputs
	}
	return nil
}

func (m *model) setFocus(group []textinput.Model, i int) {
	m.focus = (i + len(group)) % len(group)
	for k := range group {
		if k == m.focus {
			group[k].Focus()
		} else {
			group[k].Blur()
		}
	}
}

func blurAll(group []textinput.Model) {
	for k := range group {
		group[k].Blur()
	}
}

func parseDim(s, name string) (float64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive number", name)
	}
	return v, nil
}

func parseDims(group []textinput.Model, names []string) (Dims, error) {
	var v [3]float64
	for i := range v {
		var err error
		if v[i], err = parseDim(group[i].Value(), names[i]); err != nil {
			return Dims{}, err
		}
	}
	return Dims{v[0], v[1], v[2]}, nil
}

// readItemSet parses the item inputs. An empty Qty means 1.
func (m *model) readItemSet() (ItemSet, error) {
	d, err := parseDims(m.itemInputs[:3], []string{"L", "W", "H"})
	if err != nil {
		return ItemSet{}, err
	}
	qty := 1
	if q := strings.TrimSpace(m.itemInputs[3].Value()); q != "" {
		if qty, err = strconv.Atoi(q); err != nil || qty <= 0 {
			return ItemSet{}, errors.New("Qty must be a positive whole number")
		}
	}
	return ItemSet{Qty: qty, Dims: d}, nil
}

func (m *model) clearItemInputs() {
	for k := range m.itemInputs {
		m.itemInputs[k].SetValue("")
	}
	m.setFocus(m.itemInputs, 0)
}

// submitItem adds a new set, or replaces the one being edited.
func (m *model) submitItem() error {
	set, err := m.readItemSet()
	if err != nil {
		return err
	}
	if m.editing >= 0 {
		m.sel = m.load.Replace(m.editing, set, m.merge)
		m.editing = -1
		m.clearItemInputs()
		m.enterCommandMode()
		return nil
	}
	m.sel = m.load.Add(set, m.merge)
	m.clearItemInputs()
	return nil
}

func (m *model) enterCommandMode() {
	m.mode = modeCommand
	m.err = nil
	blurAll(m.itemInputs)
	blurAll(m.spcInputs)
	m.nameInput.Blur()
	m.clampSel()
}

func (m *model) clampSel() {
	if m.sel >= len(m.load.Sets) {
		m.sel = len(m.load.Sets) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

func (m *model) editSelected() {
	if m.sel >= len(m.load.Sets) {
		return
	}
	set := m.load.Sets[m.sel]
	for i, v := range []float64{set.L, set.W, set.H} {
		m.itemInputs[i].SetValue(fmtNum(v))
	}
	m.itemInputs[3].SetValue(strconv.Itoa(set.Qty))
	m.editing = m.sel
	m.mode = modeItem
	m.err = nil
	m.setFocus(m.itemInputs, 0)
}

func (m *model) applySpace() error {
	d, err := parseDims(m.spcInputs, []string{"L", "W", "H"})
	if err != nil {
		return err
	}
	m.load.Space = d
	m.custom = true
	return nil
}

func (m *model) enterItemMode() {
	m.mode = modeItem
	m.err = nil
	blurAll(m.spcInputs)
	m.setFocus(m.itemInputs, m.focus)
}

func (m *model) enterSpaceMode() {
	m.mode = modeSpace
	m.err = nil
	blurAll(m.itemInputs)
	s := m.load.Space
	for i, v := range []float64{s.L, s.W, s.H} {
		m.spcInputs[i].SetValue(strconv.FormatFloat(v, 'f', -1, 64))
	}
	m.setFocus(m.spcInputs, 0)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm := next.(model)
	nm.refreshPack()
	return nm, cmd
}

// refreshPack reruns the packing heuristic only when the load or space changed.
// It is skipped when the cheap volume and size checks already fail.
func (m *model) refreshPack() {
	key := fmt.Sprint(m.load.Space, m.load.Sets, m.load.Dolly)
	if key == m.packKey {
		return
	}
	m.packKey = key
	m.pack = Packing{}
	if len(m.load.Sets) > 0 && m.load.Fits() {
		m.pack = m.load.Pack()
	}
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}
	key, isKey := msg.(tea.KeyMsg)
	if isKey {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}
		m.status = ""
		m.err = nil
		switch m.mode {
		case modeCommand:
			return m.updateCommand(key)
		case modeSave:
			return m.updateSave(key)
		case modeOpen:
			return m.updateOpen(key)
		case modeConfirm:
			return m.updateConfirm(key)
		case modeHelp, modeLayout:
			return m.updateHelp(key)
		default:
			if next, cmd, handled := m.updateForm(key); handled {
				return next, cmd
			}
		}
	}

	// Pass everything else (typing, blink) to the focused input.
	if m.mode == modeSave {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}
	group := m.active()
	if group == nil {
		return m, nil
	}
	var cmd tea.Cmd
	group[m.focus], cmd = group[m.focus].Update(msg)
	return m, cmd
}

func (m model) updateCommand(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.sel--
		m.clampSel()
	case "down", "j":
		m.sel++
		m.clampSel()
	case "e":
		m.editSelected()
	case "d":
		if m.sel < len(m.load.Sets) {
			m.load.Sets = append(m.load.Sets[:m.sel], m.load.Sets[m.sel+1:]...)
			m.clampSel()
		}
	case "c":
		m.load.Sets = nil
		m.sel = 0
	case "?":
		m.showHelp()
	case "L":
		m.showLayout()
	case "D":
		m.load.Dolly = !m.load.Dolly
	case "w":
		m.saveCurrent()
	case "W":
		m.beginSaveAs()
	case "o":
		m.beginOpen()
	case "s":
		m.enterSpaceMode()
	case "v":
		m.vehicle = (m.vehicle + 1) % len(Vehicles)
		m.usePreset()
	case "r":
		m.seatsDown = !m.seatsDown
		m.usePreset()
	case "m":
		m.merge = !m.merge
		if m.merge {
			m.load.MergeDuplicates()
			m.clampSel()
		}
	case "i", "enter":
		m.enterItemMode()
	}
	return m, textinput.Blink
}

// updateForm handles keys shared by the item and cargo-space forms.
func (m model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	group := m.active()
	switch key.String() {
	case "tab", "down":
		m.setFocus(group, m.focus+1)
	case "shift+tab", "up":
		m.setFocus(group, m.focus-1)
	case "?":
		m.showHelp()
	case "esc":
		if m.mode == modeSpace {
			m.enterCommandMode()
			break
		}
		if m.editing >= 0 {
			m.editing = -1
			m.clearItemInputs()
		}
		m.enterCommandMode()
	case "enter":
		if m.mode == modeItem {
			m.err = m.submitItem()
			break
		}
		if m.err = m.applySpace(); m.err == nil {
			m.enterCommandMode()
		}
	default:
		return m, nil, false
	}
	return m, textinput.Blink, true
}

func cubicFeet(inches float64) float64 { return inches / cubicInchesPerCubicFoot }

func fmtVol(v float64) string {
	return fmt.Sprintf("%s in³ (%.1f ft³)", commas(int64(v+signHalf(v))), cubicFeet(v))
}

func signHalf(v float64) float64 {
	if v < 0 {
		return -0.5
	}
	return 0.5
}

func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

func fmtNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func bar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (m model) View() string {
	switch m.mode {
	case modeHelp:
		return helpText()
	case modeLayout:
		return renderLayout(m.load, m.layoutPack, m.layoutHeader(), m.layoutWidth())
	}
	var b strings.Builder
	sp := m.load.Space

	b.WriteString(titleStyle.Render("deliverd — will it fit?") + "  " + dimStyle.Render(buildLabel()) + "\n")
	veh := Vehicles[m.vehicle]
	fmt.Fprintf(&b, "Vehicle: %s (%d of %d) — %s\n", veh.Name, m.vehicle+1, len(Vehicles), seatsLabel(m.seatsDown))
	label := "Cargo space"
	if m.custom {
		label = "Cargo space (custom override)"
	} else if veh.Cargo(m.seatsDown).Estimated {
		label = "Cargo space (estimated)"
	}
	fmt.Fprintf(&b, "%s: %s × %s × %s in  =  %s\n",
		label, fmtNum(sp.L), fmtNum(sp.W), fmtNum(sp.H), fmtVol(m.load.Capacity()))
	if !m.custom {
		b.WriteString(dimStyle.Render(veh.describe(m.seatsDown)) + "\n")
	}
	b.WriteString("\n")

	switch m.mode {
	case modeSpace:
		b.WriteString("Edit cargo space (inches):\n")
		b.WriteString(joinInputs(m.spcInputs) + "\n")
	case modeSave:
		b.WriteString(m.saveView())
	case modeOpen:
		b.WriteString(m.openView())
	case modeConfirm:
		b.WriteString(badStyle.Render(m.confirmPrompt) + "\n")
	default:
		if m.editing >= 0 {
			fmt.Fprintf(&b, "Editing set #%d:\n", m.editing+1)
		} else {
			b.WriteString("Add item set (inches):\n")
		}
		b.WriteString(joinInputs(m.itemInputs) + "\n")
	}
	if m.err != nil {
		b.WriteString(badStyle.Render(m.err.Error()) + "\n")
	}
	if m.status != "" {
		b.WriteString(okStyle.Render(m.status) + "\n")
	}
	fmt.Fprintf(&b, "%s\n%s\n\n", dimStyle.Render(m.loadLine()), dimStyle.Render(fmt.Sprintf("Merge identical sets: %s • Dolly (%s × %s × %s in): %s",
		onOff(m.merge), fmtNum(Dolly.L), fmtNum(Dolly.W), fmtNum(Dolly.H), onOff(m.load.Dolly))))

	b.WriteString(m.setsTable())
	b.WriteString("\n")
	b.WriteString(m.summary())
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(m.help()) + "\n")
	return b.String()
}

func capitalize(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func joinInputs(group []textinput.Model) string {
	parts := make([]string, len(group))
	for i, t := range group {
		parts[i] = t.View()
	}
	return strings.Join(parts, "   ")
}

func (m model) setsTable() string {
	if len(m.load.Sets) == 0 && !m.load.Dolly {
		return dimStyle.Render("No item sets yet.") + "\n"
	}
	over := map[int]bool{}
	for _, i := range m.load.Oversized() {
		over[i] = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  %-3s %5s  %-22s %s\n", "#", "Qty", "L × W × H (in)", "Volume")
	for i, s := range m.load.Sets {
		dims := fmt.Sprintf("%s × %s × %s", fmtNum(s.L), fmtNum(s.W), fmtNum(s.H))
		marker := "  "
		if m.mode == modeCommand && i == m.sel {
			marker = "▸ "
		}
		line := fmt.Sprintf("%s%-3d %5d  %-22s %s", marker, i+1, s.Qty, dims, fmtVol(s.TotalVolume()))
		if over[i] {
			line = badStyle.Render(line + "  ⚠ too big")
		}
		b.WriteString(line + "\n")
	}
	if m.load.Dolly {
		d := Dolly
		line := fmt.Sprintf("  %-3s %5d  %-22s %s", "–", d.Qty,
			fmt.Sprintf("%s × %s × %s", fmtNum(d.L), fmtNum(d.W), fmtNum(d.H)), fmtVol(d.TotalVolume())+"  dolly")
		if over[len(m.load.Sets)] {
			line = badStyle.Render(line + "  ⚠ too big")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// entryName names an index into Load.All for messages.
func (m model) entryName(i int) string {
	if i >= len(m.load.Sets) {
		return "the dolly"
	}
	return fmt.Sprintf("set #%d", i+1)
}

func (m model) summary() string {
	l := m.load
	used, avail := l.Used(), l.Available()
	frac := 0.0
	if l.Capacity() > 0 {
		frac = used / l.Capacity()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Used:      %s\n", fmtVol(used))
	fmt.Fprintf(&b, "Available: %s\n", fmtVol(avail))
	fmt.Fprintf(&b, "[%s] %.0f%%\n", bar(frac, 30), frac*100)

	switch {
	case len(l.Sets) == 0:
	case l.Fits():
		b.WriteString(okStyle.Render("✅ Passes volume and item-size checks") + "\n")
		b.WriteString(m.packingLine())
	default:
		if avail < 0 {
			b.WriteString(badStyle.Render(fmt.Sprintf("❌ Over capacity by %s", fmtVol(-avail))) + "\n")
		}
		for _, i := range l.Oversized() {
			b.WriteString(badStyle.Render(fmt.Sprintf("❌ %s is too big for the cargo space", capitalize(m.entryName(i)))) + "\n")
		}
	}
	return b.String()
}

// packingLine reports the 3D packing heuristic. It is a search, so failing to
// place something means "not found", and succeeding is not a guarantee either
// (it ignores weight and crushing).
func (m model) packingLine() string {
	p := m.pack
	switch {
	case p.Total == 0:
		return ""
	case p.Skipped:
		return dimStyle.Render(fmt.Sprintf("Packing: skipped, more than %d pieces", maxPackPieces)) + "\n"
	case p.AllPlaced():
		return okStyle.Render(fmt.Sprintf("✅ Packing heuristic found a layout for all %d pieces", p.Total)) + "\n"
	}
	var parts []string
	for i, n := range p.Unplaced {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s ×%d", m.entryName(i), n))
		}
	}
	return badStyle.Render(fmt.Sprintf("⚠ Packing heuristic placed %d of %d pieces; couldn't place %s",
		len(p.Placed), p.Total, strings.Join(parts, ", "))) + "\n" +
		dimStyle.Render("  It can miss arrangements, so this may still fit if you load it by hand.") + "\n"
}

func (m model) help() string {
	switch m.mode {
	case modeCommand:
		return "? help • ↑/↓ select • e edit set • d delete set • c clear • m merge on/off • r seats up/down • v vehicle • s custom cargo space • L layout • D dolly on/off • w save • W save as • o open • i/enter add • q quit"
	case modeSpace:
		return "? help • tab next field • enter apply • esc cancel • ctrl+c quit"
	case modeSave:
		return "enter save • esc cancel • ctrl+c quit"
	case modeOpen:
		return "↑/↓ select • enter open • esc cancel • ctrl+c quit"
	case modeConfirm:
		return "y confirm • any other key cancels"
	}
	if m.editing >= 0 {
		return "? help • tab next field • enter save • esc cancel • ctrl+c quit"
	}
	return "? help • tab next field • enter add set (empty Qty = 1) • esc commands • ctrl+c quit"
}
