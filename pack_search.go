package main

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// maxSearchPieces bounds PackSearch. Each trial is a full pack, which grows
// roughly with the cube of the piece count, so this is more than Pack allows
// (maxPackPieces) but still finishes a useful number of trials in a minute.
const maxSearchPieces = 400

const defaultSearchBudget = 10 * time.Second

// SearchOptions control PackSearch. It stops at Budget or MaxIters, whichever
// comes first, when the context is cancelled, or as soon as every piece fits.
type SearchOptions struct {
	Budget   time.Duration // wall-clock limit; also paces the cooling
	MaxIters int           // trial limit; 0 for none
	Seed     int64
	// OnImprove is called each time a better layout is found, on the search's goroutine.
	OnImprove func(best Packing, iters int)
}

type SearchStats struct {
	Iterations  int
	Elapsed     time.Duration
	StartPlaced int // pieces placed by the quick packer, which the search starts from
	BestPlaced  int
	Pieces      int
	Skipped     bool // too many pieces to search
	Stopped     bool // ended because the context was cancelled
}

// genome is one candidate: the order pieces are placed in, and for each piece
// (indexed by id) which orientation to try first.
type genome struct {
	order []int
	rot   []int
}

func (g genome) clone() genome {
	return genome{append([]int(nil), g.order...), append([]int(nil), g.rot...)}
}

// decode lays the pieces out in the genome's order with the packer.
func (l Load) decode(pieces []piece, g genome) Packing {
	ordered := make([]piece, len(g.order))
	for i, id := range g.order {
		ordered[i] = pieces[id]
		ordered[i].rot = g.rot[id]
	}
	return l.packInOrder(ordered, len(l.All()))
}

// score ranks layouts by the volume placed, then by the number of pieces.
func (p Packing) score() float64 { return p.placedVolume() + 1e-3*float64(len(p.Placed)) }

// PackSearch looks for a layout that places more than Pack does. It runs
// simulated annealing over the order pieces are placed in and each piece's
// preferred orientation, using the same packer (and so the same support and
// weight rules) to turn an order into a layout. It starts from the best of
// Pack's orderings, so the result is never worse than Pack's. Like Pack it is
// a heuristic: it can fail to find a layout that exists.
func (l Load) PackSearch(ctx context.Context, opt SearchOptions) (Packing, SearchStats) {
	began := time.Now()
	pieces := l.pieces()
	n := len(pieces)
	st := SearchStats{Pieces: n}
	if n > maxSearchPieces {
		st.Skipped = true
		return Packing{Unplaced: make([]int, len(l.All())), Total: n, Skipped: true}, st
	}
	if n == 0 {
		return l.packInOrder(nil, len(l.All())), st
	}
	budget := opt.Budget
	if budget <= 0 && opt.MaxIters <= 0 {
		budget = defaultSearchBudget
	}
	rng := rand.New(rand.NewSource(opt.Seed))

	// Start from the best of the quick packer's orderings.
	var cur genome
	var curP Packing
	for i, seed := range l.baselineOrders(pieces) {
		g := genome{order: make([]int, n), rot: make([]int, n)}
		for k, pc := range seed {
			g.order[k] = pc.id
		}
		p := l.decode(pieces, g)
		if i == 0 || p.score() > curP.score() {
			cur, curP = g, p
		}
		if p.AllPlaced() {
			break
		}
	}
	bestP := curP
	st.StartPlaced, st.BestPlaced = len(curP.Placed), len(curP.Placed)

	meanVol := 0.0
	for _, pc := range pieces {
		meanVol += pc.d.Volume() / float64(n)
	}
	t0 := 0.5 * meanVol // early on, losing a typical piece is accepted fairly often

	for !bestP.AllPlaced() {
		elapsed := time.Since(began)
		if ctx.Err() != nil {
			st.Stopped = true
			break
		}
		if (opt.MaxIters > 0 && st.Iterations >= opt.MaxIters) || (budget > 0 && elapsed >= budget) {
			break
		}
		var progress float64 // 0 at the start, 1 at the limit
		if budget > 0 {
			progress = float64(elapsed) / float64(budget)
		}
		if opt.MaxIters > 0 {
			progress = math.Max(progress, float64(st.Iterations)/float64(opt.MaxIters))
		}

		cand := cur.clone()
		mutate(&cand, curP, rng)
		p := l.decode(pieces, cand)
		st.Iterations++

		temp := t0*(1-progress)*(1-progress) + 1e-12
		delta := p.score() - curP.score()
		if delta >= 0 || rng.Float64() < math.Exp(delta/temp) {
			cur, curP = cand, p
		}
		if p.score() > bestP.score() {
			bestP = p
			st.BestPlaced = len(p.Placed)
			if opt.OnImprove != nil {
				opt.OnImprove(p, st.Iterations)
			}
		}
	}
	st.Elapsed = time.Since(began)
	return bestP, st
}

// mutate makes one random change to g. Half the time it targets a piece that
// did not fit in the layout curP was decoded from, moving it earlier.
func mutate(g *genome, curP Packing, rng *rand.Rand) {
	n := len(g.order)
	move := func(from, to int) {
		id := g.order[from]
		if from < to {
			copy(g.order[from:], g.order[from+1:to+1])
		} else {
			copy(g.order[to+1:], g.order[to:from])
		}
		g.order[to] = id
	}
	switch r := rng.Intn(10); {
	case r < 5 && len(curP.unplacedAt) > 0:
		pos := curP.unplacedAt[rng.Intn(len(curP.unplacedAt))]
		move(pos, rng.Intn(pos+1))
	case r < 7:
		a, b := rng.Intn(n), rng.Intn(n)
		g.order[a], g.order[b] = g.order[b], g.order[a]
	case r < 9:
		move(rng.Intn(n), rng.Intn(n))
	default:
		g.rot[rng.Intn(n)] = rng.Intn(6)
	}
}
