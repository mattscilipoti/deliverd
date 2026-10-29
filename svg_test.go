package main

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func svgSample() (Load, Packing) {
	l := Load{
		Space: Vehicles[0].SeatsDown.Dims, PayloadLimit: 915, Occupants: 175, Dolly: DollyWithItems,
		Sets: []ItemSet{
			{Qty: 3, Weight: 40, Dims: Dims{30, 20, 15}},
			{Qty: 4, Weight: 6, Dims: Dims{12, 12, 12}},
		},
	}
	return l, l.Pack()
}

func TestSVGIsWellFormed(t *testing.T) {
	l, p := svgSample()
	dec := xml.NewDecoder(strings.NewReader(renderSVG(l, p, "Test <car> & co", "stamp")))
	var elems int
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("not well-formed XML: %v", err)
		}
		if _, ok := tok.(xml.StartElement); ok {
			elems++
		}
	}
	if elems < 20 {
		t.Errorf("only %d elements; expected a drawing", elems)
	}
}

func TestSVGEscapesText(t *testing.T) {
	l, p := svgSample()
	out := renderSVG(l, p, `Car <b>"x"</b> & Co`, "s")
	if strings.Contains(out, "<b>") || !strings.Contains(out, "&lt;b&gt;") || !strings.Contains(out, "&amp; Co") {
		t.Errorf("header text should be escaped:\n%s", out[:min(len(out), 600)])
	}
}

func TestSVGContent(t *testing.T) {
	l, p := svgSample()
	out := renderSVG(l, p, "2013 Toyota Prius v, rear seats down", "Exported now")
	for _, want := range []string{
		"2013 Toyota Prius v, rear seats down", "loading layout",
		"3D view", "Top view", "Driver side", "Passenger side", "Rear view",
		"Set #1", "Set #2", "Dolly", "placed 3 of 3", "Exported now",
		"cargo weight", "front half",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("svg missing %q", want)
		}
	}
	// Each placed piece is outlined in every flat view, and drawn as three faces in 3D.
	placed := len(p.Placed)
	rects := strings.Count(out, `stroke-width="1.6"`)
	if rects < 4*placed {
		t.Errorf("%d outlined rects for %d placed pieces; want at least one per piece in each of the four views", rects, placed)
	}
	if polys := strings.Count(out, "<polygon"); polys < 3*placed {
		t.Errorf("%d polygons; want 3 faces per piece in the 3D view", polys)
	}
}

func TestSVGUsesOneColourPerSet(t *testing.T) {
	l, p := svgSample()
	out := renderSVG(l, p, "", "")
	for i := range l.All() {
		if c := svgColor(l, i); !strings.Contains(out, `stroke="`+c+`"`) {
			t.Errorf("set %d's outline colour %s is missing", i, c)
		}
	}
	if svgColor(l, 0) == svgColor(l, 1) {
		t.Error("different sets must get different colours")
	}
	if svgColor(l, len(l.Sets)) != svgDollyColor {
		t.Error("the dolly should be grey")
	}
}

func TestSVGViewMappings(t *testing.T) {
	sp := Dims{L: 100, W: 40, H: 30}
	pl := Placement{X: 10, Y: 0, Z: 5, Box: Dims{20, 10, 8}} // driver's side, near the front
	check := func(name string, got [4]float64, want [4]float64) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	rect := func(f func(Placement, Dims) (float64, float64, float64, float64)) [4]float64 {
		a, b, c, d := f(pl, sp)
		return [4]float64{a, b, c, d}
	}
	// Top: front at the left, passenger side up, so the driver side is at the bottom.
	check("top", rect(topRect), [4]float64{10, 30, 20, 10})
	// Driver side: front at the left, floor at the bottom.
	check("driver", rect(driverRect), [4]float64{10, 17, 20, 8})
	// Passenger side is the mirror image of the driver side.
	check("passenger", rect(passengerRect), [4]float64{70, 17, 20, 8})
	// Rear: looking forward, driver at the left.
	check("rear", rect(rearRect), [4]float64{0, 17, 10, 8})
}

func TestSVGRearAndPassengerShowNearestPiece(t *testing.T) {
	l := Load{Space: Dims{100, 40, 30}, Sets: []ItemSet{{Qty: 1, Dims: Dims{10, 10, 10}}, {Qty: 1, Dims: Dims{10, 10, 10}}}}
	p := Packing{Placed: []Placement{
		{Set: 0, X: 0, Y: 0, Box: Dims{10, 10, 10}},   // front, driver side
		{Set: 1, X: 50, Y: 30, Box: Dims{10, 10, 10}}, // hatch side, passenger side
	}, Unplaced: make([]int, 2), Total: 2}
	out := renderSVG(l, p, "", "")
	for i := range l.Sets {
		if !strings.Contains(out, `stroke="`+svgColor(l, i)+`"`) {
			t.Errorf("set %d is not drawn", i)
		}
	}
}

func TestSVGMarksUnplacedPieces(t *testing.T) {
	l := Load{Space: Dims{10, 10, 10}, Sets: []ItemSet{{Qty: 3, Dims: Dims{6, 6, 6}}}}
	out := renderSVG(l, l.Pack(), "", "")
	if !strings.Contains(out, "2 not placed") || !strings.Contains(out, `fill="#c00"`) {
		t.Error("pieces that did not fit should be called out in red in the legend")
	}
}

func TestIsoPointDirections(t *testing.T) {
	x0, y0 := isoPoint(0, 0, 0)
	// More length (towards the hatch) goes down and to the left.
	if x, y := isoPoint(10, 0, 0); !(x < x0 && y > y0) {
		t.Errorf("+x should go down-left, got (%v, %v) from (%v, %v)", x, y, x0, y0)
	}
	// More width (towards the passenger side) goes down and to the right.
	if x, y := isoPoint(0, 10, 0); !(x > x0 && y > y0) {
		t.Errorf("+y should go down-right, got (%v, %v)", x, y)
	}
	if _, y := isoPoint(0, 0, 10); !(y < y0) {
		t.Error("+z should go up")
	}
}

func TestMixWhite(t *testing.T) {
	if got := mixWhite("#000000", 0.5); got != "#808080" {
		t.Errorf("mixWhite = %s, want #808080", got)
	}
	if mixWhite("#123456", 0) != "#123456" || mixWhite("#123456", 1) != "#ffffff" {
		t.Error("mixWhite should keep the colour at 0 and reach white at 1")
	}
	if mixWhite("nonsense", 0.5) != "nonsense" {
		t.Error("an unparseable colour should pass through")
	}
}
