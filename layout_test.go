package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func column(v view, c int) []int {
	out := make([]int, v.rows)
	for r := range out {
		out[r] = v.owner[r][c]
	}
	return out
}

func eq(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTopViewHalfFilled(t *testing.T) {
	sp := Dims{10, 8, 10}
	top := buildViews(sp, []Placement{{Set: 0, Box: Dims{10, 4, 10}}}, 1).top
	if top.cols != 10 || top.rows != 4 {
		t.Fatalf("top is %d×%d, want 10×4", top.cols, top.rows)
	}
	// Y=0 is the driver's side, drawn at the bottom of the top view.
	if !eq(column(top, 3), []int{-1, -1, 0, 0}) {
		t.Errorf("column = %v, want the driver-side piece on the bottom two rows", column(top, 3))
	}
}

func TestStackedPiecesTopAndSide(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, Z: 0, Box: Dims{10, 10, 5}},
		{Set: 1, Z: 5, Box: Dims{10, 10, 5}},
	}
	v := buildViews(sp, placed, 1)
	if got := column(v.top, 0); !eq(got, []int{1, 1, 1, 1, 1}) {
		t.Errorf("top column = %v, want the upper piece everywhere", got)
	}
	// Row 0 is the ceiling: the upper piece occupies the top three rows, the lower one the rest.
	for name, side := range map[string]view{"driver": v.driver, "passenger": v.passenger} {
		if got := column(side, 0); !eq(got, []int{1, 1, 1, 0, 0}) {
			t.Errorf("%s column = %v, want [1 1 1 0 0]", name, got)
		}
	}
}

func TestRearViewShowsPieceNearestHatch(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, X: 0, Box: Dims{5, 10, 10}},
		{Set: 1, X: 5, Box: Dims{5, 10, 10}},
	}
	v := buildViews(sp, placed, 1)
	if got := column(v.rear, 0); !eq(got, []int{1, 1, 1, 1, 1}) {
		t.Errorf("rear column = %v, want the piece nearest the hatch", got)
	}
	if d := v.driver.owner[0]; d[0] != 0 || d[9] != 1 {
		t.Errorf("driver row = %v, want the front piece on the left and the hatch piece on the right", d)
	}
	if p := v.passenger.owner[0]; p[0] != 1 || p[9] != 0 {
		t.Errorf("passenger row = %v, want it mirrored: hatch piece on the left", p)
	}
}

func TestSideViewsShowPieceNearestViewer(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, Y: 0, Box: Dims{10, 5, 10}}, // driver's side
		{Set: 1, Y: 5, Box: Dims{10, 5, 10}}, // passenger's side
	}
	v := buildViews(sp, placed, 1)
	if v.driver.owner[0][0] != 0 || v.passenger.owner[0][0] != 1 {
		t.Errorf("driver=%d passenger=%d, want each side to show the piece against it",
			v.driver.owner[0][0], v.passenger.owner[0][0])
	}
	// Rear view: driver at the left. Top view: passenger up, driver down.
	if v.rear.owner[0][0] != 0 || v.rear.owner[0][9] != 1 {
		t.Errorf("rear row = %v, want driver piece left, passenger piece right", v.rear.owner[0])
	}
	if v.top.owner[0][0] != 1 || v.top.owner[v.top.rows-1][0] != 0 {
		t.Errorf("top column = %v, want passenger piece up, driver piece down", column(v.top, 0))
	}
}

func TestLabels(t *testing.T) {
	l := Load{Sets: make([]ItemSet, 60)}
	for i, want := range map[int]rune{0: 'A', 25: 'Z', 26: 'a', 51: 'z', 52: '*', 59: '*'} {
		if got := labelFor(l, i); got != want {
			t.Errorf("labelFor(%d) = %c, want %c", i, got, want)
		}
	}
	l = Load{Sets: make([]ItemSet, 2), Dolly: DollyWithItems}
	if labelFor(l, 2) != '=' || labelFor(l, 1) != 'B' {
		t.Error("the dolly, last in All(), should be =, and sets keep their letters")
	}
}

func TestRenderLayoutLegendAndContent(t *testing.T) {
	l := Load{Space: Dims{60, 24, 12}, Dolly: DollyWithItems, Sets: []ItemSet{{Qty: 2, Dims: Dims{10, 10, 6}}, {Qty: 1, Dims: Dims{100, 5, 5}}}}
	out := renderLayout(l, l.Pack(), "Test car", 80, 0)
	for _, want := range []string{
		"Top view", "Rear view", "Driver side", "Passenger side", "front", "hatch", "driver", "passenger", "Test car",
		"A  set #1", "placed 2 of 2", "=  dolly", "B  set #2", "1 not placed", "any other key returns", "Dolly: with items (48 × 24 × 6 in, packed to fit)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("layout missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if n := len([]rune(line)); n > 80 {
			t.Errorf("line is %d wide, want at most 80: %q", n, line)
		}
	}
}

func TestRenderLayoutEmptyAndSkipped(t *testing.T) {
	empty := Load{Space: Dims{10, 10, 10}}
	if out := renderLayout(empty, empty.Pack(), "", 80, 0); !strings.Contains(out, "Nothing to show") {
		t.Errorf("empty layout:\n%s", out)
	}
	big := Load{Space: Dims{100, 100, 100}, Sets: []ItemSet{{Qty: maxPackPieces + 1, Dims: Dims{1, 1, 1}}}}
	if out := renderLayout(big, big.Pack(), "", 80, 0); !strings.Contains(out, "skipped") {
		t.Errorf("skipped layout:\n%s", out)
	}
}

func sampleLoad() Load {
	return Load{Space: Vehicles[0].SeatsDown.Dims, Sets: []ItemSet{
		{Qty: 6, Dims: Dims{30, 20, 15}},
		{Qty: 10, Dims: Dims{12, 12, 12}},
		{Qty: 2, Dims: Dims{40, 30, 10}},
		{Qty: 3, Dims: Dims{20, 10, 8}},
	}}
}

func maxWidth(s string) int {
	w := 0
	for _, line := range strings.Split(s, "\n") {
		w = max(w, lipgloss.Width(line))
	}
	return w
}

func scaleOf(t *testing.T, out string) int {
	t.Helper()
	var n int
	i := strings.Index(out, "1 column ≈ ")
	if i < 0 {
		t.Fatalf("no scale note in:\n%s", out)
	}
	if _, err := fmt.Sscanf(out[i:], "1 column ≈ %d in", &n); err != nil {
		t.Fatalf("can't read the scale: %v", err)
	}
	return n
}

func lineWith(out string, parts ...string) bool {
	for _, line := range strings.Split(out, "\n") {
		all := true
		for _, p := range parts {
			all = all && strings.Contains(line, p)
		}
		if all {
			return true
		}
	}
	return false
}

func TestLayoutNeverExceedsWidth(t *testing.T) {
	l := sampleLoad()
	p := l.Pack()
	for width := 70; width <= 230; width += 9 {
		for _, height := range []int{0, 24, 40, 70} {
			if w := maxWidth(renderLayout(l, p, "Test car", width, height)); w > width {
				t.Errorf("width %d height %d: a line is %d wide", width, height, w)
			}
		}
	}
}

func TestLayoutFitsTheHeightWhenPossible(t *testing.T) {
	l := sampleLoad()
	p := l.Pack()
	for _, tc := range []struct{ width, height int }{{80, 30}, {80, 24}, {120, 24}, {120, 40}, {160, 24}, {200, 30}} {
		out := renderLayout(l, p, "Test car", tc.width, tc.height)
		if n := strings.Count(out, "\n"); n > tc.height-1 {
			t.Errorf("%d×%d: layout is %d lines, want at most %d so the top isn't cut off", tc.width, tc.height, n, tc.height-1)
		}
	}
}

func TestTallerTerminalGetsFinerScale(t *testing.T) {
	l := sampleLoad()
	p := l.Pack()
	short := scaleOf(t, renderLayout(l, p, "", 120, 24))
	tall := scaleOf(t, renderLayout(l, p, "", 120, 60))
	unknown := scaleOf(t, renderLayout(l, p, "", 120, 0))
	if !(short > tall) || tall != unknown {
		t.Errorf("scale: height 24 → %d, height 60 → %d, unknown → %d; want a shorter screen to be coarser", short, tall, unknown)
	}
}

func TestWideShortTerminalPutsViewsInOneRow(t *testing.T) {
	out := renderLayout(sampleLoad(), sampleLoad().Pack(), "", 200, 24)
	if !lineWith(out, "Top view", "Driver side", "Passenger side", "Rear view") {
		t.Errorf("all four titles should share a line when the screen is wide and short:\n%s", out)
	}
	grid := renderLayout(sampleLoad(), sampleLoad().Pack(), "", 80, 0)
	if lineWith(grid, "Top view", "Driver side") || !lineWith(grid, "Top view", "Rear view") || !lineWith(grid, "Driver side", "Passenger side") {
		t.Errorf("a narrow screen should use the 2×2 grid:\n%s", grid)
	}
}

func TestLegendUsesColumnsWhenWide(t *testing.T) {
	l := sampleLoad()
	p := l.Pack()
	// Columns fill top to bottom, so with four entries in two columns set #1 sits beside set #3.
	if wide := renderLayout(l, p, "", 200, 0); !lineWith(wide, "set #1", "set #3") {
		t.Errorf("a wide screen should list the legend in columns:\n%s", wide)
	}
	if narrow := renderLayout(l, p, "", 80, 0); lineWith(narrow, "set #1", "set #3") {
		t.Errorf("a narrow screen should list the legend in one column:\n%s", narrow)
	}
}

func TestLayoutTooNarrowStillRenders(t *testing.T) {
	if out := renderLayout(sampleLoad(), sampleLoad().Pack(), "", 20, 10); !strings.Contains(out, "Top view") {
		t.Errorf("a tiny terminal should still get a layout:\n%s", out)
	}
}
