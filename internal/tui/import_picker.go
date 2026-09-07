package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/paths"
)

func (m *Model) startImportPicker() tea.Cmd {
	picker := filepicker.New()
	picker.CurrentDirectory = "."
	picker.ShowHidden = true
	picker.ShowPermissions = false
	picker.AllowedTypes = []string{".srv", ".SRV", ".csv", ".CSV", ".geojson", ".GEOJSON"}
	picker.SetHeight(max(5, m.height-7))

	m.importPicker = picker
	m.importPicking = true
	m.importReturn = m.mode
	m.message = "select a .srv, .csv, or .geojson file"
	m.lastErr = ""
	return m.importPicker.Init()
}

func (m Model) canStartImportPicker() bool {
	if m.importPicking || m.convert.picking || m.convert.confirm || m.convert.form != conversionFormNone {
		return false
	}
	if m.style.form != styleFormNone {
		return false
	}
	if m.mode == ModeHelp && m.helpList.SettingFilter() {
		return false
	}
	return true
}

func (m Model) updateImportPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			cmd := m.dispatchCommand("quit")
			return m, cmd
		case "esc":
			m.importPicking = false
			m.mode = m.importReturn
			m.message = "file selection cancelled"
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.importPicker, cmd = m.importPicker.Update(msg)
	if selected, path := m.importPicker.DidSelectFile(msg); selected {
		return m.finishPickedImport(path)
	}
	if selected, path := m.importPicker.DidSelectDisabledFile(msg); selected {
		m.setError("unsupported file type " + strconvQuote(filepath.Base(path)))
	}
	return m, cmd
}

func (m Model) finishPickedImport(path string) (tea.Model, tea.Cmd) {
	command, err := importCommandForPath(path)
	if err != nil {
		m.setError(err.Error())
		return m, nil
	}
	m.importPicking = false
	cmd := (&m).dispatchCommand(command)
	m.mode = ModeMain
	return m, cmd
}

func (m Model) renderImportPicker(width, height int) string {
	title := "Select project or import file"
	body := fmt.Sprintf("Directory: %s\n\n%s\nEnter/right: open/select  arrows/j/k: move  left/backspace: parent  Esc: cancel",
		m.importPicker.CurrentDirectory, m.importPicker.View())
	if m.lastErr != "" {
		body += "\n" + errorStyle.Render(m.lastErr)
	} else if m.message != "" {
		body += "\n" + m.message
	}
	return box(title, body, width, height)
}

func importCommandForPath(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case paths.ProjectExt:
		return "open " + quoteStyleField(path), nil
	case paths.CSVExt:
		return "import csv " + quoteStyleField(path), nil
	case paths.GeoJSONExt:
		return "import geojson " + quoteStyleField(path), nil
	default:
		return "", fmt.Errorf("unsupported file type %s", strconvQuote(filepath.Base(path)))
	}
}

func isProjectFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), paths.ProjectExt)
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}
