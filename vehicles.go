package main

import "fmt"

// Cargo is a usable cargo box for one seat configuration.
type Cargo struct {
	Dims
	Estimated bool   // true when derived rather than measured
	Note      string // where the dimensions come from
}

type Vehicle struct {
	Name      string
	SeatsUp   Cargo
	SeatsDown Cargo
	// Manufacturer-listed cargo volumes in ft³, shown so the box can be sanity-checked.
	ListedUpFt3, ListedDownFt3 float64
	// PayloadLb is the combined weight limit for occupants and cargo, in lb.
	PayloadLb   float64
	PayloadNote string
}

// DefaultOccupantsLb is the starting occupant weight: one driver.
const DefaultOccupantsLb = 175

func (v Vehicle) Cargo(seatsDown bool) Cargo {
	if seatsDown {
		return v.SeatsDown
	}
	return v.SeatsUp
}

func (v Vehicle) Listed(seatsDown bool) float64 {
	if seatsDown {
		return v.ListedDownFt3
	}
	return v.ListedUpFt3
}

const priusVMeasured = "width and height owner-measured on a 2016 Prius v (same generation), PriusChat"

// Vehicles is the selectable list; the first entry is the default.
var Vehicles = []Vehicle{
	{
		Name: "2013 Toyota Prius v",
		SeatsUp: Cargo{
			Dims: Dims{L: 38, W: 42, H: 31},
			Note: "owner-measured on a 2016 Prius v (same generation), PriusChat; rear seats slid fully back",
		},
		// Toyota lists 67.3 ft³ but no dimensions. Width and height are measured; the length is
		// solved so the box matches the listed volume: 67.3 * 1728 / (42 * 31) = 89.3, rounded down.
		SeatsDown: Cargo{
			Dims:      Dims{L: 89, W: 42, H: 31},
			Estimated: true,
			Note:      priusVMeasured + "; length estimated from the listed volume",
		},
		ListedUpFt3:   34.3,
		ListedDownFt3: 67.3,
		PayloadLb:     915,
		PayloadNote:   "forum-reported owner's manual figure, unverified; check the sticker in your driver's door jamb",
	},
}

func seatsLabel(down bool) string {
	if down {
		return "rear seats down"
	}
	return "rear seats up"
}

func (v Vehicle) describe(seatsDown bool) string {
	c := v.Cargo(seatsDown)
	kind := "measured"
	if c.Estimated {
		kind = "estimated"
	}
	return fmt.Sprintf("Toyota lists %.1f ft³ for this car (%s). Box is %s: %s.",
		v.Listed(seatsDown), seatsLabel(seatsDown), kind, c.Note)
}
