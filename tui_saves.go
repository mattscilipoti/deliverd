package main

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func setsKey(sets []ItemSet) string { return fmt.Sprint(sets) }

// dirty reports whether the item sets differ from what was last saved or opened.
func (m model) dirty() bool { return setsKey(m.load.Sets) != m.savedKey }

func (m model) storeAvailable() error {
	if m.store.Dir == "" {
		return errors.New("saving is unavailable: no config directory (use -dir)")
	}
	return nil
}

// saveTo writes the current item sets under name and makes it the current load.
func (m *model) saveTo(name string) error {
	if err := m.storeAvailable(); err != nil {
		return err
	}
	if len(m.load.Sets) == 0 {
		return errors.New("nothing to save: add an item set first")
	}
	name, err := ValidateName(name)
	if err != nil {
		return err
	}
	if err := m.store.Save(name, m.load.Sets); err != nil {
		return fmt.Errorf("couldn't save %q: %w", name, err)
	}
	m.loadName = name
	m.savedKey = setsKey(m.load.Sets)
	m.status = fmt.Sprintf("Saved %q", name)
	return nil
}

// openLoad replaces the item sets with the saved load called name.
func (m *model) openLoad(name string) error {
	sets, err := m.store.Load(name)
	if err != nil {
		return err
	}
	m.load.Sets = sets
	if m.merge {
		m.load.MergeDuplicates()
	}
	m.loadName = name
	m.savedKey = setsKey(m.load.Sets)
	m.sel, m.editing = 0, -1
	m.status = fmt.Sprintf("Opened %q", name)
	return nil
}

func (m *model) askConfirm(prompt string, action func(*model)) {
	m.mode = modeConfirm
	m.confirmPrompt = prompt
	m.confirmAction = action
	blurAll(m.itemInputs)
	m.nameInput.Blur()
}

// saveCurrent is the w key: overwrite the opened load, or ask for a name.
func (m *model) saveCurrent() {
	if m.loadName == "" {
		m.beginSaveAs()
		return
	}
	m.err = m.saveTo(m.loadName)
}

func (m *model) beginSaveAs() {
	if err := m.storeAvailable(); err != nil {
		m.err = err
		return
	}
	if len(m.load.Sets) == 0 {
		m.err = errors.New("nothing to save: add an item set first")
		return
	}
	m.mode = modeSave
	m.err = nil
	m.nameInput.SetValue(m.loadName)
	m.nameInput.CursorEnd()
	m.nameInput.Focus()
}

func (m *model) beginOpen() {
	if err := m.storeAvailable(); err != nil {
		m.err = err
		return
	}
	names, err := m.store.List()
	if err != nil {
		m.err = err
		return
	}
	if len(names) == 0 {
		m.status = "No saved loads yet. Press w to save the current items."
		return
	}
	m.openNames, m.openSel = names, 0
	for i, n := range names {
		if n == m.loadName {
			m.openSel = i
		}
	}
	m.mode = modeOpen
	m.err = nil
}

func (m model) updateSave(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.enterCommandMode()
		return m, nil
	case "enter":
		name, err := ValidateName(m.nameInput.Value())
		if err != nil {
			m.err = err
			return m, nil
		}
		save := func(m *model) {
			m.err = m.saveTo(name)
			m.enterCommandMode()
		}
		// Saving over the opened load is what the user asked for; any other
		// existing name is a different file, so confirm first.
		if name != m.loadName && m.store.Exists(name) {
			m.askConfirm(fmt.Sprintf("Overwrite the saved load %q? (y/n)", name), save)
			return m, nil
		}
		save(&m)
		return m, nil
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(key)
	return m, cmd
}

func (m model) updateOpen(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q":
		m.enterCommandMode()
	case "up", "k":
		if m.openSel > 0 {
			m.openSel--
		}
	case "down", "j":
		if m.openSel < len(m.openNames)-1 {
			m.openSel++
		}
	case "enter":
		name := m.openNames[m.openSel]
		open := func(m *model) {
			m.err = m.openLoad(name)
			m.enterCommandMode()
		}
		if m.dirty() {
			m.askConfirm(fmt.Sprintf("Discard unsaved changes and open %q? (y/n)", name), open)
			return m, nil
		}
		open(&m)
	}
	return m, nil
}

func (m model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.confirmAction
	m.confirmAction = nil
	if key.String() == "y" && action != nil {
		action(&m)
	} else {
		m.status = "Cancelled"
	}
	if m.mode == modeConfirm {
		m.enterCommandMode()
	}
	return m, nil
}

// loadLine describes which saved load is current, for the header.
func (m model) loadLine() string {
	switch {
	case m.loadName == "" && len(m.load.Sets) == 0:
		return "Saved load: none"
	case m.loadName == "":
		return "Saved load: none (unsaved items)"
	case m.dirty():
		return fmt.Sprintf("Saved load: %s (modified, w overwrites, W saves as)", m.loadName)
	}
	return fmt.Sprintf("Saved load: %s", m.loadName)
}

func (m model) saveView() string {
	var b strings.Builder
	b.WriteString("Save as:\n" + m.nameInput.View() + "\n")
	if m.loadName != "" {
		b.WriteString(dimStyle.Render("Keep the name to overwrite it, or type a new name to save a copy.") + "\n")
	}
	return b.String()
}

func (m model) openView() string {
	var b strings.Builder
	b.WriteString("Open saved load:\n")
	for i, n := range m.openNames {
		marker := "  "
		if i == m.openSel {
			marker = "▸ "
		}
		line := marker + n
		if n == m.loadName {
			line += dimStyle.Render("  (current)")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}
