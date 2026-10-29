package main

import "sort"

// maxPackPieces bounds the heuristic's cost; it is roughly cubic in the piece count.
const maxPackPieces = 150

const packEps = 1e-9

// Placement is one piece positioned in the cargo box. X runs along the
// space's length, Y along its width and Z along its height.
type Placement struct {
	Set     int // index into Load.Sets
	X, Y, Z float64
	Box     Dims // the piece as placed, after rotation
}

type Packing struct {
	Placed   []Placement
	Unplaced []int // per set: pieces the heuristic could not place
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
	set int
	d   Dims
}

// packOrders are the orderings tried; Pack keeps the one that places the most volume.
var packOrders = []struct {
	name string
	less func(a, b Dims) bool
}{
	{"volume", func(a, b Dims) bool { return a.Volume() > b.Volume() }},
	{"longest side", func(a, b Dims) bool { return a.sorted()[2] > b.sorted()[2] }},
	{"footprint", func(a, b Dims) bool { return footprint(a) > footprint(b) }},
}

// footprint is the area of the largest face, the way a piece lies if laid flat.
func footprint(d Dims) float64 {
	s := d.sorted()
	return s[1] * s[2]
}

// Pack is a first-fit-decreasing heuristic. Pieces are placed largest first at
// the lowest, then rearmost, then leftmost extreme point where some rotation
// fits without overlapping anything. It tries several orderings and keeps the
// best. It does not model weight or crushing, so pieces may rest on anything.
// A failure means "not found", not "impossible".
func (l Load) Pack() Packing {
	var pieces []piece
	for i, s := range l.Sets {
		for q := 0; q < s.Qty; q++ {
			pieces = append(pieces, piece{i, s.Dims})
		}
	}
	if len(pieces) > maxPackPieces {
		return Packing{Unplaced: make([]int, len(l.Sets)), Total: len(pieces), Skipped: true}
	}

	var best Packing
	var bestVol float64
	for i, o := range packOrders {
		sorted := append([]piece(nil), pieces...)
		sort.SliceStable(sorted, func(a, b int) bool { return o.less(sorted[a].d, sorted[b].d) })
		p := l.packInOrder(sorted)
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

func (l Load) packInOrder(pieces []piece) Packing {
	p := Packing{Unplaced: make([]int, len(l.Sets))}
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
				if overlapsAny(p.Placed, pt, o) {
					continue
				}
				p.Placed = append(p.Placed, Placement{Set: pc.set, X: pt.x, Y: pt.y, Z: pt.z, Box: o})
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
