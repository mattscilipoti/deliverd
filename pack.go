package main

import (
	"math"
	"sort"
)

// maxPackPieces bounds the heuristic's cost; it is roughly cubic in the piece count.
const maxPackPieces = 150

const packEps = 1e-9

// minSupport is the share of a piece's base that must rest on pieces directly
// below it when it is off the floor.
const minSupport = 0.6

// Placement is one piece positioned in the cargo box. X runs along the
// space's length, Y along its width and Z along its height.
type Placement struct {
	Set     int // index into Load.All
	X, Y, Z float64
	Box     Dims    // the piece as placed, after rotation
	Weight  float64 // lb, 0 if not entered
}

type Packing struct {
	Placed   []Placement
	Unplaced []int // per entry of Load.All: pieces the heuristic could not place
	Total    int   // pieces across all sets
	Skipped  bool  // too many pieces to try
}

func (p Packing) UnplacedTotal() int {
	var n int
	for _, u := range p.Unplaced {
		n += u
	}
	return n
}

func (p Packing) AllPlaced() bool { return !p.Skipped && p.UnplacedTotal() == 0 }

type point struct{ x, y, z float64 }

type piece struct {
	set    int
	d      Dims
	weight float64
}

type packOrder struct {
	name string
	less func(a, b piece) bool
}

// weightOrder places heavy pieces first, so they take the low, forward spots,
// and uses volume to break ties. It is only tried when some weight is known.
var weightOrder = packOrder{"weight", func(a, b piece) bool {
	if a.weight != b.weight {
		return a.weight > b.weight
	}
	return a.d.Volume() > b.d.Volume()
}}

// packOrders are the orderings tried; Pack keeps the one that places the most volume.
var packOrders = []packOrder{
	{"volume", func(a, b piece) bool { return a.d.Volume() > b.d.Volume() }},
	{"longest side", func(a, b piece) bool { return a.d.sorted()[2] > b.d.sorted()[2] }},
	{"footprint", func(a, b piece) bool { return footprint(a.d) > footprint(b.d) }},
}

// footprint is the area of the largest face, the way a piece lies if laid flat.
func footprint(d Dims) float64 {
	s := d.sorted()
	return s[1] * s[2]
}

// Pack is a first-fit-decreasing heuristic. Pieces are placed in order at the
// lowest, then frontmost, then leftmost extreme point where some rotation fits
// without overlapping anything. It tries several orderings and keeps the one
// that places the most volume.
//
// Pieces off the floor must rest on enough of the pieces below (minSupport),
// and a piece never rests on a lighter one, so with weights entered the heavy
// pieces sit low and forward and the light ones go on top. A piece with no
// weight counts as weightless: it can sit on anything, and nothing weighted
// sits on it. A failure means "not found", not "impossible".
func (l Load) Pack() Packing {
	all := l.All()
	var pieces []piece
	for i, s := range all {
		for q := 0; q < s.Qty; q++ {
			pieces = append(pieces, piece{i, s.Dims, s.Weight})
		}
	}
	if len(pieces) > maxPackPieces {
		return Packing{Unplaced: make([]int, len(all)), Total: len(pieces), Skipped: true}
	}

	orders := packOrders
	for _, pc := range pieces {
		if pc.weight > 0 {
			orders = append([]packOrder{weightOrder}, packOrders...)
			break
		}
	}

	var best Packing
	var bestVol float64
	for i, o := range orders {
		sorted := append([]piece(nil), pieces...)
		sort.SliceStable(sorted, func(a, b int) bool { return o.less(sorted[a], sorted[b]) })
		p := l.packInOrder(sorted, len(all))
		var vol float64
		for _, pl := range p.Placed {
			vol += pl.Box.Volume()
		}
		if i == 0 || vol > bestVol {
			best, bestVol = p, vol
		}
		if p.AllPlaced() {
			break
		}
	}
	return best
}

func (l Load) packInOrder(pieces []piece, entries int) Packing {
	p := Packing{Unplaced: make([]int, entries)}
	p.Total = len(pieces)

	sp := l.Space
	points := []point{{}}
	for _, pc := range pieces {
		sort.Slice(points, func(a, b int) bool {
			pa, pb := points[a], points[b]
			if pa.z != pb.z {
				return pa.z < pb.z
			}
			if pa.x != pb.x {
				return pa.x < pb.x
			}
			return pa.y < pb.y
		})

		placed := false
	search:
		for pi, pt := range points {
			for _, o := range orientations(pc.d) {
				if pt.x+o.L > sp.L+packEps || pt.y+o.W > sp.W+packEps || pt.z+o.H > sp.H+packEps {
					continue
				}
				if overlapsAny(p.Placed, pt, o) || !restsOK(p.Placed, pt, o, pc.weight) {
					continue
				}
				p.Placed = append(p.Placed, Placement{Set: pc.set, X: pt.x, Y: pt.y, Z: pt.z, Box: o, Weight: pc.weight})
				points = append(points[:pi], points[pi+1:]...)
				for _, np := range []point{{pt.x + o.L, pt.y, pt.z}, {pt.x, pt.y + o.W, pt.z}, {pt.x, pt.y, pt.z + o.H}} {
					if np.x < sp.L-packEps && np.y < sp.W-packEps && np.z < sp.H-packEps && !hasPoint(points, np) {
						points = append(points, np)
					}
				}
				placed = true
				break search
			}
		}
		if !placed {
			p.Unplaced[pc.set]++
		}
	}
	return p
}

// orientations returns the distinct rotations of d, flattest first so pieces
// lie low and leave headroom, then longest along the length axis.
func orientations(d Dims) []Dims {
	perms := []Dims{
		{d.L, d.W, d.H}, {d.L, d.H, d.W},
		{d.W, d.L, d.H}, {d.W, d.H, d.L},
		{d.H, d.L, d.W}, {d.H, d.W, d.L},
	}
	var out []Dims
	for _, o := range perms {
		dup := false
		for _, e := range out {
			if e == o {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].H != out[b].H {
			return out[a].H < out[b].H
		}
		return out[a].L > out[b].L
	})
	return out
}

func hasPoint(points []point, np point) bool {
	for _, q := range points {
		if q == np {
			return true
		}
	}
	return false
}

func overlapsAny(placed []Placement, pt point, o Dims) bool {
	for _, q := range placed {
		if pt.x < q.X+q.Box.L-packEps && q.X < pt.x+o.L-packEps &&
			pt.y < q.Y+q.Box.W-packEps && q.Y < pt.y+o.W-packEps &&
			pt.z < q.Z+q.Box.H-packEps && q.Z < pt.z+o.H-packEps {
			return true
		}
	}
	return false
}

// overlap1 is how far [a, a+alen) and [b, b+blen) overlap.
func overlap1(a, alen, b, blen float64) float64 {
	return math.Min(a+alen, b+blen) - math.Max(a, b)
}

// restsOK reports whether a piece of the given weight at pt, oriented as o, is
// stable: off the floor it needs minSupport of its base on the pieces directly
// below, and it may not rest on a lighter piece or have a heavier one rest on it.
func restsOK(placed []Placement, pt point, o Dims, weight float64) bool {
	var supported float64
	for _, q := range placed {
		ox := overlap1(pt.x, o.L, q.X, q.Box.L)
		oy := overlap1(pt.y, o.W, q.Y, q.Box.W)
		if ox <= packEps || oy <= packEps {
			continue
		}
		switch {
		case math.Abs(q.Z+q.Box.H-pt.z) <= packEps: // q is directly below
			if q.Weight < weight {
				return false
			}
			supported += ox * oy
		case math.Abs(pt.z+o.H-q.Z) <= packEps: // q rests directly on this piece
			if weight < q.Weight {
				return false
			}
		}
	}
	return pt.z <= packEps || supported >= minSupport*o.L*o.W-packEps
}

// Balance describes where the entered weight sits in the cargo area.
type Balance struct {
	Weight     float64 // lb of placed pieces that have a weight
	FrontShare float64 // 0–1, share of that weight whose centre is in the front half
	CenterZ    float64 // height of the centre of mass above the floor, in
}

// Balance reports the weight distribution of the placed pieces. ok is false
// when none of them has a weight.
func (p Packing) Balance(sp Dims) (b Balance, ok bool) {
	var front, moment float64
	for _, pl := range p.Placed {
		if pl.Weight <= 0 {
			continue
		}
		b.Weight += pl.Weight
		if pl.X+pl.Box.L/2 < sp.L/2 {
			front += pl.Weight
		}
		moment += pl.Weight * (pl.Z + pl.Box.H/2)
	}
	if b.Weight == 0 {
		return Balance{}, false
	}
	b.FrontShare = front / b.Weight
	b.CenterZ = moment / b.Weight
	return b, true
}
