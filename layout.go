package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	layoutMargin = 2 // left margin of every line
	layoutGap    = 4 // between the side and rear views
	emptyCell    = "·"
	dollyLabel   = '='
)

// A view is a character grid; each cell holds the index into Load.All() of
// the piece drawn there, or -1 for empty.
type view struct {
	cols, rows int
	owner      [][]int
}

// labelFor names entry i of Load.All(): A–Z, then a–z, then * once those run
// out. The dolly, when included, is always the last entry and gets =.
func labelFor(l Load, i int) rune {
	if l.DollyPacked() && i == len(l.Sets) {
		return dollyLabel
	}
	switch {
	case i < 26:
		return 'A' + rune(i)
	case i < 52:
		return 'a' + rune(i-26)
	}
	return '*'
}

func cells(dim, per float64) int {
	return max(1, int(math.Ceil(dim/per-1e-9)))
}

// fill builds a grid by sampling the centre of each cell. at maps a cell to
// the two plotted coordinates, within says whether a placement covers them,
// and depth ranks overlapping pieces so the one nearest the viewer wins.
func fill(cols, rows int, placed []Placement,
	at func(c, r int) (float64, float64),
	within func(p Placement, a, b float64) bool,
	depth func(p Placement) float64) view {

	v := view{cols: cols, rows: rows, owner: make([][]int, rows)}
	for r := range v.owner {
		v.owner[r] = make([]int, cols)
		for c := range v.owner[r] {
			v.owner[r][c] = -1
			a, b := at(c, r)
			best := math.Inf(-1)
			for _, p := range placed {
				if within(p, a, b) && depth(p) > best {
					best = depth(p)
					v.owner[r][c] = p.Set
				}
			}
		}
	}
	return v
}

// cellCenter is the middle of cell i (counting from 0 at the origin) along an
// axis of length dim, clipped to the box so a partial last cell samples inside it.
func cellCenter(i int, per, dim float64) float64 {
	lo := float64(i) * per
	hi := math.Min(lo+per, dim)
	return (lo + hi) / 2
}

func in(x, lo, size float64) bool { return x >= lo && x < lo+size }

// Y=0 is the driver's side (left-hand drive) and X=0 is the front of the cargo area.
type views struct{ top, rear, driver, passenger view }

// buildViews projects the placements onto four grids. s is inches per column;
// rows cover 2s inches because terminal cells are about twice as tall as wide.
//
//   - top: looking down with the front at the left, so the passenger side is up
//   - driver / passenger: seen from outside the car, so the front is at the
//     left of the driver view and at the right of the passenger view
//   - rear: looking forward through the hatch, driver side at the left
//
// Where pieces overlap in a view, the one nearest the viewer is drawn.
func buildViews(sp Dims, placed []Placement, s float64) views {
	rowH := 2 * s
	lenCols, widCols := cells(sp.L, s), cells(sp.W, s)
	topRows, hgtRows := cells(sp.W, rowH), cells(sp.H, rowH)
	// Row 0 is the ceiling / passenger edge, so the index from the floor / driver edge is rows-1-r.
	z := func(r int) float64 { return cellCenter(hgtRows-1-r, rowH, sp.H) }

	inXZ := func(p Placement, x, zz float64) bool { return in(x, p.X, p.Box.L) && in(zz, p.Z, p.Box.H) }
	var v views
	v.top = fill(lenCols, topRows, placed,
		func(c, r int) (float64, float64) {
			return cellCenter(c, s, sp.L), cellCenter(topRows-1-r, rowH, sp.W)
		},
		func(p Placement, x, y float64) bool { return in(x, p.X, p.Box.L) && in(y, p.Y, p.Box.W) },
		func(p Placement) float64 { return p.Z + p.Box.H }) // highest top is visible from above
	v.driver = fill(lenCols, hgtRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(c, s, sp.L), z(r) },
		inXZ,
		func(p Placement) float64 { return -p.Y }) // smallest Y is nearest the driver side
	v.passenger = fill(lenCols, hgtRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(lenCols-1-c, s, sp.L), z(r) },
		inXZ,
		func(p Placement) float64 { return p.Y + p.Box.W })
	v.rear = fill(widCols, hgtRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(c, s, sp.W), z(r) },
		func(p Placement, y, zz float64) bool { return in(y, p.Y, p.Box.W) && in(zz, p.Z, p.Box.H) },
		func(p Placement) float64 { return p.X + p.Box.L }) // nearest the hatch is visible from behind
	return v
}

var layoutPalette = []lipgloss.Color{"1", "2", "3", "4", "5", "6", "9", "10", "11", "12", "13", "14"}

func entryStyle(l Load, i int) lipgloss.Style {
	if l.DollyPacked() && i == len(l.Sets) {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(layoutPalette[i%len(layoutPalette)]).Bold(true)
}

func (v view) render(l Load) string {
	lines := make([]string, v.rows)
	for r, row := range v.owner {
		var b strings.Builder
		for _, o := range row {
			if o < 0 {
				b.WriteString(dimStyle.Render(emptyCell))
				continue
			}
			b.WriteString(entryStyle(l, o).Render(string(labelFor(l, o))))
		}
		lines[r] = b.String()
	}
	return strings.Join(lines, "\n")
}

// axis draws a labelled arrow spanning cols characters, e.g. "front ◂──────▸ hatch".
func axis(cols int, left, right string) string {
	n := cols - len(left) - len(right) - 4
	if n < 1 {
		return left + " … " + right
	}
	return strings.TrimSpace(left + " ◂" + strings.Repeat("─", n) + "▸ " + right)
}

func indent(s string) string {
	pad := strings.Repeat(" ", layoutMargin)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

type arrangement int

const (
	arrRow  arrangement = iota // top, driver, passenger and rear side by side
	arrGrid                    // top | rear above driver | passenger
)

const maxLayoutScale = 12

// estWidth is the width the views need at scale s, before anything is rendered.
func estWidth(sp Dims, s float64, arr arrangement) int {
	l, w := cells(sp.L, s), cells(sp.W, s)
	if arr == arrRow {
		return layoutMargin + 3*l + w + 3*layoutGap
	}
	return layoutMargin + max(l+w, 2*l) + layoutGap
}

// wrapText breaks s into lines of at most width characters at spaces.
func wrapText(s string, width int) []string {
	var lines []string
	var cur string
	for _, w := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func layoutHead(l Load, header string) string {
	return titleStyle.Render("deliverd — layout (approximate)") + "\n" +
		header + dimStyle.Render("  ·  Dolly: "+dollyText(l.Dolly))
}

const layoutFoot = "s search • t search time • i image • D dolly • any other key returns"

// composeViews lays the four views out in the given arrangement.
func composeViews(l Load, v views, arr arrangement) string {
	block := func(title string, vw view, caption string) string {
		return titleStyle.Render(title) + "\n" + vw.render(l) + "\n" + caption
	}
	gap := strings.Repeat(" ", layoutGap)
	top := block("Top view", v.top, axis(v.top.cols, "front", "hatch"))
	driver := block("Driver side", v.driver, axis(v.driver.cols, "front", "hatch"))
	passenger := block("Passenger side", v.passenger, axis(v.passenger.cols, "hatch", "front"))
	rear := block("Rear view", v.rear, axis(v.rear.cols, "driver", "passenger"))
	if arr == arrRow {
		return indent(lipgloss.JoinHorizontal(lipgloss.Top, top, gap, driver, gap, passenger, gap, rear))
	}
	return indent(lipgloss.JoinHorizontal(lipgloss.Top, top, gap, rear) + "\n" +
		lipgloss.JoinHorizontal(lipgloss.Top, driver, gap, passenger))
}

// composeScreen assembles the whole layout screen for one scale and arrangement.
// statusLines wraps a status message to the screen width, styled for display.
func statusLines(status string, width int) []string {
	var out []string
	for _, line := range wrapText(status, max(width, 20)) {
		out = append(out, okStyle.Render(line))
	}
	return out
}

func composeScreen(l Load, p Packing, header string, width int, s float64, arr arrangement, status []string) string {
	v := buildViews(l.Space, p.Placed, s)
	parts := []string{layoutHead(l, header), composeViews(l, v, arr)}

	scale := fmt.Sprintf("Cargo %s × %s × %s in · 1 column ≈ %.0f in, 1 row ≈ %.0f in",
		fmtNum(l.Space.L), fmtNum(l.Space.W), fmtNum(l.Space.H), s, 2*s)
	notes := "Top: passenger side up. Rear: looking forward. Sides: seen from outside the car. " +
		"Side and rear: top row is the ceiling. Left-hand drive assumed. " +
		"Heavier pieces go in first: low, forward, never on lighter ones. " +
		"Thin pieces may not show. The packing is a heuristic, not a guarantee."
	if mixedWeights(l) {
		notes += " Pieces with no weight are treated as weightless: nothing heavy rests on them."
	}
	text := []string{scale}
	text = append(text, wrapText(notes, width-layoutMargin)...)
	for i, line := range text {
		text[i] = indent(dimStyle.Render(line))
	}
	parts = append(parts, strings.Join(text, "\n"))

	if bal, ok := p.Balance(l.Space); ok {
		parts = append(parts, indent(fmt.Sprintf("Weight balance: %.0f%% in the front half, centre of mass %s in above the floor.",
			bal.FrontShare*100, fmtNum(math.Round(bal.CenterZ)))))
	}
	parts = append(parts, legendBlock(l, p, width))
	parts = append(parts, status...)
	parts = append(parts, dimStyle.Render(layoutFoot))
	return strings.Join(parts, "\n")
}

// renderLayout draws the layout screen. header describes the vehicle. It picks
// the finest scale at which the views, in a single row if that fits or else a
// 2×2 grid, fit within width and, when height is known (> 0), height. status,
// if set, is shown above the footer and counts against the height.
func renderLayout(l Load, p Packing, header string, width, height int, status string) string {
	head, foot := layoutHead(l, header), dimStyle.Render(layoutFoot)
	st := statusLines(status, width)
	stText := strings.Join(append(st, foot), "\n")
	switch {
	case len(l.All()) == 0:
		return head + "\nNothing to show yet. Add item sets first.\n" + stText + "\n"
	case p.Skipped:
		return head + fmt.Sprintf("\nQuick packing was skipped because there are more than %d pieces. Press s to run the longer search, which handles up to %d.\n", maxPackPieces, maxSearchPieces) + stText + "\n"
	case len(p.Placed) == 0:
		return head + "\nThe packing heuristic couldn't place any pieces.\n" + legendBlock(l, p, width) + "\n" + stText + "\n"
	}

	type choice struct {
		s     float64
		arr   arrangement
		lines int
	}
	var best *choice
	for s := 1.0; s <= maxLayoutScale; s++ {
		for _, arr := range []arrangement{arrRow, arrGrid} {
			if estWidth(l.Space, s, arr) > width {
				continue
			}
			screen := composeScreen(l, p, header, width, s, arr, nil)
			if lipgloss.Width(screen) > width {
				continue
			}
			n := strings.Count(screen, "\n") + 1 + len(st)
			if height <= 0 || n <= height-1 { // leave a line so nothing scrolls off the top
				return composeScreen(l, p, header, width, s, arr, st) + "\n"
			}
			if best == nil || n < best.lines {
				best = &choice{s, arr, n}
			}
		}
	}
	if best == nil { // too narrow for any scale: draw the coarsest grid anyway
		best = &choice{maxLayoutScale, arrGrid, 0}
	}
	return composeScreen(l, p, header, width, best.s, best.arr, st) + "\n"
}

// mixedWeights reports whether some entries have a weight and some do not.
func mixedWeights(l Load) bool {
	var with, without bool
	for _, s := range l.All() {
		if s.Weight > 0 {
			with = true
		} else {
			without = true
		}
	}
	return with && without
}

// legendBlock lists the entries in as many columns as fit within width.
func legendBlock(l Load, p Packing, width int) string {
	lines := legendLines(l, p)
	colW := 0
	for _, ln := range lines {
		colW = max(colW, lipgloss.Width(ln))
	}
	cols := max(1, min(len(lines), (width-layoutMargin+layoutGap)/(colW+layoutGap)))
	rows := (len(lines) + cols - 1) / cols
	var blocks []string
	for c := 0; c < cols; c++ {
		lo, hi := c*rows, min((c+1)*rows, len(lines))
		if lo >= hi {
			break
		}
		blocks = append(blocks, strings.Join(lines[lo:hi], "\n"))
		if c < cols-1 {
			blocks = append(blocks, strings.Repeat(" ", layoutGap))
		}
	}
	return indent(lipgloss.JoinHorizontal(lipgloss.Top, blocks...))
}

func legendLines(l Load, p Packing) []string {
	placed := make([]int, len(l.All()))
	for _, pl := range p.Placed {
		placed[pl.Set]++
	}
	var out []string
	for i, s := range l.All() {
		name := fmt.Sprintf("set #%d", i+1)
		if l.DollyPacked() && i == len(l.Sets) {
			name = "dolly"
		}
		tail := ""
		if s.Weight > 0 {
			tail = "   " + fmtLb(s.TotalWeight())
		}
		line := fmt.Sprintf("%s  %-7s %2d × %s×%s×%s in   placed %d of %d%s",
			entryStyle(l, i).Render(string(labelFor(l, i))), name,
			s.Qty, fmtNum(s.L), fmtNum(s.W), fmtNum(s.H), placed[i], s.Qty, tail)
		if placed[i] < s.Qty {
			line = badStyle.Render(fmt.Sprintf("%c  %-7s %2d × %s×%s×%s in   placed %d of %d — %d not placed%s",
				labelFor(l, i), name, s.Qty, fmtNum(s.L), fmtNum(s.W), fmtNum(s.H), placed[i], s.Qty, s.Qty-placed[i], tail))
		}
		out = append(out, line)
	}
	return out
}
