package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	commandhelp "lsurvey/internal/help"
)

type helpCommandItem struct {
	command commandhelp.Command
}

func (item helpCommandItem) Title() string {
	return item.command.Usage
}

func (item helpCommandItem) Description() string {
	return item.command.Group + "  " + item.command.Description
}

func (item helpCommandItem) FilterValue() string {
	return commandhelp.SearchText(item.command)
}

type helpCommandDelegate struct{}

func (helpCommandDelegate) Height() int {
	return 1
}

func (helpCommandDelegate) Spacing() int {
	return 0
}

func (helpCommandDelegate) Update(tea.Msg, *list.Model) tea.Cmd {
	return nil
}

func (helpCommandDelegate) Render(w io.Writer, m list.Model, index int, raw list.Item) {
	item, ok := raw.(helpCommandItem)
	if !ok {
		return
	}

	items := m.VisibleItems()
	pageStart := m.Paginator.Page * m.Paginator.PerPage
	section := ""
	if index == pageStart || index == 0 || previousGroup(items, index) != item.command.Group {
		section = helpGroupStyle.Render(item.command.Group + "  ")
	}

	prefix := "  "
	usageStyle := helpUsageStyle
	descriptionStyle := helpDescriptionStyle
	if index == m.Index() {
		prefix = "> "
		usageStyle = helpSelectedUsageStyle
		descriptionStyle = helpSelectedDescriptionStyle
	}
	fmt.Fprintf(w, "%s%s%s  %s", prefix, section, usageStyle.Render(item.command.Usage), descriptionStyle.Render(item.command.Description))
}

func previousGroup(items []list.Item, index int) string {
	if index <= 0 || index > len(items)-1 {
		return ""
	}
	previous, ok := items[index-1].(helpCommandItem)
	if !ok {
		return ""
	}
	return previous.command.Group
}

var (
	helpGroupStyle               = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	helpUsageStyle               = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	helpDescriptionStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	helpSelectedUsageStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	helpSelectedDescriptionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
)

func newHelpBrowser(width, height int) list.Model {
	items := make([]list.Item, 0, len(commandhelp.All()))
	for _, command := range commandhelp.All() {
		items = append(items, helpCommandItem{command: command})
	}
	browser := list.New(items, helpCommandDelegate{}, width, height)
	browser.Title = "Command Help"
	browser.Filter = list.UnsortedFilter
	browser.SetStatusBarItemName("command", "commands")
	browser.DisableQuitKeybindings()
	browser.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
			key.NewBinding(key.WithKeys("f1"), key.WithHelp("F1", "close")),
		}
	}
	return browser
}
