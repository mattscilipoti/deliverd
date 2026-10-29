package main

import (
	"context"
	"math/rand"
	"testing"
	"time"
)

// assertValid checks a packing is a legal layout of load l: inside the box,
// no overlaps, stable and weight-ordered, and every piece accounted for.
func assertValid(t *testing.T, l Load, p Packing) {
	t.Helper()
	for i, a := range p.Placed {
		if a.X < -packEps || a.Y < -packEps || a.Z < -packEps ||
			a.X+a.Box.L > l.Space.L+packEps || a.Y+a.Box.W > l.Space.W+packEps || a.Z+a.Box.H > l.Space.H+packEps {
			t.Errorf("placement %d is outside the cargo area: %+v", i, a)
		}
		if a.Box.sorted() != l.All()[a.Set].Dims.sorted() {
			t.Errorf("placement %d changed the piece's dimensions: %v vs %v", i, a.Box, l.All()[a.Set].Dims)
		}
		if a.Weight != l.All()[a.Set].Weight {
			t.Errorf("placement %d has weight %v, want %v", i, a.Weight, l.All()[a.Set].Weight)
		}
		others := append(append([]Placement(nil), p.Placed[:i]...), p.Placed[i+1:]...)
		if overlapsAny(others, point{a.X, a.Y, a.Z}, a.Box) {
			t.Errorf("placement %d overlaps another piece: %+v", i, a)
		}
		if !restsOK(others, point{a.X, a.Y, a.Z}, a.Box, a.Weight) {
			t.Errorf("placement %d is unsupported or rests on a lighter piece: %+v", i, a)
		}
	}
	if got := len(p.Placed) + p.UnplacedTotal(); got != p.Total {
		t.Errorf("placed %d + unplaced %d != total %d", len(p.Placed), p.UnplacedTotal(), p.Total)
	}
}

func search(l Load, iters int, seed int64) (Packing, SearchStats) {
	return l.PackSearch(context.Background(), SearchOptions{MaxIters: iters, Seed: seed})
}

func TestSearchBeatsQuickPack(t *testing.T) {
	l := sampleLoad() // the quick packer leaves 8 of 21 pieces behind
	quick := l.Pack()
	best, st := search(l, 3000, 1)
	if len(best.Placed) <= len(quick.Placed) {
		t.Fatalf("search placed %d, quick pack placed %d; want an improvement", len(best.Placed), len(quick.Placed))
	}
	if st.StartPlaced != len(quick.Placed) || st.BestPlaced != len(best.Placed) || st.Iterations == 0 {
		t.Errorf("stats = %+v, want start=%d best=%d", st, len(quick.Placed), len(best.Placed))
	}
	assertValid(t, l, best)
}

func TestSearchResultsAreValidWithWeights(t *testing.T) {
	l := Load{Space: Vehicles[0].SeatsDown.Dims, Sets: []ItemSet{
		{Qty: 6, Weight: 40, Dims: Dims{30, 20, 15}},
		{Qty: 10, Weight: 6, Dims: Dims{12, 12, 12}},
		{Qty: 2, Weight: 25, Dims: Dims{40, 30, 10}},
		{Qty: 5, Dims: Dims{18, 9, 6}},
	}, Dolly: DollyWithItems}
	best, _ := search(l, 1500, 7)
	assertValid(t, l, best)
}

func TestSearchIsNeverWorseThanQuickPack(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 25; i++ {
		l := Load{Space: Dims{float64(40 + rng.Intn(60)), float64(30 + rng.Intn(30)), float64(25 + rng.Intn(25))}}
		for s := 0; s < 1+rng.Intn(4); s++ {
			l.Sets = append(l.Sets, ItemSet{
				Qty:    1 + rng.Intn(8),
				Weight: float64(rng.Intn(3) * 10),
				Dims:   Dims{float64(5 + rng.Intn(30)), float64(5 + rng.Intn(20)), float64(3 + rng.Intn(15))},
			})
		}
		quick := l.Pack()
		best, _ := search(l, 150, int64(i))
		if best.placedVolume() < quick.placedVolume() {
			t.Errorf("case %d: search placed %.0f in³, quick pack %.0f in³", i, best.placedVolume(), quick.placedVolume())
		}
		assertValid(t, l, best)
	}
}

func TestSearchStopsWhenEverythingFits(t *testing.T) {
	l := Load{Space: Dims{20, 10, 10}, Sets: []ItemSet{{Qty: 2, Dims: Dims{10, 10, 10}}}}
	best, st := search(l, 1000, 1)
	if !best.AllPlaced() || st.Iterations != 0 {
		t.Errorf("AllPlaced=%v iterations=%d; a load the quick packer solves needs no search", best.AllPlaced(), st.Iterations)
	}
	// And a load only the search solves stops as soon as it does.
	hard := sampleLoad()
	best, st = search(hard, 100000, 1)
	if !best.AllPlaced() || st.Iterations >= 100000 {
		t.Errorf("AllPlaced=%v iterations=%d; the search should stop once everything is placed", best.AllPlaced(), st.Iterations)
	}
}

func TestSearchIsDeterministicForASeed(t *testing.T) {
	l := Load{Space: Dims{60, 30, 25}, Sets: []ItemSet{
		{Qty: 9, Dims: Dims{14, 11, 9}}, {Qty: 7, Dims: Dims{17, 8, 12}}, {Qty: 5, Dims: Dims{20, 13, 6}},
	}}
	a, _ := search(l, 400, 99)
	b, _ := search(l, 400, 99)
	if len(a.Placed) != len(b.Placed) {
		t.Fatalf("same seed placed %d and %d", len(a.Placed), len(b.Placed))
	}
	for i := range a.Placed {
		if a.Placed[i] != b.Placed[i] {
			t.Fatalf("placement %d differs: %+v vs %+v", i, a.Placed[i], b.Placed[i])
		}
	}
}

func TestSearchReportsEachImprovement(t *testing.T) {
	var scores []float64
	var last Packing
	l := sampleLoad()
	best, _ := l.PackSearch(context.Background(), SearchOptions{MaxIters: 3000, Seed: 1,
		OnImprove: func(p Packing, iters int) { scores = append(scores, p.score()); last = p }})
	if len(scores) == 0 {
		t.Fatal("OnImprove was never called")
	}
	for i := 1; i < len(scores); i++ {
		if scores[i] <= scores[i-1] {
			t.Errorf("improvement %d (%v) is not better than the one before (%v)", i, scores[i], scores[i-1])
		}
	}
	if last.score() != best.score() {
		t.Errorf("the last reported layout (%v) should be the result (%v)", last.score(), best.score())
	}
}

func TestSearchStopsWhenCancelled(t *testing.T) {
	l := Load{Space: Dims{20, 20, 20}, Sets: []ItemSet{{Qty: 120, Dims: Dims{6, 6, 6}}}} // too much to fit
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	begin := time.Now()
	best, st := l.PackSearch(ctx, SearchOptions{Budget: time.Minute, Seed: 1})
	if time.Since(begin) > 3*time.Second || !st.Stopped {
		t.Errorf("took %v, Stopped=%v; want it to stop soon after cancellation", time.Since(begin), st.Stopped)
	}
	if len(best.Placed) < len(l.Pack().Placed) {
		t.Error("a cancelled search must still return at least the quick packer's layout")
	}
}

func TestSearchRespectsBudget(t *testing.T) {
	l := Load{Space: Dims{20, 20, 20}, Sets: []ItemSet{{Qty: 120, Dims: Dims{6, 6, 6}}}}
	begin := time.Now()
	_, st := l.PackSearch(context.Background(), SearchOptions{Budget: 200 * time.Millisecond, Seed: 1})
	if d := time.Since(begin); d < 150*time.Millisecond || d > 3*time.Second || st.Stopped {
		t.Errorf("took %v (Stopped=%v), want about the 200ms budget", d, st.Stopped)
	}
}

func TestSearchSkipsHugeLoadsAndHandlesEmpty(t *testing.T) {
	big := Load{Space: Dims{1000, 1000, 1000}, Sets: []ItemSet{{Qty: maxSearchPieces + 1, Dims: Dims{1, 1, 1}}}}
	if p, st := search(big, 10, 1); !p.Skipped || !st.Skipped {
		t.Error("more than maxSearchPieces should be skipped")
	}
	empty := Load{Space: Dims{10, 10, 10}}
	if p, _ := search(empty, 10, 1); !p.AllPlaced() || p.Total != 0 {
		t.Errorf("empty load: AllPlaced=%v Total=%d", p.AllPlaced(), p.Total)
	}
}

func TestSearchHandlesMoreThanQuickPackLimit(t *testing.T) {
	// 200 pieces is over Pack's cap but within the search's.
	l := Load{Space: Dims{192, 96, 96}, Sets: []ItemSet{{Qty: 200, Dims: Dims{20, 16, 12}}}}
	if !l.Pack().Skipped {
		t.Fatal("setup: the quick packer should skip 200 pieces")
	}
	best, _ := search(l, 5, 1)
	if best.Skipped || len(best.Placed) == 0 {
		t.Errorf("Skipped=%v placed=%d; the search should lay out 200 pieces", best.Skipped, len(best.Placed))
	}
	assertValid(t, l, best)
}

func TestOrientationsFrom(t *testing.T) {
	d := Dims{10, 20, 30}
	all := orientations(d)
	for first := range all {
		got := orientationsFrom(d, first)
		if got[0] != all[first] || len(got) != len(all) {
			t.Errorf("first=%d: got %v", first, got)
		}
	}
	if got := orientationsFrom(d, 99); got[0] != all[0] {
		t.Error("an out-of-range preference should fall back to the default order")
	}
}
