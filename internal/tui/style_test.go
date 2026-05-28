package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lsurvey/internal/project"
)

func TestStyleScreenOpensFromCommandShortcutAndHelpTab(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("style")
	if m.mode != ModeStyle {
		t.Fatalf("mode=%v want style", m.mode)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(Model)
	if m.mode != ModeHelp {
		t.Fatalf("mode=%v want help", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = updated.(Model)
	if m.mode != ModeHelp {
		t.Fatalf("mode=%v want help to stay selected", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != ModeMain {
		t.Fatalf("mode=%v want main", m.mode)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyF3})
	m = updated.(Model)
	if m.mode != ModeStyle {
		t.Fatalf("F3 mode=%v want style", m.mode)
	}
}

func TestStyleScreenRendersGroupsMappingsAndNominalPalette(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("group add BOUND layer=BOUNDARIES color=1 desc=lots")
	m.ExecuteCommand("code style set PEG group=BOUND")
	m.ExecuteCommand("style")
	m.width = 110
	m.height = 36

	got := m.View()
	for _, want := range []string{
		"Code Styling", "Style Groups", "BOUND", "BOUNDARIES",
		"Point Code Defaults", "PEG", "ACI Palette", "ACI 1", "RGB(255,0,0)",
		"GROUP", "LAYER", "CODE", "Nominal ACI preview",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("style screen missing %q:\n%s", want, got)
		}
	}
	if count := strings.Count(got, "╭"); count < 5 {
		t.Fatalf("style screen should render outer and logical pane boxes, got %d:\n%s", count, got)
	}
}

func TestStyleScreenFormsSubmitAuditedCommandsAndSelectPaletteColor(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterStyle()
	m.startGroupForm(false)
	m.style.fields[0].input.SetValue("BOUND")
	m.style.fields[1].input.SetValue("BOUNDARIES")
	m.style.fields[2].input.SetValue("1")
	m.style.fields[3].input.SetValue("lots")
	m.style.Pane = stylePanePalette
	m.style.ACI = 3
	m.acceptStylePalette()
	if got := m.style.fields[2].input.Value(); got != "3" {
		t.Fatalf("selected color=%q want 3", got)
	}
	m.submitStyleForm()
	group, ok := m.project.Groups["BOUND"]
	if !ok || group.Color != 3 || group.Layer != "BOUNDARIES" {
		t.Fatalf("group=%+v ok=%v", group, ok)
	}
	m.ExecuteCommand("code style list")
	if !strings.Contains(m.message, "BOUND layer=BOUNDARIES color=3") {
		t.Fatalf("style list should show newly created group: %q", m.message)
	}

	m.startCodeForm(false)
	m.style.fields[0].input.SetValue("PEG")
	m.style.fields[1].input.SetValue("BOUND")
	m.submitStyleForm()
	if got := m.project.PointCodeStyles["PEG"]; got != "BOUND" {
		t.Fatalf("code mapping=%q want BOUND", got)
	}
	if !m.dirty {
		t.Fatal("style mutations should dirty the session")
	}
}

func TestStyleTablesPageThroughOverflowAndShowPosition(t *testing.T) {
	m := NewModel(project.New("test"), "")
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("G%02d", i)
		m.ExecuteCommand(fmt.Sprintf("group add %s layer=L%02d color=1", id, i))
		m.ExecuteCommand(fmt.Sprintf("code style set C%02d group=%s", i, id))
	}
	m.enterStyle()
	m.width = 110
	m.height = 30
	got := m.View()
	if !strings.Contains(got, "1-") || !strings.Contains(got, "/10") || strings.Contains(got, "G10") {
		t.Fatalf("initial group table should indicate overflow:\n%s", got)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	got = m.View()
	if !strings.Contains(got, "G10") || !strings.Contains(got, "/10") {
		t.Fatalf("page down should reveal lower style groups:\n%s", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(Model)
	got = m.View()
	if !strings.Contains(got, "C10") {
		t.Fatalf("page down should reveal lower code defaults:\n%s", got)
	}
}

func TestStyleTableMouseWheelMovesActiveSelection(t *testing.T) {
	m := NewModel(project.New("test"), "")
	for i := 1; i <= 6; i++ {
		m.ExecuteCommand(fmt.Sprintf("group add G%02d layer=L%02d color=1", i, i))
	}
	m.enterStyle()
	updated, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m = updated.(Model)
	if m.style.GroupIndex != 1 {
		t.Fatalf("group selection=%d want scrolled selection", m.style.GroupIndex)
	}
}

func TestStyleScreenDeleteRequiresConfirmation(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("group add BOUND layer=BOUNDARIES color=1")
	m.enterStyle()
	m.style.Pane = stylePaneGroups
	m.beginStyleDelete()
	if _, ok := m.project.Groups["BOUND"]; !ok || m.style.form != styleFormDelete {
		t.Fatal("delete should wait for confirmation")
	}
	m.confirmStyleDelete()
	if _, ok := m.project.Groups["BOUND"]; ok {
		t.Fatal("confirmed delete should submit group deletion")
	}
}

func TestStyleScreenCanCreateGroupWhileAddingPointCodeDefault(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterStyle()
	m.startCodeForm(false)
	m.style.fields[0].input.SetValue("TREE")
	updated, _ := m.updateStyleForm(tea.KeyMsg{Type: tea.KeyCtrlG})
	m = updated.(Model)
	if m.style.form != styleFormGroupAdd {
		t.Fatalf("form=%v want group add", m.style.form)
	}
	m.style.fields[0].input.SetValue("VEG")
	m.style.fields[1].input.SetValue("VEGETATION")
	m.style.fields[2].input.SetValue("3")
	m.submitStyleForm()
	if m.style.form != styleFormCodeAdd || m.style.fields[0].input.Value() != "TREE" || m.style.fields[1].input.Value() != "VEG" {
		t.Fatalf("pending mapping form=%v fields=%+v", m.style.form, m.style.fields)
	}
	m.submitStyleForm()
	if got := m.project.PointCodeStyles["TREE"]; got != "VEG" {
		t.Fatalf("mapping=%q want VEG", got)
	}
}

func TestStyleScreenUsesDistinctCreationShortcuts(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.enterStyle()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if m.style.form != styleFormGroupAdd {
		t.Fatalf("g form=%v want group add", m.style.form)
	}
	m.cancelStyleForm()

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.style.form != styleFormCodeAdd {
		t.Fatalf("c form=%v want code add", m.style.form)
	}
}

func TestStyleScreenGroupEditCanClearDescription(t *testing.T) {
	m := NewModel(project.New("test"), "")
	m.ExecuteCommand("group add BOUND layer=BOUNDARIES color=1 desc=lots")
	m.enterStyle()
	m.startGroupForm(true)
	m.style.fields[3].input.SetValue("")
	m.submitStyleForm()
	if got := m.project.Groups["BOUND"].Description; got != "" {
		t.Fatalf("description=%q want blank", got)
	}
}
