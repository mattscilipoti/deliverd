package main

import "testing"

func load(space Dims, sets ...ItemSet) Load { return Load{Space: space, Sets: sets} }

func TestPackTwoCubesInTightBox(t *testing.T) {
	p := load(Dims{20, 10, 10}, ItemSet{Qty: 2, Dims: Dims{10, 10, 10}}).Pack()
	if !p.AllPlaced() || len(p.Placed) != 2 {
		t.Errorf("placed=%d unplaced=%v, want both placed", len(p.Placed), p.Unplaced)
	}
}

func TestPackReportsUnplacedBySet(t *testing.T) {
	p := load(Dims{20, 10, 10},
		ItemSet{Qty: 1, Dims: Dims{10, 10, 10}},
		ItemSet{Qty: 2, Dims: Dims{10, 10, 10}}).Pack()
	if p.AllPlaced() || p.UnplacedTotal() != 1 || len(p.Placed) != 2 {
		t.Errorf("placed=%d unplaced=%v, want 2 placed and 1 unplaced", len(p.Placed), p.Unplaced)
	}
}

func TestPackRotatesPieces(t *testing.T) {
	p := load(Dims{5, 5, 30}, ItemSet{Qty: 1, Dims: Dims{30, 5, 5}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want the piece rotated to fit", p.Unplaced)
	}
}

func TestPackStacksVertically(t *testing.T) {
	p := load(Dims{10, 10, 30}, ItemSet{Qty: 3, Dims: Dims{10, 10, 10}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want three stacked cubes", p.Unplaced)
	}
}

func TestPackMixedSizesFitCleanly(t *testing.T) {
	// Four 20x10x5 slabs fill a 20x10x20 box exactly.
	p := load(Dims{20, 10, 20}, ItemSet{Qty: 4, Dims: Dims{20, 10, 5}}).Pack()
	if !p.AllPlaced() {
		t.Errorf("unplaced=%v, want exact fill", p.Unplaced)
	}
}

// Whatever the heuristic does, placements must stay inside the box and never overlap.
func TestPackPlacementsAreValid(t *testing.T) {
	l := load(Dims{89, 42, 31},
		ItemSet{Qty: 3, Dims: Dims{30, 20, 15}},
		ItemSet{Qty: 5, Dims: Dims{12, 12, 12}},
		ItemSet{Qty: 2, Dims: Dims{40, 30, 10}},
		ItemSet{Qty: 4, Dims: Dims{18, 9, 6}},
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
	p := load(Dims{100, 100, 100}, ItemSet{Qty: maxPackPieces + 1, Dims: Dims{1, 1, 1}}).Pack()
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
	l := load(Dims{50, 24, 12}, ItemSet{Qty: 1, Dims: Dims{10, 10, 6}})
	l.Dolly = DollyWithItems
	p := l.Pack()
	if !p.AllPlaced() || len(p.Placed) != 2 || p.Total != 2 {
		t.Errorf("placed=%d unplaced=%v total=%d, want dolly and box placed", len(p.Placed), p.Unplaced, p.Total)
	}
}

func TestPackDollyLeavesNoRoom(t *testing.T) {
	// Volume and sizes pass, but the dolly fills the 24 in width and leaves 12 in of length.
	l := load(Dims{60, 24, 6}, ItemSet{Qty: 1, Dims: Dims{13, 13, 6}})
	l.Dolly = DollyWithItems
	if !l.Fits() {
		t.Fatal("setup: should pass the volume and size checks")
	}
	p := l.Pack()
	if p.AllPlaced() || p.UnplacedTotal() != 1 {
		t.Errorf("unplaced=%v, want exactly the box unplaced", p.Unplaced)
	}
}

func TestPackIgnoresDollyBehindSeats(t *testing.T) {
	l := load(Dims{10, 10, 10}, ItemSet{Qty: 1, Dims: Dims{10, 10, 10}})
	l.Dolly = DollyBehindSeats
	if p := l.Pack(); !p.AllPlaced() || p.Total != 1 {
		t.Errorf("Total=%d AllPlaced=%v, want only the item packed", p.Total, p.AllPlaced())
	}
}

func TestHeavyPiecesGoLowAndLightOnTop(t *testing.T) {
	// The light set is listed first, so only the weight ordering puts the heavy one on the floor.
	l := load(Dims{10, 10, 20},
		ItemSet{Qty: 1, Weight: 5, Dims: Dims{10, 10, 10}},
		ItemSet{Qty: 1, Weight: 50, Dims: Dims{10, 10, 10}})
	p := l.Pack()
	if !p.AllPlaced() {
		t.Fatalf("unplaced = %v", p.Unplaced)
	}
	for _, pl := range p.Placed {
		wantZ := 10.0
		if pl.Weight == 50 {
			wantZ = 0
		}
		if pl.Z != wantZ {
			t.Errorf("%v lb piece at z=%v, want %v", pl.Weight, pl.Z, wantZ)
		}
	}
}

func TestHeavyPiecesGoForward(t *testing.T) {
	l := load(Dims{30, 10, 10},
		ItemSet{Qty: 1, Weight: 5, Dims: Dims{10, 10, 10}},
		ItemSet{Qty: 1, Weight: 50, Dims: Dims{10, 10, 10}})
	for _, pl := range l.Pack().Placed {
		if pl.Weight == 50 && pl.X != 0 {
			t.Errorf("the heavy piece is at x=%v, want it at the front (x=0)", pl.X)
		}
	}
}

func TestRestsOK(t *testing.T) {
	floor := func(w float64, box Dims) Placement { return Placement{Box: box, Weight: w} }
	base := []Placement{floor(20, Dims{10, 10, 5})}
	on := point{0, 0, 5}
	for _, tc := range []struct {
		name   string
		placed []Placement
		pt     point
		box    Dims
		weight float64
		want   bool
	}{
		{"floor needs no support", nil, point{3, 3, 0}, Dims{4, 4, 4}, 99, true},
		{"lighter on heavier", base, on, Dims{10, 10, 5}, 10, true},
		{"equal weights stack", base, on, Dims{10, 10, 5}, 20, true},
		{"heavier on lighter", base, on, Dims{10, 10, 5}, 30, false},
		{"unweighted on weighted", base, on, Dims{10, 10, 5}, 0, true},
		{"weighted on unweighted", []Placement{floor(0, Dims{10, 10, 5})}, on, Dims{10, 10, 5}, 1, false},
		{"floating", nil, point{0, 0, 5}, Dims{4, 4, 4}, 1, false},
		{"hanging mostly off the edge", base, point{8, 0, 5}, Dims{10, 10, 5}, 1, false},
		{"enough overlap", base, point{3, 0, 5}, Dims{10, 10, 5}, 1, true}, // 70% supported
	} {
		if got := restsOK(tc.placed, tc.pt, tc.box, tc.weight); got != tc.want {
			t.Errorf("%s: restsOK = %v, want %v", tc.name, got, tc.want)
		}
	}
	// A piece may not slide under one that is heavier than it.
	shelf := []Placement{{Z: 5, Box: Dims{10, 10, 5}, Weight: 40}}
	if restsOK(shelf, point{0, 0, 0}, Dims{10, 10, 5}, 10) {
		t.Error("a 10 lb piece was allowed under a 40 lb piece")
	}
	if !restsOK(shelf, point{0, 0, 0}, Dims{10, 10, 5}, 40) {
		t.Error("a piece as heavy as the one above it should be allowed under it")
	}
}

// Whatever the packer does, the final arrangement must be stable and ordered by weight.
func TestPackWithWeightsIsStable(t *testing.T) {
	l := load(Dims{89, 42, 31},
		ItemSet{Qty: 3, Weight: 60, Dims: Dims{30, 20, 15}},
		ItemSet{Qty: 6, Weight: 8, Dims: Dims{12, 12, 12}},
		ItemSet{Qty: 2, Weight: 25, Dims: Dims{40, 30, 10}},
		ItemSet{Qty: 5, Dims: Dims{18, 9, 6}}, // no weight
	)
	p := l.Pack()
	if len(p.Placed) == 0 {
		t.Fatal("nothing placed")
	}
	for i, a := range p.Placed {
		others := append(append([]Placement(nil), p.Placed[:i]...), p.Placed[i+1:]...)
		if overlapsAny(others, point{a.X, a.Y, a.Z}, a.Box) {
			t.Errorf("placement %d overlaps another: %+v", i, a)
		}
		if !restsOK(others, point{a.X, a.Y, a.Z}, a.Box, a.Weight) {
			t.Errorf("placement %d is unstable or rests on a lighter piece: %+v", i, a)
		}
		if a.Weight != l.All()[a.Set].Weight {
			t.Errorf("placement %d carries weight %v, want %v", i, a.Weight, l.All()[a.Set].Weight)
		}
	}
}

func TestPackWithoutWeightsStillRequiresSupport(t *testing.T) {
	l := load(Dims{89, 42, 31},
		ItemSet{Qty: 4, Dims: Dims{30, 20, 15}},
		ItemSet{Qty: 8, Dims: Dims{12, 12, 12}},
		ItemSet{Qty: 3, Dims: Dims{22, 7, 5}})
	p := l.Pack()
	for i, a := range p.Placed {
		others := append(append([]Placement(nil), p.Placed[:i]...), p.Placed[i+1:]...)
		if !restsOK(others, point{a.X, a.Y, a.Z}, a.Box, 0) {
			t.Errorf("placement %d floats in the air: %+v", i, a)
		}
	}
}

func TestBalance(t *testing.T) {
	sp := Dims{100, 40, 40}
	p := Packing{Placed: []Placement{
		{X: 0, Z: 0, Box: Dims{10, 10, 10}, Weight: 30},   // front half, centre 5 in up
		{X: 80, Z: 10, Box: Dims{10, 10, 10}, Weight: 10}, // rear half, centre 15 in up
		{X: 50, Z: 0, Box: Dims{10, 10, 10}},              // no weight: ignored
	}}
	b, ok := p.Balance(sp)
	if !ok || b.Weight != 40 || b.FrontShare != 0.75 || b.CenterZ != 7.5 {
		t.Errorf("Balance = %+v, %v; want 40 lb, 0.75 front, 7.5 in up", b, ok)
	}
	if _, ok := (Packing{Placed: []Placement{{Box: Dims{1, 1, 1}}}}).Balance(sp); ok {
		t.Error("Balance should report ok=false when no placed piece has a weight")
	}
}
