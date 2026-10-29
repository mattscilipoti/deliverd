package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestModel(merge bool) model {
	return newModel(options{seatsDown: true, merge: merge})
}

func press(m model, keys ...string) model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

// typeSet fills L, W, H and Qty (skipping the optional weight) and presses enter.
func typeSet(m model, l, w, h, qty string) model { return typeSetW(m, l, w, h, "", qty) }

// typeSetW fills L, W, H, Wt and Qty; an empty wt or qty is left blank.
func typeSetW(m model, l, w, h, wt, qty string) model {
	m = press(m, l, "tab", w, "tab", h, "tab")
	if wt != "" {
		m = press(m, wt)
	}
	m = press(m, "tab")
	if qty != "" {
		m = press(m, qty)
	}
	return press(m, "enter")
}

func TestEmptyQtyDefaultsToOne(t *testing.T) {
	m := typeSet(newTestModel(true), "10", "20", "30", "")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 1 {
		t.Fatalf("Sets = %v, want one set with qty 1", m.load.Sets)
	}
	if m.err != nil {
		t.Errorf("err = %v", m.err)
	}
}

func TestQtyIsLastField(t *testing.T) {
	m := typeSet(newTestModel(true), "10", "20", "30", "4")
	want := ItemSet{Qty: 4, Dims: Dims{10, 20, 30}}
	if len(m.load.Sets) != 1 || m.load.Sets[0] != want {
		t.Fatalf("Sets = %v, want [%v]", m.load.Sets, want)
	}
}

func TestInvalidQtyRejected(t *testing.T) {
	m := typeSet(newTestModel(true), "10", "20", "30", "x")
	if m.err == nil || len(m.load.Sets) != 0 {
		t.Errorf("err=%v sets=%v, want an error and no sets", m.err, m.load.Sets)
	}
}

func TestMergeOnCombinesQuantities(t *testing.T) {
	m := newTestModel(true)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "10", "10", "10", "3")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 5 {
		t.Fatalf("Sets = %v, want one set with qty 5", m.load.Sets)
	}
}

func TestMergeOffKeepsSeparate(t *testing.T) {
	m := newTestModel(false)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "10", "10", "10", "3")
	if len(m.load.Sets) != 2 {
		t.Fatalf("Sets = %v, want 2", m.load.Sets)
	}
	// Turning merge on combines the existing duplicates.
	m = press(m, "esc", "m")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 5 {
		t.Fatalf("after toggle Sets = %v, want one set with qty 5", m.load.Sets)
	}
}

func TestEditExistingSet(t *testing.T) {
	m := newTestModel(true)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "20", "20", "20", "1")
	// Select the first set and edit it.
	m = press(m, "esc", "k", "e")
	if m.mode != modeItem || m.editing != 0 {
		t.Fatalf("mode=%v editing=%d, want item mode editing 0", m.mode, m.editing)
	}
	if got := m.itemInputs[0].Value(); got != "10" {
		t.Errorf("L prefilled = %q, want 10", got)
	}
	// Change L from 10 to 15 and save.
	m.itemInputs[0].SetValue("15")
	m = press(m, "enter")
	want := ItemSet{Qty: 2, Dims: Dims{15, 10, 10}}
	if m.load.Sets[0] != want || len(m.load.Sets) != 2 {
		t.Fatalf("Sets = %v, want first set %v", m.load.Sets, want)
	}
	if m.mode != modeCommand || m.editing != -1 {
		t.Errorf("mode=%v editing=%d, want command mode, not editing", m.mode, m.editing)
	}
}

func TestEditCancelLeavesSetUnchanged(t *testing.T) {
	m := typeSet(newTestModel(true), "10", "10", "10", "2")
	m = press(m, "esc", "e", "esc")
	if m.load.Sets[0].Qty != 2 || m.editing != -1 || m.mode != modeCommand {
		t.Errorf("Sets=%v editing=%d mode=%v", m.load.Sets, m.editing, m.mode)
	}
}

func TestDeleteSelected(t *testing.T) {
	m := newTestModel(true)
	m = typeSet(m, "1", "1", "1", "")
	m = typeSet(m, "2", "2", "2", "")
	m = typeSet(m, "3", "3", "3", "")
	m = press(m, "esc", "k", "d") // sel starts on the last set; k moves to the middle
	if len(m.load.Sets) != 2 || m.load.Sets[0].L != 1 || m.load.Sets[1].L != 3 {
		t.Errorf("Sets = %v, want sets 1 and 3 remaining", m.load.Sets)
	}
}

func TestPackingLineAppearsAndUpdates(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, override: Dims{L: 20, W: 10, H: 10}})
	m = typeSet(m, "10", "10", "10", "2")
	if !m.pack.AllPlaced() || !strings.Contains(m.View(), "Packing heuristic found a layout for all 2 pieces") {
		t.Fatalf("pack=%+v\n%s", m.pack, m.View())
	}
	// Three cubes: the volume check fails first, so packing is not attempted.
	m = typeSet(m, "10", "10", "10", "1")
	if m.pack.Total != 0 || strings.Contains(m.View(), "Packing heuristic") {
		t.Errorf("packing should be hidden when volume already fails: %+v", m.pack)
	}
}

func TestPackingFailureShownAsWarning(t *testing.T) {
	// Two 6 in cubes pass the volume (432 of 1000) and size checks in a 10 in
	// cube, but cannot both be placed, so the packing line should warn.
	m := newModel(options{seatsDown: true, merge: true, override: Dims{L: 10, W: 10, H: 10}})
	m = typeSet(m, "6", "6", "6", "2")
	if !m.load.Fits() {
		t.Fatal("setup: load should pass the volume and size checks")
	}
	v := m.View()
	for _, want := range []string{"✅ Passes volume and item-size checks", "placed 1 of 2 pieces", "set #1 ×1"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestDollyCyclesThroughLocations(t *testing.T) {
	m := typeSet(newTestModel(true), "10", "10", "10", "1")
	base := m.load.Used()
	m = press(m, "esc", "D") // behind the seats: shown, but takes no cargo space
	if m.load.Dolly != DollyBehindSeats || m.load.Used() != base {
		t.Fatalf("Dolly=%v Used=%v, want behind seats and nothing counted", m.load.Dolly, m.load.Used())
	}
	v := m.View()
	if !strings.Contains(v, "Dolly: behind seats (not counted)") || strings.Contains(v, "  dolly") {
		t.Errorf("behind-seats view:\n%s", v)
	}
	m = press(m, "D") // with the items: counted and packed
	if m.load.Dolly != DollyWithItems || m.load.Used() != base+Dolly.Volume() {
		t.Fatalf("Dolly=%v Used=%v, want the dolly added to used volume", m.load.Dolly, m.load.Used())
	}
	v = m.View()
	for _, want := range []string{"Dolly: with items (48 × 24 × 6 in, packed to fit)", "dolly", "found a layout for all 2 pieces"} {
		if !strings.Contains(v, want) {
			t.Errorf("with-items view missing %q:\n%s", want, v)
		}
	}
	m = press(m, "D")
	if m.load.Dolly != DollyOff || m.load.Used() != base || !strings.Contains(m.View(), "Dolly: off") {
		t.Errorf("Dolly=%v Used=%v, want it off again", m.load.Dolly, m.load.Used())
	}
}

func TestDollyDoesNotAffectSetEditing(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, dolly: DollyWithItems})
	m = typeSet(m, "10", "10", "10", "1")
	m = press(m, "esc", "j", "j", "d") // selection can't move onto the dolly
	if len(m.load.Sets) != 0 || m.load.Dolly != DollyWithItems {
		t.Errorf("Sets=%v Dolly=%v, want the set deleted and the dolly kept", m.load.Sets, m.load.Dolly)
	}
}

func TestDollyTooBigMessage(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, dolly: DollyWithItems, override: Dims{L: 40, W: 20, H: 10}})
	m = typeSet(m, "5", "5", "5", "1")
	if !strings.Contains(m.View(), "The dolly is too big for the cargo space") {
		t.Errorf("view missing the dolly message:\n%s", m.View())
	}
}

func TestHelpOpensAndReturnsToSameMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []string
		want mode
	}{
		{"item form", nil, modeItem},
		{"command mode", []string{"esc"}, modeCommand},
		{"cargo space form", []string{"esc", "s"}, modeSpace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := press(newTestModel(true), tc.keys...)
			m = press(m, "?")
			if m.mode != modeHelp {
				t.Fatalf("mode = %v, want help", m.mode)
			}
			v := m.View()
			for _, want := range []string{"deliverd — help", "Command mode", "Saved loads", "-seats up|down", "Press any key"} {
				if !strings.Contains(v, want) {
					t.Errorf("help missing %q", want)
				}
			}
			m = press(m, "x") // any key closes it, and is not typed into a field
			if m.mode != tc.want {
				t.Errorf("mode = %v, want %v after closing", m.mode, tc.want)
			}
			for _, in := range m.itemInputs {
				if in.Value() != "" {
					t.Errorf("closing help typed %q into a field", in.Value())
				}
			}
		})
	}
}

func TestQuestionMarkIsNotTypedIntoFields(t *testing.T) {
	m := press(newTestModel(true), "5", "?", "x")
	if got := m.itemInputs[0].Value(); got != "5" {
		t.Errorf("L = %q, want 5 (the ? opens help instead of being typed)", got)
	}
}

func TestQuestionMarkTypesInSaveName(t *testing.T) {
	m := typeSet(newSaveModel(t), "10", "10", "10", "1")
	m = press(m, "esc", "w")
	m.nameInput.SetValue("")
	m = press(m, "ok", "?")
	if m.mode != modeSave || m.nameInput.Value() != "ok?" {
		t.Errorf("mode=%v name=%q, want the ? typed into the name", m.mode, m.nameInput.Value())
	}
}

func TestLayoutOpensAndReturns(t *testing.T) {
	m := typeSet(newTestModel(true), "40", "30", "20", "2")
	m = press(m, "esc", "L")
	if m.mode != modeLayout {
		t.Fatalf("mode = %v, want layout", m.mode)
	}
	v := m.View()
	for _, want := range []string{"layout (approximate)", "Top view", "Rear view", "Driver side", "Passenger side", "2013 Toyota Prius v", "placed 2 of 2"} {
		if !strings.Contains(v, want) {
			t.Errorf("layout missing %q:\n%s", want, v)
		}
	}
	m = press(m, "x")
	if m.mode != modeCommand {
		t.Errorf("mode = %v, want command mode after closing", m.mode)
	}
}

func TestLayoutWorksWhenOverCapacity(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, override: Dims{L: 10, W: 10, H: 10}})
	m = typeSet(m, "6", "6", "6", "3")
	m = press(m, "esc", "L")
	if v := m.View(); !strings.Contains(v, "2 not placed") {
		t.Errorf("layout should show what could not be placed:\n%s", v)
	}
	if !strings.Contains(m.View(), "Custom cargo space") {
		t.Error("header should say the cargo space is custom")
	}
}

func TestWindowSizeChangesLayoutScale(t *testing.T) {
	m := typeSet(newTestModel(true), "40", "30", "20", "1")
	m = press(m, "esc")
	resize := func(w, h int) model {
		next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		return next.(model)
	}
	narrow, wide := resize(80, 60), resize(200, 80)
	if narrow.width != 80 || wide.width != 200 || wide.height != 80 {
		t.Fatalf("widths = %d, %d", narrow.width, wide.width)
	}
	narrow, wide = press(narrow, "L"), press(wide, "L")
	if !strings.Contains(narrow.View(), "1 column ≈ 3 in") || !strings.Contains(wide.View(), "1 column ≈ 1 in") {
		t.Errorf("scale should shrink with width:\n%s\n%s", narrow.View(), wide.View())
	}
}

func TestLayoutDefaultsTo80Columns(t *testing.T) {
	m := typeSet(newTestModel(true), "40", "30", "20", "1")
	m = press(m, "esc", "L")
	if m.width != 0 || !strings.Contains(m.View(), "1 column ≈ 3 in") {
		t.Errorf("width=%d; an unknown width should lay out as 80 columns", m.width)
	}
}

func TestBuildLabelOnMainScreenAndHelp(t *testing.T) {
	old := buildTime
	defer func() { buildTime = old }()

	buildTime = ""
	m := newTestModel(true)
	if !strings.Contains(m.View(), "dev build (not stamped)") || !strings.Contains(helpText(), "dev build (not stamped)") {
		t.Error("an unstamped build should say so on the main screen and in help")
	}

	buildTime = "2026-10-29 12:56 UTC"
	if !strings.Contains(m.View(), "built 2026-10-29 12:56 UTC") {
		t.Errorf("main screen missing build time:\n%s", m.View())
	}
	if !strings.Contains(press(m, "?").View(), "built 2026-10-29 12:56 UTC") {
		t.Error("help missing build time")
	}
}

func TestBuildLabelLocalTimeZone(t *testing.T) {
	const raw = "2026-10-29T14:10:30Z"
	edt := time.FixedZone("EDT", -4*3600)
	ist := time.FixedZone("IST", 5*3600+1800)
	for _, tc := range []struct {
		name string
		raw  string
		loc  *time.Location
		want string
	}{
		{"east coast", raw, edt, "built 2026-10-29 10:10:30 EDT (14:10 UTC)"},
		{"half-hour zone crossing midnight", "2026-10-29T20:00:00Z", ist, "built 2026-10-30 01:30:00 IST (20:00 UTC)"},
		{"UTC machine", raw, time.UTC, "built 2026-10-29 14:10:30 UTC"},
		{"not RFC 3339", "last Tuesday", edt, "built last Tuesday"},
		{"unstamped", "", edt, "dev build (not stamped)"},
	} {
		if got := buildLabelIn(tc.raw, tc.loc); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDollyCycleInsideLayout(t *testing.T) {
	m := typeSet(newTestModel(true), "40", "30", "20", "1")
	m = press(m, "esc", "L")
	if m.load.Dolly != DollyOff || !strings.Contains(m.View(), "Dolly: off") || strings.Contains(m.View(), "=  dolly") {
		t.Fatalf("dolly should start off:\n%s", m.View())
	}
	m = press(m, "D")
	if v := m.View(); m.mode != modeLayout || !strings.Contains(v, "Dolly: behind seats (not counted)") || strings.Contains(v, "=  dolly") {
		t.Fatalf("a stowed dolly should be named but not drawn:\n%s", v)
	}
	m = press(m, "D")
	v := m.View()
	if m.mode != modeLayout || !strings.Contains(v, "Dolly: with items") || !strings.Contains(v, "=  dolly") || !strings.Contains(v, "placed 1 of 1") {
		t.Fatalf("mode=%v; the dolly should be drawn and in the legend:\n%s", m.mode, v)
	}
	if m = press(m, "D"); m.load.Dolly != DollyOff || m.mode != modeLayout {
		t.Errorf("third D should turn it off: dolly=%v mode=%v", m.load.Dolly, m.mode)
	}
	if m = press(m, "x"); m.mode != modeCommand {
		t.Errorf("mode = %v, want command mode after another key", m.mode)
	}
}

func TestLayoutShowsDollyAlone(t *testing.T) {
	m := press(newTestModel(true), "esc", "D", "D", "L") // D twice: with items
	if v := m.View(); !strings.Contains(v, "Driver side") || !strings.Contains(v, "=  dolly") {
		t.Errorf("a dolly with no item sets should still draw:\n%s", v)
	}
}

func TestStowedDollyAloneHasNothingToDraw(t *testing.T) {
	m := press(newTestModel(true), "esc", "D", "L")
	if v := m.View(); !strings.Contains(v, "Nothing to show") || !strings.Contains(v, "behind seats") {
		t.Errorf("a stowed dolly takes no cargo space, so there is nothing to draw:\n%s", v)
	}
}

func TestWeightEntry(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "12.5", "4")
	want := ItemSet{Qty: 4, Weight: 12.5, Dims: Dims{10, 10, 10}}
	if len(m.load.Sets) != 1 || m.load.Sets[0] != want {
		t.Fatalf("Sets = %v, want [%v]", m.load.Sets, want)
	}
	if v := m.View(); !strings.Contains(v, "50 lb") {
		t.Errorf("table should show the set's total weight (4 × 12.5):\n%s", v)
	}
	if bad := typeSetW(newTestModel(true), "10", "10", "10", "heavy", "1"); bad.err == nil || len(bad.load.Sets) != 0 {
		t.Errorf("err=%v sets=%v, want an invalid weight rejected", bad.err, bad.load.Sets)
	}
	if neg := typeSetW(newTestModel(true), "10", "10", "10", "-3", "1"); neg.err == nil {
		t.Error("a negative weight should be rejected")
	}
	if blank := typeSet(newTestModel(true), "10", "10", "10", "1"); blank.err != nil || blank.load.Sets[0].Weight != 0 {
		t.Errorf("err=%v sets=%v, want an empty weight accepted as not entered", blank.err, blank.load.Sets)
	}
}

func TestWeightSummaryWithinAndOver(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "50", "3") // 150 lb
	v := m.View()
	for _, want := range []string{
		"Weight limit 915 lb − occupants 175 lb = 740 lb for cargo",
		"from the driver's door plaque",
		"150 lb of 740 lb for cargo", "Within the weight limit (590 lb to spare)",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
	m = typeSetW(m, "20", "20", "20", "100", "9") // +900 lb
	if v := m.View(); !strings.Contains(v, "Over the weight limit by 310 lb") {
		t.Errorf("view should report the overage:\n%s", v)
	}
}

func TestUnweightedSetsWarn(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "50", "1")
	m = typeSet(m, "5", "5", "5", "2")
	if v := m.View(); !strings.Contains(v, "1 of 2 sets have no weight, so the total is a minimum") {
		t.Errorf("view missing the unweighted warning:\n%s", v)
	}
}

func TestEditWeightLimitAndOccupants(t *testing.T) {
	m := press(newTestModel(true), "esc", "p")
	if m.mode != modeWeights || m.occInputs[0].Value() != "915" || m.occInputs[1].Value() != "175" {
		t.Fatalf("mode=%v limit=%q occupants=%q, want the current values prefilled",
			m.mode, m.occInputs[0].Value(), m.occInputs[1].Value())
	}
	m.occInputs[0].SetValue("1000")
	m.occInputs[1].SetValue("350")
	m = press(m, "enter")
	if m.mode != modeCommand || m.load.PayloadLimit != 1000 || m.load.Occupants != 350 || !m.customLimit {
		t.Fatalf("mode=%v limit=%v occupants=%v custom=%v", m.mode, m.load.PayloadLimit, m.load.Occupants, m.customLimit)
	}
	if v := m.View(); !strings.Contains(v, "= 650 lb for cargo") || !strings.Contains(v, "Limit is custom") {
		t.Errorf("header should reflect the custom limit:\n%s", v)
	}
	// Typing the vehicle's own limit back goes to the vehicle's note again.
	m = press(m, "p")
	m.occInputs[0].SetValue("915")
	if m = press(m, "enter"); m.customLimit {
		t.Error("restoring the vehicle's limit should clear the custom flag")
	}
}

func TestWeightFormRejectsBadInputAndCancels(t *testing.T) {
	m := press(newTestModel(true), "esc", "p")
	m.occInputs[1].SetValue("lots")
	m = press(m, "enter")
	if m.mode != modeWeights || m.err == nil || m.load.Occupants != DefaultOccupantsLb {
		t.Errorf("mode=%v err=%v occupants=%v, want the form to stay open and nothing applied", m.mode, m.err, m.load.Occupants)
	}
	m = press(m, "esc")
	if m.mode != modeCommand || m.load.Occupants != DefaultOccupantsLb {
		t.Errorf("mode=%v occupants=%v after cancel", m.mode, m.load.Occupants)
	}
}

func TestOccupantsAloneOverLimit(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, occupants: 1000, occupantsSet: true})
	if v := m.View(); !strings.Contains(v, "Occupants alone exceed the weight limit by 85 lb") {
		t.Errorf("view missing the occupants warning:\n%s", v)
	}
}

func TestPayloadAndOccupantsOptions(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, payload: 800, occupants: 0, occupantsSet: true})
	if m.load.PayloadLimit != 800 || m.load.Occupants != 0 || !m.customLimit {
		t.Errorf("limit=%v occupants=%v custom=%v; flags should override, and 0 is a valid occupant weight",
			m.load.PayloadLimit, m.load.Occupants, m.customLimit)
	}
	// Switching seats keeps a custom limit; switching vehicles resets it.
	if m = press(m, "esc", "r"); m.load.PayloadLimit != 800 {
		t.Errorf("limit = %v after toggling seats, want it kept", m.load.PayloadLimit)
	}
	if m = press(m, "v"); m.load.PayloadLimit != Vehicles[0].PayloadLb || m.customLimit {
		t.Errorf("limit = %v custom=%v after choosing a vehicle, want the vehicle's limit", m.load.PayloadLimit, m.customLimit)
	}
}

func TestEditKeepsWeight(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "7.5", "2")
	m = press(m, "esc", "e")
	if got := m.itemInputs[3].Value(); got != "7.5" {
		t.Fatalf("Wt prefilled = %q, want 7.5", got)
	}
	m.itemInputs[3].SetValue("9")
	m = press(m, "enter")
	if m.load.Sets[0].Weight != 9 || m.load.Sets[0].Qty != 2 {
		t.Errorf("Sets = %v, want weight 9 and qty 2", m.load.Sets)
	}
}

func TestSameSizeDifferentWeightStaySeparate(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "5", "1")
	m = typeSetW(m, "10", "10", "10", "8", "1")
	m = typeSetW(m, "10", "10", "10", "8", "2")
	if len(m.load.Sets) != 2 || m.load.Sets[1].Qty != 3 {
		t.Errorf("Sets = %v, want 2 sets with the second at qty 3", m.load.Sets)
	}
}

func TestWeightsSurviveSaveAndOpen(t *testing.T) {
	m := saveAsName(typeSetW(newSaveModel(t), "10", "10", "10", "25", "2"), "heavy")
	m = press(m, "c")
	m = press(m, "o", "enter", "y")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Weight != 25 || m.load.Weight() != 50 {
		t.Errorf("Sets = %v, want the weight kept through save and open", m.load.Sets)
	}
}

func TestLayoutLegendShowsWeight(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "25", "2")
	m = press(m, "esc", "L")
	if v := m.View(); !strings.Contains(v, "50 lb") {
		t.Errorf("legend should show the set's total weight:\n%s", v)
	}
}

func TestLayoutShowsBalanceAndWeightNotes(t *testing.T) {
	m := typeSetW(newTestModel(true), "10", "10", "10", "50", "2")
	m = press(m, "esc", "L")
	v := m.View()
	for _, want := range []string{"Weight balance:", "front half", "centre of mass", "Heavier pieces go in first"} {
		if !strings.Contains(flat(v), want) {
			t.Errorf("layout missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "treated as weightless") {
		t.Error("the weightless note should only appear when weights are mixed")
	}
	m = press(m, "x", "i")
	m = typeSet(m, "5", "5", "5", "1") // a set with no weight
	m = press(m, "esc", "L")
	if !strings.Contains(flat(m.View()), "treated as weightless") {
		t.Errorf("mixed weights should explain how unweighted pieces are handled:\n%s", m.View())
	}
}

func TestLayoutHasNoBalanceWithoutWeights(t *testing.T) {
	m := press(typeSet(newTestModel(true), "10", "10", "10", "2"), "esc", "L")
	if strings.Contains(m.View(), "Weight balance") {
		t.Errorf("no weights entered, so there is no balance to report:\n%s", m.View())
	}
}

// flat collapses runs of whitespace, so text that wraps can still be matched.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }
