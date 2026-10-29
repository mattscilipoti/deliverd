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
	top, _, _ := buildViews(sp, []Placement{{Set: 0, Box: Dims{10, 4, 10}}}, 1)
	if top.cols != 10 || top.rows != 4 {
		t.Fatalf("top is %d×%d, want 10×4", top.cols, top.rows)
	}
	if !eq(column(top, 3), []int{0, 0, -1, -1}) {
		t.Errorf("column = %v, want the piece on the first two rows only", column(top, 3))
	}
}

func TestStackedPiecesTopAndSide(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, Z: 0, Box: Dims{10, 10, 5}},
		{Set: 1, Z: 5, Box: Dims{10, 10, 5}},
	}
	top, side, _ := buildViews(sp, placed, 1)
	if got := column(top, 0); !eq(got, []int{1, 1, 1, 1, 1}) {
		t.Errorf("top column = %v, want the upper piece everywhere", got)
	}
	// Row 0 is the ceiling: the upper piece occupies the top three rows, the lower one the rest.
	if got := column(side, 0); !eq(got, []int{1, 1, 1, 0, 0}) {
		t.Errorf("side column = %v, want [1 1 1 0 0]", got)
	}
}

func TestRearViewShowsPieceNearestHatch(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, X: 0, Box: Dims{5, 10, 10}},
		{Set: 1, X: 5, Box: Dims{5, 10, 10}},
	}
	_, side, rear := buildViews(sp, placed, 1)
	if got := column(rear, 0); !eq(got, []int{1, 1, 1, 1, 1}) {
		t.Errorf("rear column = %v, want the piece nearest the hatch", got)
	}
	if side.owner[0][0] != 0 || side.owner[0][9] != 1 {
		t.Errorf("side row = %v, want front piece on the left and hatch piece on the right", side.owner[0])
	}
}

func TestSideViewShowsPieceNearestViewer(t *testing.T) {
	sp := Dims{10, 10, 10}
	placed := []Placement{
		{Set: 0, Y: 0, Box: Dims{10, 5, 10}},
		{Set: 1, Y: 5, Box: Dims{10, 5, 10}},
	}
	_, side, _ := buildViews(sp, placed, 1)
	if side.owner[0][0] != 1 {
		t.Errorf("side cell = %d, want the piece with the larger Y", side.owner[0][0])
	}
}

func TestChooseScaleFitsWidth(t *testing.T) {
	sp := Vehicles[0].SeatsDown.Dims
	if got := chooseScale(sp, 80); got != 2 {
		t.Errorf("scale at 80 = %v, want 2", got)
	}
	if got := chooseScale(sp, 140); got != 1 {
		t.Errorf("scale at 140 = %v, want 1", got)
	}
	for w := 40; w <= 200; w += 7 {
		s := chooseScale(sp, w)
		if used := cells(sp.L, s) + cells(sp.W, s) + layoutMargin + layoutGap; used > w {
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
		"Top view", "Side view", "Rear view", "front", "hatch", "Test car",
		"A  set #1", "placed 2 of 2", "=  dolly", "B  set #2", "1 not placed", "Press any key",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("layout missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if n := len([]rune(line)); n > 80 && !strings.Contains(line, "placed") && !strings.Contains(line, "Cargo") && !strings.Contains(line, "Pieces thinner") {
			t.Errorf("grid line is %d wide, want at most 80: %q", n, line)
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
