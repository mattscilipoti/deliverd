package main

import (
	"fmt"
	"sort"
)

const cubicInchesPerCubicFoot = 1728

type Dims struct {
	L float64 `json:"l"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

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
	Qty    int     `json:"qty"`
	Weight float64 `json:"weight,omitempty"` // lb per item; 0 means not entered
	Dims
}

func (s ItemSet) TotalVolume() float64 { return float64(s.Qty) * s.Volume() }

func (s ItemSet) TotalWeight() float64 { return float64(s.Qty) * s.Weight }

// sameKind reports whether two sets are interchangeable and can be merged:
// same dimensions and same per-item weight.
func (s ItemSet) sameKind(o ItemSet) bool { return s.Dims == o.Dims && s.Weight == o.Weight }

// Dolly is optional equipment: 48 × 24 × 6 in when it rides with the items.
var Dolly = ItemSet{Qty: 1, Dims: Dims{L: 48, W: 24, H: 6}}

// DollyMode says where the dolly goes.
type DollyMode int

const (
	DollyOff DollyMode = iota
	// DollyBehindSeats stows it lengthwise behind the seats, where it takes
	// almost no cargo space, so it is not counted or packed.
	DollyBehindSeats
	// DollyWithItems carries it in the cargo area, packed like an item and
	// free to rotate to fit around the products.
	DollyWithItems
)

// Next cycles off, behind seats, with items.
func (d DollyMode) Next() DollyMode { return (d + 1) % 3 }

// ParseDollyMode reads the -dolly flag: off, seats or items.
func ParseDollyMode(s string) (DollyMode, error) {
	switch s {
	case "off", "":
		return DollyOff, nil
	case "seats":
		return DollyBehindSeats, nil
	case "items":
		return DollyWithItems, nil
	}
	return DollyOff, fmt.Errorf(`-dolly must be "off", "seats" or "items"`)
}

// dollyText describes the mode for the main and layout screens.
func dollyText(d DollyMode) string {
	switch d {
	case DollyBehindSeats:
		return "behind seats (not counted)"
	case DollyWithItems:
		return fmt.Sprintf("with items (%s × %s × %s in, packed to fit)", fmtNum(Dolly.L), fmtNum(Dolly.W), fmtNum(Dolly.H))
	}
	return "off"
}

type Load struct {
	Space Dims
	Sets  []ItemSet
	Dolly DollyMode

	// PayloadLimit is the vehicle's combined limit for occupants and cargo in
	// lb; 0 means unknown, so no weight check is made. Occupants is what the
	// people in the car weigh, which comes out of that limit.
	PayloadLimit float64
	Occupants    float64
}

// DollyPacked reports whether the dolly is counted and packed with the items.
func (l Load) DollyPacked() bool { return l.Dolly == DollyWithItems }

// All is Sets followed by the dolly when it rides with the items. The dolly is
// always last, so an index equal to len(Sets) refers to it.
func (l Load) All() []ItemSet {
	if !l.DollyPacked() {
		return l.Sets
	}
	return append(append([]ItemSet(nil), l.Sets...), Dolly)
}

func (l Load) Capacity() float64 { return l.Space.Volume() }

func (l Load) Used() float64 {
	var total float64
	for _, s := range l.All() {
		total += s.TotalVolume()
	}
	return total
}

// Available is negative when the load exceeds the cargo volume.
func (l Load) Available() float64 { return l.Capacity() - l.Used() }

// Oversized returns the indexes (into All) of sets whose items cannot fit in the space in any orientation.
func (l Load) Oversized() []int {
	var idx []int
	for i, s := range l.All() {
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
			if l.Sets[i].sameKind(s) {
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
		if l.Sets[k].sameKind(s) {
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
			if out[k].sameKind(s) {
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

// Weight is the total entered weight of the item sets in lb. The dolly is not
// included; count it in Occupants.
func (l Load) Weight() float64 {
	var total float64
	for _, s := range l.Sets {
		total += s.TotalWeight()
	}
	return total
}

// Unweighted counts the item sets with no weight entered. While it is above
// zero, Weight is a minimum.
func (l Load) Unweighted() int {
	var n int
	for _, s := range l.Sets {
		if s.Weight == 0 {
			n++
		}
	}
	return n
}

// WeightAllowance is what the cargo may weigh once the occupants are
// subtracted from the limit. It can be negative.
func (l Load) WeightAllowance() float64 { return l.PayloadLimit - l.Occupants }

// WeightRemaining is the allowance left after the entered cargo weight.
func (l Load) WeightRemaining() float64 { return l.WeightAllowance() - l.Weight() }

// HasWeightLimit reports whether a payload limit is known.
func (l Load) HasWeightLimit() bool { return l.PayloadLimit > 0 }

// WeightOK is true when the entered weight is within the allowance, or when no
// limit is known.
func (l Load) WeightOK() bool { return !l.HasWeightLimit() || l.WeightRemaining() >= 0 }
