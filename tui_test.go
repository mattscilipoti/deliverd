package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(m model, keys ...string) model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

// typeSet fills L, W, H (and Qty if non-empty) and presses enter.
func typeSet(m model, l, w, h, qty string) model {
	m = press(m, l, "tab", w, "tab", h, "tab")
	if qty != "" {
		m = press(m, qty)
	}
	return press(m, "enter")
}

func TestEmptyQtyDefaultsToOne(t *testing.T) {
	m := typeSet(newModel(DefaultSpace, true), "10", "20", "30", "")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 1 {
		t.Fatalf("Sets = %v, want one set with qty 1", m.load.Sets)
	}
	if m.err != nil {
		t.Errorf("err = %v", m.err)
	}
}

func TestQtyIsLastField(t *testing.T) {
	m := typeSet(newModel(DefaultSpace, true), "10", "20", "30", "4")
	want := ItemSet{Qty: 4, Dims: Dims{10, 20, 30}}
	if len(m.load.Sets) != 1 || m.load.Sets[0] != want {
		t.Fatalf("Sets = %v, want [%v]", m.load.Sets, want)
	}
}

func TestInvalidQtyRejected(t *testing.T) {
	m := typeSet(newModel(DefaultSpace, true), "10", "20", "30", "x")
	if m.err == nil || len(m.load.Sets) != 0 {
		t.Errorf("err=%v sets=%v, want an error and no sets", m.err, m.load.Sets)
	}
}

func TestMergeOnCombinesQuantities(t *testing.T) {
	m := newModel(DefaultSpace, true)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "10", "10", "10", "3")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 5 {
		t.Fatalf("Sets = %v, want one set with qty 5", m.load.Sets)
	}
}

func TestMergeOffKeepsSeparate(t *testing.T) {
	m := newModel(DefaultSpace, false)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "10", "10", "10", "3")
	if len(m.load.Sets) != 2 {
		t.Fatalf("Sets = %v, want 2", m.load.Sets)
	}
	// Turning merge on combines the existing duplicates.
	m = press(m, "esc", "m")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 5 {
		t.Fatalf("after toggle Sets = %v, want one set with qty 5", m.load.Sets)
	}
}

func TestEditExistingSet(t *testing.T) {
	m := newModel(DefaultSpace, true)
	m = typeSet(m, "10", "10", "10", "2")
	m = typeSet(m, "20", "20", "20", "1")
	// Select the first set and edit it.
	m = press(m, "esc", "k", "e")
	if m.mode != modeItem || m.editing != 0 {
		t.Fatalf("mode=%v editing=%d, want item mode editing 0", m.mode, m.editing)
	}
	if got := m.itemInputs[0].Value(); got != "10" {
		t.Errorf("L prefilled = %q, want 10", got)
	}
	// Change L from 10 to 15 and save.
	m.itemInputs[0].SetValue("15")
	m = press(m, "enter")
	want := ItemSet{Qty: 2, Dims: Dims{15, 10, 10}}
	if m.load.Sets[0] != want || len(m.load.Sets) != 2 {
		t.Fatalf("Sets = %v, want first set %v", m.load.Sets, want)
	}
	if m.mode != modeCommand || m.editing != -1 {
		t.Errorf("mode=%v editing=%d, want command mode, not editing", m.mode, m.editing)
	}
}

func TestEditCancelLeavesSetUnchanged(t *testing.T) {
	m := typeSet(newModel(DefaultSpace, true), "10", "10", "10", "2")
	m = press(m, "esc", "e", "esc")
	if m.load.Sets[0].Qty != 2 || m.editing != -1 || m.mode != modeCommand {
		t.Errorf("Sets=%v editing=%d mode=%v", m.load.Sets, m.editing, m.mode)
	}
}

func TestDeleteSelected(t *testing.T) {
	m := newModel(DefaultSpace, true)
	m = typeSet(m, "1", "1", "1", "")
	m = typeSet(m, "2", "2", "2", "")
	m = typeSet(m, "3", "3", "3", "")
	m = press(m, "esc", "k", "d") // sel starts on the last set; k moves to the middle
	if len(m.load.Sets) != 2 || m.load.Sets[0].L != 1 || m.load.Sets[1].L != 3 {
		t.Errorf("Sets = %v, want sets 1 and 3 remaining", m.load.Sets)
	}
}
