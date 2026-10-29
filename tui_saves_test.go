package main

import (
	"strings"
	"testing"
)

func newSaveModel(t *testing.T) model {
	t.Helper()
	return newModel(options{seatsDown: true, merge: true, storeDir: t.TempDir()})
}

func saveAsName(m model, name string) model {
	m = press(m, "esc", "W")
	m.nameInput.SetValue(name)
	return press(m, "enter")
}

func TestSaveNewLoad(t *testing.T) {
	m := typeSet(newSaveModel(t), "10", "10", "10", "2")
	m = press(m, "esc", "w") // no name yet, so it prompts
	if m.mode != modeSave {
		t.Fatalf("mode = %v, want the name prompt", m.mode)
	}
	m = press(m, "Tuesday", "enter")
	if m.loadName != "Tuesday" || m.dirty() || m.mode != modeCommand {
		t.Errorf("loadName=%q dirty=%v mode=%v", m.loadName, m.dirty(), m.mode)
	}
	got, err := m.store.Load("Tuesday")
	if err != nil || len(got) != 1 || got[0].Qty != 2 {
		t.Errorf("stored = %v, %v", got, err)
	}
	if !strings.Contains(m.View(), `Saved "Tuesday"`) {
		t.Errorf("view missing save confirmation:\n%s", m.View())
	}
}

func TestEditThenOverwrite(t *testing.T) {
	m := saveAsName(typeSet(newSaveModel(t), "10", "10", "10", "2"), "run")
	m = press(m, "i")
	m = typeSet(m, "20", "20", "20", "1")
	if !m.dirty() || !strings.Contains(m.View(), "modified") {
		t.Fatalf("dirty=%v, want modified after adding a set", m.dirty())
	}
	m = press(m, "esc", "w") // overwrite, no prompt
	if m.mode != modeCommand || m.dirty() {
		t.Errorf("mode=%v dirty=%v, want saved silently", m.mode, m.dirty())
	}
	got, _ := m.store.Load("run")
	if len(got) != 2 {
		t.Errorf("stored sets = %v, want the edited load", got)
	}
}

func TestSaveAsKeepsOriginal(t *testing.T) {
	m := saveAsName(typeSet(newSaveModel(t), "10", "10", "10", "2"), "original")
	m = press(m, "i")
	m = typeSet(m, "20", "20", "20", "1")
	m = saveAsName(m, "copy")
	if m.loadName != "copy" {
		t.Errorf("loadName = %q, want copy", m.loadName)
	}
	orig, _ := m.store.Load("original")
	cp, _ := m.store.Load("copy")
	if len(orig) != 1 || len(cp) != 2 {
		t.Errorf("original=%v copy=%v, want original untouched", orig, cp)
	}
}

func TestSaveAsExistingNameAsksFirst(t *testing.T) {
	m := saveAsName(typeSet(newSaveModel(t), "10", "10", "10", "1"), "a")
	m = press(m, "i")
	m = typeSet(m, "20", "20", "20", "1")
	m = saveAsName(m, "b") // b is new, saved without a prompt
	m = press(m, "c")      // clear
	m = press(m, "i")
	m = typeSet(m, "5", "5", "5", "9")
	m = saveAsName(m, "a") // a exists and isn't the open load (b is)
	if m.mode != modeConfirm || !strings.Contains(m.View(), `Overwrite the saved load "a"?`) {
		t.Fatalf("mode=%v, want an overwrite confirmation", m.mode)
	}
	m = press(m, "n")
	if got, _ := m.store.Load("a"); len(got) != 1 || got[0].Qty != 1 {
		t.Errorf("a = %v after cancelling, want it unchanged", got)
	}
	m = saveAsName(press(m, "i"), "a")
	m = press(m, "y")
	if got, _ := m.store.Load("a"); len(got) != 1 || got[0].Qty != 9 {
		t.Errorf("a = %v after confirming, want the new content", got)
	}
}

func TestOpenLoad(t *testing.T) {
	m := saveAsName(typeSet(newSaveModel(t), "10", "10", "10", "3"), "first")
	m = press(m, "c") // clear: now differs from what's saved
	m = press(m, "o")
	if m.mode != modeOpen {
		t.Fatalf("mode = %v, want the open list", m.mode)
	}
	m = press(m, "enter")
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v, want a discard confirmation since the load was cleared", m.mode)
	}
	m = press(m, "y")
	if len(m.load.Sets) != 1 || m.load.Sets[0].Qty != 3 || m.dirty() || m.mode != modeCommand {
		t.Errorf("Sets=%v dirty=%v mode=%v", m.load.Sets, m.dirty(), m.mode)
	}
}

func TestOpenCleanLoadNeedsNoConfirm(t *testing.T) {
	m := saveAsName(typeSet(newSaveModel(t), "10", "10", "10", "1"), "x")
	m = press(m, "o", "enter")
	if m.mode != modeCommand || m.err != nil {
		t.Errorf("mode=%v err=%v, want opened straight away", m.mode, m.err)
	}
}

func TestOpenWithNothingSaved(t *testing.T) {
	m := press(newSaveModel(t), "esc", "o")
	if m.mode != modeCommand || !strings.Contains(m.status, "No saved loads") {
		t.Errorf("mode=%v status=%q", m.mode, m.status)
	}
}

func TestSaveErrors(t *testing.T) {
	m := press(newSaveModel(t), "esc", "w")
	if m.err == nil || m.mode != modeCommand {
		t.Errorf("empty save: err=%v mode=%v, want an error", m.err, m.mode)
	}
	m = typeSet(newSaveModel(t), "10", "10", "10", "1")
	m = saveAsName(m, "bad/name")
	if m.err == nil || m.mode != modeSave || len(listOrNil(m)) != 0 {
		t.Errorf("bad name: err=%v mode=%v saved=%v", m.err, m.mode, listOrNil(m))
	}
	noDir := typeSet(newModel(options{seatsDown: true, merge: true}), "10", "10", "10", "1")
	noDir = press(noDir, "esc", "w")
	if noDir.err == nil {
		t.Error("saving with no storage directory should report an error")
	}
}

func listOrNil(m model) []string {
	names, _ := m.store.List()
	return names
}
