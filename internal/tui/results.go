package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type commandResult struct{ Command, Text string }
type clipboardResultMsg struct{ err error }

func (m *Model) retainResult(command string) {
	body := m.message
	if m.lastErr != "" {
		body = "Error: " + m.lastErr
	}
	if body == "" {
		return
	}
	m.appendResult(commandResult{command, ansi.Strip(body)})
}

func (m *Model) appendResult(result commandResult) {
	m.results = append(m.results, result)
	if len(m.results) > 100 {
		m.results = append([]commandResult(nil), m.results[len(m.results)-100:]...)
	}
}

func (m *Model) openResults() {
	m.resultReturn = m.mode
	m.mode = ModeResults
	m.resultIndex = max(0, len(m.results)-1)
	m.resultExport = false
	m.lastErr = ""
	m.message = ""
	m.resultView = viewport.New(max(1, m.width-4), max(1, m.height-6))
	m.refreshResult()
}

func (m *Model) refreshResult() {
	body := "No command results yet."
	if len(m.results) > 0 {
		r := m.results[m.resultIndex]
		body = "> " + r.Command + "\n\n" + r.Text
	}
	m.resultView.Width = max(1, m.width-4)
	m.resultView.Height = max(1, m.height-7)
	m.resultView.SetContent(ansi.Hardwrap(body, m.resultView.Width, true))
	m.resultView.GotoTop()
}

func (m *Model) exportResult(path string) error {
	if len(m.results) == 0 {
		return fmt.Errorf("no result selected")
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	r := m.results[m.resultIndex]
	_, err = fmt.Fprintf(f, "> %s\n\n%s\n", r.Command, r.Text)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (m Model) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if m.resultExport {
			switch k.String() {
			case "esc":
				m.resultExport = false
				return m, nil
			case "enter":
				if err := m.exportResult(strings.TrimSpace(m.resultPath.Value())); err != nil {
					m.setError(err.Error())
				} else {
					m.message = "Report exported"
					m.lastErr = ""
					m.resultExport = false
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.resultPath, cmd = m.resultPath.Update(msg)
			return m, cmd
		}
		switch k.String() {
		case "esc", "f6":
			m.mode = m.resultReturn
			return m, nil
		case "[":
			m.resultIndex = max(0, m.resultIndex-1)
			m.refreshResult()
			return m, nil
		case "]":
			m.resultIndex = max(0, min(len(m.results)-1, m.resultIndex+1))
			m.refreshResult()
			return m, nil
		case "c":
			if len(m.results) > 0 {
				body := m.results[m.resultIndex].Text
				return m, func() tea.Msg { return clipboardResultMsg{clipboard.WriteAll(body)} }
			}
			return m, nil
		case "e":
			m.resultExport = true
			m.resultPath.Focus()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.resultView, cmd = m.resultView.Update(msg)
	return m, cmd
}

func (m Model) renderResults() string {
	footer := "[/]: previous/next result  c: copy  e: export  Esc/F6: back"
	if m.resultExport {
		footer = "Export (new file): " + m.resultPath.View()
	}
	if m.lastErr != "" {
		footer += "\n" + m.lastErr
	} else if m.message != "" {
		footer += "\n" + m.message
	}
	return m.renderWithTabs(box(fmt.Sprintf("Results %d/%d", min(m.resultIndex+1, len(m.results)), len(m.results)), m.resultView.View()+"\n"+footer, max(12, m.width), max(4, m.height-1)), max(12, m.width))
}
