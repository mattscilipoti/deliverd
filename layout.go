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
	if l.Dolly && i == len(l.Sets) {
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

// chooseScale returns the smallest whole number of inches per column such that
// the widest row of the layout fits in width. The rows are top + rear, and
// driver side + passenger side; the latter is usually the wider.
func chooseScale(sp Dims, width int) float64 {
	for s := 1.0; s < 1000; s++ {
		l, w := cells(sp.L, s), cells(sp.W, s)
		if max(l+w, 2*l)+layoutMargin+layoutGap <= width {
			return s
		}
	}
	return 1000
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
	if l.Dolly && i == len(l.Sets) {
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

// renderLayout draws the whole layout screen. header describes the vehicle.
func renderLayout(l Load, p Packing, header string, width int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("deliverd — layout (approximate)") + "\n" + header + "\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Dolly (%s × %s × %s in): %s",
		fmtNum(Dolly.L), fmtNum(Dolly.W), fmtNum(Dolly.H), onOff(l.Dolly))) + "\n\n")

	entries := l.All()
	switch {
	case len(entries) == 0:
		b.WriteString("Nothing to show yet. Add item sets first.\n")
	case p.Skipped:
		fmt.Fprintf(&b, "Packing was skipped because there are more than %d pieces, so there is no layout to draw.\n", maxPackPieces)
	case len(p.Placed) == 0:
		b.WriteString("The packing heuristic couldn't place any pieces.\n")
		b.WriteString(legend(l, p))
	default:
		s := chooseScale(l.Space, width)
		v := buildViews(l.Space, p.Placed, s)
		block := func(title string, vw view, caption string) string {
			return titleStyle.Render(title) + "\n" + indent(vw.render(l)+"\n"+caption)
		}
		gap := strings.Repeat(" ", layoutGap)
		row1 := lipgloss.JoinHorizontal(lipgloss.Top,
			block("Top view", v.top, axis(v.top.cols, "front", "hatch")), gap,
			block("Rear view", v.rear, axis(v.rear.cols, "driver", "passenger")))
		row2 := lipgloss.JoinHorizontal(lipgloss.Top,
			block("Driver side", v.driver, axis(v.driver.cols, "front", "hatch")), gap,
			block("Passenger side", v.passenger, axis(v.passenger.cols, "hatch", "front")))
		b.WriteString(row1 + "\n\n" + row2 + "\n\n")

		b.WriteString(dimStyle.Render(fmt.Sprintf("Cargo %s × %s × %s in. One column ≈ %.0f in, one row ≈ %.0f in.",
			fmtNum(l.Space.L), fmtNum(l.Space.W), fmtNum(l.Space.H), s, 2*s)) + "\n")
		for _, note := range []string{
			"Top: passenger side up. Rear: looking forward. Sides: seen from outside.",
			"Side and rear: top row is the ceiling. Left-hand drive assumed.",
			"Thin pieces may not show. The packing is a heuristic and ignores weight.",
		} {
			b.WriteString(dimStyle.Render(note) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(legend(l, p))
	}
	b.WriteString("\n" + dimStyle.Render("D dolly on/off • any other key returns") + "\n")
	return b.String()
}

func legend(l Load, p Packing) string {
	placed := make([]int, len(l.All()))
	for _, pl := range p.Placed {
		placed[pl.Set]++
	}
	var b strings.Builder
	for i, s := range l.All() {
		name := fmt.Sprintf("set #%d", i+1)
		if l.Dolly && i == len(l.Sets) {
			name = "dolly"
		}
		line := fmt.Sprintf("%s  %-7s %2d × %s×%s×%s in   placed %d of %d",
			entryStyle(l, i).Render(string(labelFor(l, i))), name,
			s.Qty, fmtNum(s.L), fmtNum(s.W), fmtNum(s.H), placed[i], s.Qty)
		if placed[i] < s.Qty {
			line = badStyle.Render(fmt.Sprintf("%c  %-7s %2d × %s×%s×%s in   placed %d of %d — %d not placed",
				labelFor(l, i), name, s.Qty, fmtNum(s.L), fmtNum(s.W), fmtNum(s.H), placed[i], s.Qty, s.Qty-placed[i]))
		}
		b.WriteString(indent(line) + "\n")
	}
	return b.String()
}
