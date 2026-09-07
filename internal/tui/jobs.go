package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/app"
)

type backgroundJob struct {
	label                  string
	started                time.Time
	cancel                 chan struct{}
	cancelled, cancellable bool
	after                  string
	styleImport            bool
	conversionCommit       bool
	commitAll              bool
	assumeExisting         bool
}
type jobTickMsg struct{}
type jobDoneMsg struct {
	session          *app.Session
	outcome          app.Outcome
	err              error
	conversion       *ConversionState
	message, lastErr string
}

func jobTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return jobTickMsg{} })
}

// Update serializes publication and rejects mutations while a worker owns its
// private session. View continues to read the original, immutable project.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(jobDoneMsg); ok {
		return m.finishJob(done)
	}
	if m.job != nil {
		switch v := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = v.Width
			m.height = v.Height
		case jobTickMsg:
			return m, jobTick()
		case tea.KeyMsg:
			if (v.String() == "esc" || v.String() == "ctrl+c") && m.job.cancellable && !m.job.cancelled {
				close(m.job.cancel)
				copy := *m.job
				copy.cancelled = true
				m.job = &copy
			}
		}
		return m, nil
	}
	if _, ok := msg.(jobTickMsg); ok {
		return m, nil
	}
	updated, cmd := m.update(msg)
	next := updated.(Model)
	if next.queuedJob != nil {
		worker := next.queuedJob
		next.queuedJob = nil
		return next, tea.Batch(cmd, worker, jobTick())
	}
	return next, cmd
}

func isUICommand(command string) bool {
	fields, err := app.Fields(command)
	if err != nil || len(fields) == 0 {
		return true
	}
	switch fields[0] {
	case "quit", "exit", "filter", "clear", "sort", "help", "map", "style", "convert", "results":
		return true
	}
	return false
}

func (m *Model) queueSessionCommand(command string) {
	fields, _ := app.Fields(command)
	cancellable := len(fields) > 0 && fields[0] != "save" && fields[0] != "saveas" && fields[0] != "export"
	job := &backgroundJob{label: command, started: time.Now(), cancel: make(chan struct{}), cancellable: cancellable}
	m.job = job
	session := m.session
	m.queuedJob = func() tea.Msg {
		select {
		case <-job.cancel:
			return jobDoneMsg{}
		default:
		}
		working := session.Fork()
		outcome, err := working.Execute(command)
		return jobDoneMsg{session: working, outcome: outcome, err: err}
	}
}

func (m *Model) queueConversion(path string) {
	job := &backgroundJob{label: "Calculate staged conversions", started: time.Now(), cancel: make(chan struct{}), cancellable: true}
	if path != "" {
		job.label = "Import staged CSV " + path
	}
	state := m.convert
	state.Rows = append([]ConversionRow(nil), state.Rows...)
	m.job = job
	m.queuedJob = func() tea.Msg {
		worker := Model{convert: state}
		if path != "" {
			if err := worker.importConversionCSV(path); err != nil {
				worker.setError(err.Error())
			}
		} else {
			worker.calculateConversions()
		}
		return jobDoneMsg{conversion: &worker.convert, message: worker.message, lastErr: worker.lastErr}
	}
}

func (m *Model) queueConversionCommit(request app.ConversionCommit, all bool) {
	job := &backgroundJob{label: "Commit converted points", started: time.Now(), cancel: make(chan struct{}), cancellable: true, conversionCommit: true, commitAll: all, assumeExisting: request.AssumeExisting}
	m.job = job
	session := m.session
	m.queuedJob = func() tea.Msg {
		working := session.Fork()
		outcome, err := working.CommitConvertedPoints(request)
		return jobDoneMsg{session: working, outcome: outcome, err: err}
	}
}

func (m Model) finishJob(done jobDoneMsg) (tea.Model, tea.Cmd) {
	if m.job == nil {
		return m, nil
	}
	job := m.job
	m.job = nil
	m.queuedJob = nil
	if job.cancelled {
		m.message = "Cancelled; no changes applied"
		m.lastErr = ""
		return m, nil
	}
	if done.err != nil {
		m.setError(done.err.Error())
		if job.conversionCommit && !job.assumeExisting && strings.Contains(done.err.Error(), "confirm they are") {
			m.convert.confirm = true
			m.convert.commitAll = job.commitAll
			m.message = done.err.Error()
			m.lastErr = ""
		}
	} else if done.conversion != nil {
		if done.lastErr != "" {
			m.setError(done.lastErr)
		} else {
			m.convert = *done.conversion
			m.message = done.message
			m.lastErr = ""
		}
	} else if done.session != nil {
		m.session = done.session
		m.syncSessionState()
		if done.outcome.ProjectChanged || done.outcome.ProjectReplaced {
			m.tableRevision++
		}
		m.message = done.outcome.Message
		m.lastErr = ""
		if done.outcome.ProjectReplaced {
			m.mapState = newMapState()
			m.style = newStyleState()
			m.convert = newConversionState()
		}
		if job.styleImport {
			m.style.form = styleFormNone
			m.style.fields = nil
			m.style.ensureSelection(m.project)
		}
		if job.conversionCommit {
			m.convert.confirm = false
			if job.commitAll {
				m.convert.Rows = nil
				m.convert.Selected = 0
			} else {
				m.removeConversionRow()
			}
		}
	}
	m.retainResult(job.label)
	m.syncMainViewports()
	m.refreshCompletions()
	if job.after != "" && m.lastErr == "" {
		m.pendingAction = job.after
		updated, cmd := m.finishProtectedAction()
		next := updated.(Model)
		if next.queuedJob != nil {
			worker := next.queuedJob
			next.queuedJob = nil
			return next, tea.Batch(cmd, worker, jobTick())
		}
		return next, cmd
	}
	return m, nil
}

func (m Model) renderBusy() string {
	elapsed := time.Since(m.job.started).Round(100 * time.Millisecond)
	policy := "Writing files; please wait for completion."
	if m.job.cancellable {
		policy = "Esc: cancel (waits for computation to finish; result will not be applied)"
	}
	if m.job.cancelled {
		policy = "Cancellation requested; waiting for worker to finish safely."
	}
	return box("Working", fmt.Sprintf("%s\nElapsed: %s %s\n\n%s", m.job.label, elapsed, strings.Repeat("·", int(time.Since(m.job.started).Milliseconds()/100)%4+1), policy), max(12, m.width), max(8, m.height))
}
