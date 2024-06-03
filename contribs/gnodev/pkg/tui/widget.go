package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type IWidget interface {
	tea.Model

	UpdateWidget(msg tea.Msg) (IWidget, tea.Cmd)
	Destroy() error
	Name() string
}

var (
	borderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("69"))
)

type Widget struct {
	Name      string
	InitModel func() tea.Model
	Handler   func() tea.Cmd

	width, height int
}

func (m Widget) Init() tea.Cmd {
	return nil
}

func (m Widget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		msg.Width, msg.Height = msg.Width-2, msg.Height-2
	default:
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m Widget) View() string {
	return borderStyle.Width(m.width - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			timeTitleStyle(m.Name),
			m.Model.View(),
		),
	)
}
