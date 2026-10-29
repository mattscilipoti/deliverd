package main

import "sort"

const cubicInchesPerCubicFoot = 1728

type Dims struct{ L, W, H float64 }

func (d Dims) Volume() float64 { return d.L * d.W * d.H }

func (d Dims) sorted() [3]float64 {
	s := [3]float64{d.L, d.W, d.H}
	sort.Float64s(s[:])
	return s
}

// FitsInside reports whether item fits in space in at least one of the six
// axis-aligned orientations. Comparing the sorted dimensions covers them all.
func FitsInside(item, space Dims) bool {
	i, s := item.sorted(), space.sorted()
	for k := range i {
		if i[k] > s[k] {
			return false
		}
	}
	return true
}

type ItemSet struct {
	Qty int
	Dims
}

func (s ItemSet) TotalVolume() float64 { return float64(s.Qty) * s.Volume() }

type Load struct {
	Space Dims
	Sets  []ItemSet
}

func (l Load) Capacity() float64 { return l.Space.Volume() }

func (l Load) Used() float64 {
	var total float64
	for _, s := range l.Sets {
		total += s.TotalVolume()
	}
	return total
}

// Available is negative when the load exceeds the cargo volume.
func (l Load) Available() float64 { return l.Capacity() - l.Used() }

// Oversized returns the indexes of sets whose items cannot fit in the space in any orientation.
func (l Load) Oversized() []int {
	var idx []int
	for i, s := range l.Sets {
		if !FitsInside(s.Dims, l.Space) {
			idx = append(idx, i)
		}
	}
	return idx
}

// Fits is an estimate: total volume is within capacity and every item fits on its own.
func (l Load) Fits() bool {
	return l.Available() >= 0 && len(l.Oversized()) == 0
}

// Add appends s, or when merge is set and a set with identical dimensions
// exists, adds to that set's quantity. It returns the index of the set.
func (l *Load) Add(s ItemSet, merge bool) int {
	if merge {
		for i := range l.Sets {
			if l.Sets[i].Dims == s.Dims {
				l.Sets[i].Qty += s.Qty
				return i
			}
		}
	}
	l.Sets = append(l.Sets, s)
	return len(l.Sets) - 1
}

// Replace overwrites set i with s, then merges duplicates if merge is set.
// It returns the index where the edited set ended up.
func (l *Load) Replace(i int, s ItemSet, merge bool) int {
	l.Sets[i] = s
	if !merge {
		return i
	}
	l.MergeDuplicates()
	for k := range l.Sets {
		if l.Sets[k].Dims == s.Dims {
			return k
		}
	}
	return i
}

// MergeDuplicates combines sets with identical dimensions, keeping the position of the first.
func (l *Load) MergeDuplicates() {
	var out []ItemSet
	for _, s := range l.Sets {
		merged := false
		for k := range out {
			if out[k].Dims == s.Dims {
				out[k].Qty += s.Qty
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, s)
		}
	}
	l.Sets = out
}
