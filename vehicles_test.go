package main

import (
	"math"
	"strings"
	"testing"
)

// The estimated seats-down box should land close to the listed volume, so the
// default can't silently drift from the figure Toyota publishes.
func TestPresetsMatchListedVolume(t *testing.T) {
	for _, v := range Vehicles {
		for _, down := range []bool{false, true} {
			c := v.Cargo(down)
			boxFt3 := c.Volume() / cubicInchesPerCubicFoot
			listed := v.Listed(down)
			if c.Estimated && math.Abs(boxFt3-listed)/listed > 0.02 {
				t.Errorf("%s %s: estimated box %.1f ft³ is not within 2%% of listed %.1f ft³",
					v.Name, seatsLabel(down), boxFt3, listed)
			}
			if boxFt3 > listed*1.02 {
				t.Errorf("%s %s: box %.1f ft³ exceeds listed %.1f ft³", v.Name, seatsLabel(down), boxFt3, listed)
			}
		}
	}
}

func TestDefaultIsPriusVSeatsDown(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true})
	want := Vehicles[0].SeatsDown.Dims
	if m.load.Space != want || m.custom {
		t.Errorf("Space=%v custom=%v, want %v preset", m.load.Space, m.custom, want)
	}
	if !strings.Contains(Vehicles[0].Name, "2013 Toyota Prius v") {
		t.Errorf("default vehicle = %q", Vehicles[0].Name)
	}
}

func TestToggleSeats(t *testing.T) {
	m := press(newTestModel(true), "esc", "r")
	if m.seatsDown || m.load.Space != Vehicles[0].SeatsUp.Dims {
		t.Errorf("seatsDown=%v Space=%v, want seats-up preset", m.seatsDown, m.load.Space)
	}
	m = press(m, "r")
	if !m.seatsDown || m.load.Space != Vehicles[0].SeatsDown.Dims {
		t.Errorf("seatsDown=%v Space=%v, want seats-down preset", m.seatsDown, m.load.Space)
	}
}

func TestOverrideAndReset(t *testing.T) {
	m := newModel(options{seatsDown: true, merge: true, override: Dims{L: 50}})
	if !m.custom || m.load.Space.L != 50 || m.load.Space.W != Vehicles[0].SeatsDown.W {
		t.Errorf("Space=%v custom=%v, want only length overridden", m.load.Space, m.custom)
	}
	// Choosing a seat configuration goes back to the vehicle preset.
	m = press(m, "esc", "r", "r")
	if m.custom || m.load.Space != Vehicles[0].SeatsDown.Dims {
		t.Errorf("Space=%v custom=%v, want preset restored", m.load.Space, m.custom)
	}
}

func TestViewNamesVehicleAndListedVolume(t *testing.T) {
	v := newTestModel(true).View()
	for _, want := range []string{"2013 Toyota Prius v", "rear seats down", "estimated", "67.3 ft³"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
