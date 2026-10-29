package main

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Layout of the exported image, in pixels.
const (
	svgWidth  = 1400
	svgMargin = 30
	svgGap    = 30
)

// svgPalette gives each set its own outline colour; the dolly is grey.
var svgPalette = []string{
	"#e6194b", "#3cb44b", "#4363d8", "#f58231", "#911eb4", "#008080",
	"#9a6324", "#800000", "#808000", "#000075", "#f032e6", "#469990",
}

const svgDollyColor = "#555555"

func svgColor(l Load, i int) string {
	if l.DollyPacked() && i == len(l.Sets) {
		return svgDollyColor
	}
	return svgPalette[i%len(svgPalette)]
}

// mixWhite blends a #rrggbb colour toward white; f=0 keeps it and f=1 is white.
func mixWhite(hex string, f float64) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		return hex
	}
	mix := func(c uint64) uint64 { return uint64(math.Round(float64(c) + (255-float64(c))*f)) }
	return fmt.Sprintf("#%02x%02x%02x", mix(v>>16&0xff), mix(v>>8&0xff), mix(v&0xff))
}

func f1(v float64) string { return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) }

// svgBuilder accumulates elements and tracks the page height.
type svgBuilder struct{ b strings.Builder }

func (s *svgBuilder) add(format string, args ...any) { fmt.Fprintf(&s.b, format+"\n", args...) }

func (s *svgBuilder) text(x, y float64, size int, weight, fill, anchor, body string) {
	s.add(`<text x="%s" y="%s" font-size="%d" font-weight="%s" fill="%s" text-anchor="%s">%s</text>`,
		f1(x), f1(y), size, weight, fill, anchor, html.EscapeString(body))
}

// pane is a rectangle on the page that a view is drawn into.
type pane struct{ x, y, k float64 } // origin in px, and px per inch

// rect maps a box to a rectangle given the two plotted extents.
func (p pane) rect(x0, y0, w, h float64) (float64, float64, float64, float64) {
	return p.x + x0*p.k, p.y + y0*p.k, w * p.k, h * p.k
}

// Placement-to-pane mappings, matching the terminal views: the top view has
// the front at the left and the passenger side up; the driver side view has
// the front at the left; the passenger side view is its mirror; the rear view
// looks forward with the driver at the left.
func topRect(pl Placement, sp Dims) (x0, y0, w, h float64) {
	return pl.X, sp.W - (pl.Y + pl.Box.W), pl.Box.L, pl.Box.W
}
func driverRect(pl Placement, sp Dims) (x0, y0, w, h float64) {
	return pl.X, sp.H - (pl.Z + pl.Box.H), pl.Box.L, pl.Box.H
}
func passengerRect(pl Placement, sp Dims) (x0, y0, w, h float64) {
	return sp.L - (pl.X + pl.Box.L), sp.H - (pl.Z + pl.Box.H), pl.Box.L, pl.Box.H
}
func rearRect(pl Placement, sp Dims) (x0, y0, w, h float64) {
	return pl.Y, sp.H - (pl.Z + pl.Box.H), pl.Box.W, pl.Box.H
}

// drawFlat draws one orthographic view. Pieces are painted farthest first so
// the nearest piece ends up on top. depth ranks pieces; larger is nearer.
func (s *svgBuilder) drawFlat(l Load, p Packing, title string, pn pane, w, h float64,
	rect func(Placement, Dims) (float64, float64, float64, float64), depth func(Placement) float64, caption string) {

	sp := l.Space
	s.text(pn.x, pn.y-8, 15, "bold", "#222", "start", title)
	x, y, cw, ch := pn.rect(0, 0, w, h)
	s.add(`<rect x="%s" y="%s" width="%s" height="%s" fill="#fafafa" stroke="#888" stroke-dasharray="6 4"/>`, f1(x), f1(y), f1(cw), f1(ch))

	order := append([]Placement(nil), p.Placed...)
	sort.SliceStable(order, func(a, b int) bool { return depth(order[a]) < depth(order[b]) })
	for _, pl := range order {
		x0, y0, bw, bh := rect(pl, sp)
		rx, ry, rw, rh := pn.rect(x0, y0, bw, bh)
		col := svgColor(l, pl.Set)
		s.add(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.6"/>`,
			f1(rx), f1(ry), f1(rw), f1(rh), mixWhite(col, 0.78), col)
		if rw >= 14 && rh >= 14 {
			s.text(rx+rw/2, ry+rh/2+5, int(math.Min(15, math.Min(rw, rh)*0.7)), "bold", col, "middle", string(labelFor(l, pl.Set)))
		}
	}
	s.text(pn.x+cw/2, pn.y+ch+18, 12, "normal", "#666", "middle", caption)
}

// isoPoint projects a point for a viewer above the hatch on the passenger
// side: x (length) runs down-left, y (width) runs down-right and z runs up.
func isoPoint(x, y, z float64) (float64, float64) {
	return (y - x) * math.Cos(math.Pi/6), (x+y)*0.5 - z
}

func (s *svgBuilder) drawIso(l Load, p Packing, x, y, w, h float64) {
	sp := l.Space
	s.text(x, y-8, 15, "bold", "#222", "start", "3D view (from the hatch, passenger side)")

	// Scale and offset so the projected cargo box fills the pane.
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range [][3]float64{{0, 0, 0}, {sp.L, 0, 0}, {0, sp.W, 0}, {sp.L, sp.W, 0}, {0, 0, sp.H}, {sp.L, 0, sp.H}, {0, sp.W, sp.H}, {sp.L, sp.W, sp.H}} {
		px, py := isoPoint(c[0], c[1], c[2])
		minX, maxX = math.Min(minX, px), math.Max(maxX, px)
		minY, maxY = math.Min(minY, py), math.Max(maxY, py)
	}
	k := math.Min(w/(maxX-minX), h/(maxY-minY))
	pt := func(px, py, pz float64) string {
		sx, sy := isoPoint(px, py, pz)
		return f1(x+(sx-minX)*k+(w-(maxX-minX)*k)/2) + "," + f1(y+(sy-minY)*k)
	}
	poly := func(fill, stroke string, sw float64, pts ...string) {
		s.add(`<polygon points="%s" fill="%s" stroke="%s" stroke-width="%s" stroke-linejoin="round"/>`,
			strings.Join(pts, " "), fill, stroke, f1(sw))
	}

	// The two back walls and the floor of the cargo area.
	poly("#f1f1f1", "#999", 1, pt(0, 0, 0), pt(sp.L, 0, 0), pt(sp.L, 0, sp.H), pt(0, 0, sp.H))
	poly("#e8e8e8", "#999", 1, pt(0, 0, 0), pt(0, sp.W, 0), pt(0, sp.W, sp.H), pt(0, 0, sp.H))
	poly("#fafafa", "#999", 1, pt(0, 0, 0), pt(sp.L, 0, 0), pt(sp.L, sp.W, 0), pt(0, sp.W, 0))

	order := append([]Placement(nil), p.Placed...)
	centre := func(pl Placement) float64 {
		return (pl.X + pl.Box.L/2) + (pl.Y + pl.Box.W/2) + (pl.Z + pl.Box.H/2)
	}
	sort.SliceStable(order, func(a, b int) bool { return centre(order[a]) < centre(order[b]) })
	for _, pl := range order {
		x0, y0, z0 := pl.X, pl.Y, pl.Z
		x1, y1, z1 := x0+pl.Box.L, y0+pl.Box.W, z0+pl.Box.H
		col := svgColor(l, pl.Set)
		poly(mixWhite(col, 0.78), col, 1.4, pt(x0, y0, z1), pt(x1, y0, z1), pt(x1, y1, z1), pt(x0, y1, z1)) // top
		poly(mixWhite(col, 0.55), col, 1.4, pt(x1, y0, z0), pt(x1, y1, z0), pt(x1, y1, z1), pt(x1, y0, z1)) // hatch face
		poly(mixWhite(col, 0.35), col, 1.4, pt(x0, y1, z0), pt(x1, y1, z0), pt(x1, y1, z1), pt(x0, y1, z1)) // passenger face
	}
	s.text(x, y+h+16, 12, "normal", "#666", "start", "Front of the car is at the back right; the hatch faces you on the left.")
}

func (s *svgBuilder) drawLegend(l Load, p Packing, x, y float64) float64 {
	placed := make([]int, len(l.All()))
	for _, pl := range p.Placed {
		placed[pl.Set]++
	}
	s.text(x, y, 15, "bold", "#222", "start", "Legend")
	y += 12
	for i, it := range l.All() {
		y += 24
		col := svgColor(l, i)
		s.add(`<rect x="%s" y="%s" width="16" height="16" fill="%s" stroke="%s" stroke-width="1.6"/>`, f1(x), f1(y-13), mixWhite(col, 0.78), col)
		name := fmt.Sprintf("Set #%d", i+1)
		if l.DollyPacked() && i == len(l.Sets) {
			name = "Dolly"
		}
		line := fmt.Sprintf("%c  %s: %d × %s×%s×%s in", labelFor(l, i), name, it.Qty, fmtNum(it.L), fmtNum(it.W), fmtNum(it.H))
		if it.Weight > 0 {
			line += ", " + fmtLb(it.TotalWeight())
		}
		line += fmt.Sprintf(", placed %d of %d", placed[i], it.Qty)
		fill := "#222"
		if placed[i] < it.Qty {
			fill = "#c00"
			line += fmt.Sprintf(" (%d not placed)", it.Qty-placed[i])
		}
		s.text(x+26, y, 14, "normal", fill, "start", line)
	}
	return y
}

// renderSVG draws the load: a 3D view and the legend on top, then the top,
// driver side, passenger side and rear views to scale. header describes the
// vehicle and stamp the build and time of export.
func renderSVG(l Load, p Packing, header, stamp string) string {
	sp := l.Space
	innerW := float64(svgWidth - 2*svgMargin)
	k := (innerW - 3*svgGap) / (3*sp.L + sp.W) // px per inch for the four flat views

	var s svgBuilder
	y := float64(svgMargin) + 20
	s.text(svgMargin, y, 24, "bold", "#111", "start", "deliverd — loading layout")
	y += 26
	s.text(svgMargin, y, 15, "normal", "#333", "start", header)
	y += 22
	info := "Dolly: " + dollyText(l.Dolly)
	if l.HasWeightLimit() {
		info += fmt.Sprintf(" · cargo weight %s of %s allowed", fmtLb(l.Weight()), fmtLb(l.WeightAllowance()))
	}
	if bal, ok := p.Balance(sp); ok {
		info += fmt.Sprintf(" · %.0f%% of weight in the front half, centre of mass %s in up", bal.FrontShare*100, fmtNum(math.Round(bal.CenterZ)))
	}
	s.text(svgMargin, y, 14, "normal", "#555", "start", info)

	// Row 1: the 3D view and the legend.
	isoY := y + 50
	isoW, isoH := 700.0, 380.0
	s.drawIso(l, p, svgMargin, isoY, isoW, isoH)
	legendBottom := s.drawLegend(l, p, svgMargin+isoW+40, isoY+10)

	// Row 2: the four flat views, to scale and aligned along the top.
	rowY := math.Max(isoY+isoH+50, legendBottom+40)
	x := float64(svgMargin)
	ly := rowY + 24
	s.drawFlat(l, p, "Top view (passenger side up)", pane{x, ly, k}, sp.L, sp.W, topRect,
		func(pl Placement) float64 { return pl.Z + pl.Box.H }, "front ◂ → ▸ hatch")
	x += sp.L*k + svgGap
	s.drawFlat(l, p, "Driver side", pane{x, ly, k}, sp.L, sp.H, driverRect,
		func(pl Placement) float64 { return -pl.Y }, "front ◂ → ▸ hatch")
	x += sp.L*k + svgGap
	s.drawFlat(l, p, "Passenger side", pane{x, ly, k}, sp.L, sp.H, passengerRect,
		func(pl Placement) float64 { return pl.Y + pl.Box.W }, "hatch ◂ → ▸ front")
	x += sp.L*k + svgGap
	s.drawFlat(l, p, "Rear view (looking forward)", pane{x, ly, k}, sp.W, sp.H, rearRect,
		func(pl Placement) float64 { return pl.X + pl.Box.L }, "driver ◂ → ▸ passenger")

	bottom := ly + math.Max(sp.W, sp.H)*k + 40
	s.text(svgMargin, bottom, 12, "normal", "#777", "start",
		fmt.Sprintf("Cargo %s × %s × %s in. The packing is a heuristic, not a guarantee. Left-hand drive assumed. %s",
			fmtNum(sp.L), fmtNum(sp.W), fmtNum(sp.H), stamp))

	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%s" viewBox="0 0 %d %s" font-family="system-ui, -apple-system, Segoe UI, Helvetica, Arial, sans-serif">`+"\n",
		svgWidth, f1(bottom+svgMargin), svgWidth, f1(bottom+svgMargin))
	out.WriteString(`<title>deliverd loading layout</title>` + "\n")
	fmt.Fprintf(&out, `<rect width="100%%" height="100%%" fill="#ffffff"/>`+"\n")
	out.WriteString(s.b.String())
	out.WriteString("</svg>\n")
	return out.String()
}
