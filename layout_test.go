package main

import (
	"strings"
	"testing"
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

func TestChooseScaleFitsWidth(t *testing.T) {
	sp := Vehicles[0].SeatsDown.Dims
	// Two side views sit next to each other, so they set the width.
	for width, want := range map[int]float64{80: 3, 140: 2, 200: 1} {
		if got := chooseScale(sp, width); got != want {
			t.Errorf("scale at %d = %v, want %v", width, got, want)
		}
	}
	for w := 40; w <= 220; w += 7 {
		s := chooseScale(sp, w)
		l, wd := cells(sp.L, s), cells(sp.W, s)
		if used := max(l+wd, 2*l) + layoutMargin + layoutGap; used > w {
			t.Errorf("width %d: scale %v needs %d columns", w, s, used)
		}
	}
}

func TestLabels(t *testing.T) {
	l := Load{Sets: make([]ItemSet, 60)}
	for i, want := range map[int]rune{0: 'A', 25: 'Z', 26: 'a', 51: 'z', 52: '*', 59: '*'} {
		if got := labelFor(l, i); got != want {
			t.Errorf("labelFor(%d) = %c, want %c", i, got, want)
		}
	}
	l = Load{Sets: make([]ItemSet, 2), Dolly: true}
	if labelFor(l, 2) != '=' || labelFor(l, 1) != 'B' {
		t.Error("the dolly, last in All(), should be =, and sets keep their letters")
	}
}

func TestRenderLayoutLegendAndContent(t *testing.T) {
	l := Load{Space: Dims{60, 24, 12}, Dolly: true, Sets: []ItemSet{{2, Dims{10, 10, 6}}, {1, Dims{100, 5, 5}}}}
	out := renderLayout(l, l.Pack(), "Test car", 80)
	for _, want := range []string{
		"Top view", "Rear view", "Driver side", "Passenger side", "front", "hatch", "driver", "passenger", "Test car",
		"A  set #1", "placed 2 of 2", "=  dolly", "B  set #2", "1 not placed", "Press any key",
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
	if out := renderLayout(empty, empty.Pack(), "", 80); !strings.Contains(out, "Nothing to show") {
		t.Errorf("empty layout:\n%s", out)
	}
	big := Load{Space: Dims{100, 100, 100}, Sets: []ItemSet{{maxPackPieces + 1, Dims{1, 1, 1}}}}
	if out := renderLayout(big, big.Pack(), "", 80); !strings.Contains(out, "skipped") {
		t.Errorf("skipped layout:\n%s", out)
	}
}
