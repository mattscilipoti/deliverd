package main

import "testing"

func load(space Dims, sets ...ItemSet) Load { return Load{Space: space, Sets: sets} }

func TestPackTwoCubesInTightBox(t *testing.T) {
	p := load(Dims{20, 10, 10}, ItemSet{2, Dims{10, 10, 10}}).Pack()
	if !p.AllPlaced() || len(p.Placed) != 2 {
		t.Errorf("placed=%d unplaced=%v, want both placed", len(p.Placed), p.Unplaced)
	}
}

func TestPackReportsUnplacedBySet(t *testing.T) {
	p := load(Dims{20, 10, 10},
		ItemSet{1, Dims{10, 10, 10}},
		ItemSet{2, Dims{10, 10, 10}}).Pack()
	if p.AllPlaced() || p.UnplacedTotal() != 1 || len(p.Placed) != 2 {
		t.Errorf("placed=%d unplaced=%v, want 2 placed and 1 unplaced", len(p.Placed), p.Unplaced)
	}
}

func TestPackRotatesPieces(t *testing.T) {
	p := load(Dims{5, 5, 30}, ItemSet{1, Dims{30, 5, 5}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want the piece rotated to fit", p.Unplaced)
	}
}

func TestPackStacksVertically(t *testing.T) {
	p := load(Dims{10, 10, 30}, ItemSet{3, Dims{10, 10, 10}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want three stacked cubes", p.Unplaced)
	}
}

func TestPackMixedSizesFitCleanly(t *testing.T) {
	// Four 20x10x5 slabs fill a 20x10x20 box exactly.
	p := load(Dims{20, 10, 20}, ItemSet{4, Dims{20, 10, 5}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want exact fill", p.Unplaced)
	}
}

// Whatever the heuristic does, placements must stay inside the box and never overlap.
func TestPackPlacementsAreValid(t *testing.T) {
	l := load(Dims{89, 42, 31},
		ItemSet{3, Dims{30, 20, 15}},
		ItemSet{5, Dims{12, 12, 12}},
		ItemSet{2, Dims{40, 30, 10}},
		ItemSet{4, Dims{18, 9, 6}},
	)
	p := l.Pack()
	for i, a := range p.Placed {
		if a.X < 0 || a.Y < 0 || a.Z < 0 ||
			a.X+a.Box.L > l.Space.L+packEps || a.Y+a.Box.W > l.Space.W+packEps || a.Z+a.Box.H > l.Space.H+packEps {
			t.Errorf("placement %d outside the box: %+v", i, a)
		}
		if a.Box.sorted() != l.Sets[a.Set].Dims.sorted() {
			t.Errorf("placement %d changed dimensions: %v vs %v", i, a.Box, l.Sets[a.Set].Dims)
		}
		if overlapsAny(p.Placed[:i], point{a.X, a.Y, a.Z}, a.Box) {
			t.Errorf("placement %d overlaps an earlier one: %+v", i, a)
		}
	}
	if got := len(p.Placed) + p.UnplacedTotal(); got != p.Total {
		t.Errorf("placed+unplaced = %d, want %d", got, p.Total)
	}
}

func TestPackSkipsHugeLoads(t *testing.T) {
	p := load(Dims{100, 100, 100}, ItemSet{maxPackPieces + 1, Dims{1, 1, 1}}).Pack()
	if !p.Skipped || p.AllPlaced() {
		t.Errorf("Skipped=%v, want skipped and not reported as placed", p.Skipped)
	}
}

func TestPackEmptyLoad(t *testing.T) {
	p := load(Dims{10, 10, 10}).Pack()
	if !p.AllPlaced() || p.Total != 0 {
		t.Errorf("Total=%d AllPlaced=%v", p.Total, p.AllPlaced())
	}
}

func TestPackIncludesDolly(t *testing.T) {
	// 50x24x12 fits the 48x24x6 dolly with a 10x10x6 box on top.
	l := load(Dims{50, 24, 12}, ItemSet{1, Dims{10, 10, 6}})
	l.Dolly = true
	p := l.Pack()
	if !p.AllPlaced() || len(p.Placed) != 2 || p.Total != 2 {
		t.Errorf("placed=%d unplaced=%v total=%d, want dolly and box placed", len(p.Placed), p.Unplaced, p.Total)
	}
}

func TestPackDollyLeavesNoRoom(t *testing.T) {
	// Volume and sizes pass, but the dolly fills the 24 in width and leaves 12 in of length.
	l := load(Dims{60, 24, 6}, ItemSet{1, Dims{13, 13, 6}})
	l.Dolly = true
	if !l.Fits() {
		t.Fatal("setup: should pass the volume and size checks")
	}
	p := l.Pack()
	if p.AllPlaced() || p.UnplacedTotal() != 1 {
		t.Errorf("unplaced=%v, want exactly the box unplaced", p.Unplaced)
	}
}
