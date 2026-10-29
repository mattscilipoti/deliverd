package main

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// searchBudgets are the times the t key cycles through on the layout screen.
var searchBudgets = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}

// searchMsg carries a better layout, or the final result, from the search goroutine.
type searchMsg struct {
	pack  Packing
	iters int
	final bool
	stats SearchStats
}

type searchTickMsg struct{}

// searchState tracks a search running in the background.
type searchState struct {
	running bool
	stopped bool // the user asked it to stop
	cancel  context.CancelFunc
	ch      <-chan searchMsg
	started time.Time
	budget  time.Duration
	iters   int
	before  int // pieces the quick packer placed
}

func waitSearch(ch <-chan searchMsg) tea.Cmd { return func() tea.Msg { return <-ch } }

// searchTick refreshes the elapsed time while a search runs.
func searchTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return searchTickMsg{} })
}

func fmtSeconds(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

// nextBudget is the next search time after cur, wrapping around.
func nextBudget(cur time.Duration) time.Duration {
	for _, b := range searchBudgets {
		if b > cur {
			return b
		}
	}
	return searchBudgets[0]
}

// startSearch launches PackSearch in the background on a copy of the load.
func (m *model) startSearch() tea.Cmd {
	n := len(m.load.pieces())
	switch {
	case n == 0:
		m.status = "Nothing to search: add item sets first."
		return nil
	case n > maxSearchPieces:
		m.status = fmt.Sprintf("Too many pieces to search (%d; the limit is %d).", n, maxSearchPieces)
		return nil
	case m.layoutPack.AllPlaced():
		m.status = "Everything already fits, so there is nothing to improve."
		return nil
	}

	load := m.load
	load.Sets = append([]ItemSet(nil), m.load.Sets...)
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan searchMsg, 16)
	seed := m.searchSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	budget := m.searchBudget
	go func() {
		var last time.Time
		best, stats := load.PackSearch(ctx, SearchOptions{Budget: budget, Seed: seed,
			OnImprove: func(p Packing, iters int) {
				// Progress is best-effort; the final message is what counts.
				if time.Since(last) < 150*time.Millisecond {
					return
				}
				last = time.Now()
				select {
				case ch <- searchMsg{pack: p, iters: iters}:
				default:
				}
			}})
		ch <- searchMsg{pack: best, iters: stats.Iterations, final: true, stats: stats}
	}()

	m.search = searchState{running: true, cancel: cancel, ch: ch, started: time.Now(), budget: budget, before: len(m.layoutPack.Placed)}
	return tea.Batch(waitSearch(ch), searchTick())
}

// stopSearch asks the running search to finish; its final message still arrives.
func (m *model) stopSearch() {
	m.search.stopped = true
	m.search.cancel()
	m.status = "Stopping the search…"
}

// handleSearch applies a progress or final message from the search.
func (m model) handleSearch(sm searchMsg) (tea.Model, tea.Cmd) {
	if sm.pack.score() > m.layoutPack.score() || m.layoutPack.Skipped {
		m.layoutPack = sm.pack
	}
	m.search.iters = sm.iters
	if !sm.final {
		return m, waitSearch(m.search.ch)
	}

	m.search.running = false
	m.search.cancel()
	total, placed := m.layoutPack.Total, len(m.layoutPack.Placed)
	took := fmtSeconds(time.Since(m.search.started))
	prefix := ""
	if m.search.stopped {
		prefix = "Stopped. "
	}
	switch {
	case m.layoutPack.AllPlaced():
		m.status = fmt.Sprintf("%sSearch found a layout for all %d pieces (%d tries, %s).", prefix, total, sm.stats.Iterations, took)
	case placed > m.search.before:
		m.status = fmt.Sprintf("%sSearch placed %d of %d pieces, up from %d (%d tries, %s).", prefix, placed, total, m.search.before, sm.stats.Iterations, took)
	default:
		m.status = fmt.Sprintf("%sNo better layout found: still %d of %d placed (%d tries, %s).", prefix, placed, total, sm.stats.Iterations, took)
	}
	return m, nil
}

// layoutStatus is the line shown on the layout screen: live progress while a
// search runs, otherwise the last message.
func (m model) layoutStatus() string {
	if !m.search.running {
		return m.status
	}
	if m.search.stopped {
		return "Stopping the search…"
	}
	return fmt.Sprintf("Searching for a better layout… %s of %s · %d tries · placing %d of %d pieces · any key stops",
		fmtSeconds(time.Since(m.search.started)), fmtSeconds(m.search.budget), m.search.iters,
		len(m.layoutPack.Placed), m.layoutPack.Total)
}
