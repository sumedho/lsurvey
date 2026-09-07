package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/app"
)

func (m *Model) dispatchCommand(command string) tea.Cmd {
	fields, err := app.Fields(command)
	if err == nil && len(fields) > 0 {
		switch fields[0] {
		case "open", "new", "recover", "quit", "exit":
			if m.dirty || len(m.convert.Rows) > 0 || m.hasFormDraft() {
				m.pendingAction = command
				m.confirmSave = false
				m.lastErr = ""
				return nil
			}
		}
	}
	if m.asyncJobs && !isUICommand(command) {
		m.queueSessionCommand(command)
		return nil
	}
	return m.ExecuteCommand(command)
}

func (m Model) updateProtection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmSave {
		switch msg.String() {
		case "esc":
			m.confirmSave = false
			return m, nil
		case "enter":
			path := strings.TrimSpace(m.savePath.Value())
			if path == "" {
				m.setError("Enter a new project path")
				return m, nil
			}
			return m.saveBeforeAction("saveas " + quoteStyleField(path))
		}
		var cmd tea.Cmd
		m.savePath, cmd = m.savePath.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "esc", "c", "n", "ctrl+c":
		m.pendingAction = ""
		m.message = "Action cancelled"
		return m, nil
	case "s", "a":
		if m.pendingAction == "conversion-source" {
			return m, nil
		}
		if m.path == "" || msg.String() == "a" {
			m.confirmSave = true
			m.savePath = textinput.New()
			m.savePath.Placeholder = "project.srv"
			m.savePath.Focus()
			return m, nil
		}
		return m.saveBeforeAction("save")
	case "d", "y":
		return m.finishProtectedAction()
	}
	return m, nil
}

func (m Model) saveBeforeAction(command string) (tea.Model, tea.Cmd) {
	if m.asyncJobs {
		m.queueSessionCommand(command)
		m.job.after = m.pendingAction
		return m, nil
	}
	m.ExecuteCommand(command)
	if m.lastErr != "" {
		return m, nil
	}
	return m.finishProtectedAction()
}

func (m Model) finishProtectedAction() (tea.Model, tea.Cmd) {
	action := m.pendingAction
	m.pendingAction = ""
	m.confirmSave = false
	if action == "conversion-source" {
		m.convert.Source = nextConversionSystem(m.convert.Source)
		m.convert.Rows = nil
		m.convert.Selected = 0
		m.message = "Source changed; staged rows cleared"
		return m, nil
	}
	var cmd tea.Cmd
	if m.asyncJobs && !isUICommand(action) {
		m.queueSessionCommand(action)
	} else {
		cmd = m.ExecuteCommand(action)
	}
	return m, cmd
}

func (m Model) renderProtection() string {
	body := "Continue with: " + m.pendingAction + "\nUnsaved work will be lost.\ns: save and continue  a: save as  d: discard  c/Esc: cancel"
	if len(m.convert.Rows) > 0 {
		body += "\nStaged conversion rows are NOT saved in the project and will be discarded."
	}
	if m.hasFormDraft() {
		body += "\nForm drafts are NOT saved in the project and will be discarded."
	}
	if m.pendingAction == "conversion-source" {
		body = "Changing the source clears staged conversion rows.\nd: discard staged rows  c/Esc: cancel"
	}
	if m.confirmSave {
		body = "Save project before continuing (existing files will not be overwritten):\n" + m.savePath.View() + "\nEnter: save  Esc: back"
	}
	if m.lastErr != "" {
		body += "\n" + m.lastErr
	}
	return box("Unsaved work", body, max(12, m.width), max(8, m.height))
}

func (m Model) hasFormDraft() bool {
	if m.style.form != styleFormNone {
		for _, f := range m.style.fields {
			if strings.TrimSpace(f.input.Value()) != "" {
				return true
			}
		}
	}
	if m.convert.form != conversionFormNone {
		for _, f := range m.convert.fields {
			if strings.TrimSpace(f.input.Value()) != "" {
				return true
			}
		}
	}
	return false
}
