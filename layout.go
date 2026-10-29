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
// the side and rear views fit side by side in width.
func chooseScale(sp Dims, width int) float64 {
	for s := 1.0; s < 1000; s++ {
		if cells(sp.L, s)+cells(sp.W, s)+layoutMargin+layoutGap <= width {
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

// buildViews projects the placements onto three grids. s is inches per column;
// rows cover 2s inches because terminal cells are about twice as tall as wide.
func buildViews(sp Dims, placed []Placement, s float64) (top, side, rear view) {
	rowH := 2 * s
	lenCols, widCols := cells(sp.L, s), cells(sp.W, s)
	topRows, hgtRows := cells(sp.W, rowH), cells(sp.H, rowH)
	// Row 0 is the ceiling, so the cell index up from the floor is hgtRows-1-r.
	z := func(r int) float64 { return cellCenter(hgtRows-1-r, rowH, sp.H) }

	top = fill(lenCols, topRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(c, s, sp.L), cellCenter(r, rowH, sp.W) },
		func(p Placement, x, y float64) bool { return in(x, p.X, p.Box.L) && in(y, p.Y, p.Box.W) },
		func(p Placement) float64 { return p.Z + p.Box.H }) // highest top is visible from above
	side = fill(lenCols, hgtRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(c, s, sp.L), z(r) },
		func(p Placement, x, zz float64) bool { return in(x, p.X, p.Box.L) && in(zz, p.Z, p.Box.H) },
		func(p Placement) float64 { return p.Y + p.Box.W })
	rear = fill(widCols, hgtRows, placed,
		func(c, r int) (float64, float64) { return cellCenter(c, s, sp.W), z(r) },
		func(p Placement, y, zz float64) bool { return in(y, p.Y, p.Box.W) && in(zz, p.Z, p.Box.H) },
		func(p Placement) float64 { return p.X + p.Box.L }) // nearest the hatch is visible from behind
	return top, side, rear
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
	b.WriteString(titleStyle.Render("deliverd — layout (approximate)") + "\n" + header + "\n\n")

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
		top, side, rear := buildViews(l.Space, p.Placed, s)

		b.WriteString(titleStyle.Render("Top view") + dimStyle.Render("  looking down") + "\n")
		b.WriteString(indent(top.render(l)+"\n"+axis(top.cols, "front", "hatch")) + "\n\n")

		sideBlock := titleStyle.Render("Side view") + "\n" + indent(side.render(l)+"\n"+axis(side.cols, "front", "hatch"))
		rearBlock := titleStyle.Render("Rear view") + "\n" + rear.render(l) + "\n" + axis(rear.cols, "", "width")
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, sideBlock, strings.Repeat(" ", layoutGap), rearBlock) + "\n\n")

		b.WriteString(dimStyle.Render(fmt.Sprintf(
			"Cargo %s × %s × %s in. One column ≈ %.0f in, one row ≈ %.0f in. Side and rear: top row is the ceiling, bottom is the floor.",
			fmtNum(l.Space.L), fmtNum(l.Space.W), fmtNum(l.Space.H), s, 2*s)) + "\n")
		b.WriteString(dimStyle.Render("Pieces thinner than a cell may not show. The packing is a heuristic; it ignores weight.") + "\n\n")
		b.WriteString(legend(l, p))
	}
	b.WriteString("\n" + dimStyle.Render("Press any key to return.") + "\n")
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
