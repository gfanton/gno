package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type WidgetModel interface {
	tea.Model
	SetWidth() string
}

var (
	widgetBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder(), true, false, false, false).
				BorderForeground(lipgloss.Color("69"))
	widgetInnerStyle = lipgloss.NewStyle().Margin(0, 1, 0, 1).
				AlignHorizontal(lipgloss.Center)
	widgetBorderSize = lipgloss.Width(
		widgetBorderStyle.Render(widgetInnerStyle.Render("")),
	)
)

type Widget struct {
	Name    string
	Model   tea.Model
	InitCmd func() tea.Cmd
	Handler func(msg tea.Msg) tea.Cmd
	Cleanup func() tea.Cmd

	width, height int
}

type newWidgetMsg struct {
	widget Widget
}

func RunWidget(w Widget) tea.Msg {
	return newWidgetMsg{widget: w}
}

func (m Widget) init(width, height int) tea.Cmd {
	m.width, m.height = width, height
	if m.InitCmd != nil {
		return m.InitCmd()
	}

	return nil
}

func (m Widget) update(msg tea.Msg) (Widget, tea.Cmd) {
	if !m.isRunning() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		msg.Width, msg.Height = msg.Width-widgetBorderSize, msg.Height-widgetBorderSize
		m.width, m.height = msg.Width, msg.Height
	default:
	}

	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	if m.Handler == nil {
		return m, cmd
	}

	return m, tea.Batch(cmd, m.Handler(msg))
}

func (m Widget) destroy() (Widget, tea.Cmd) {
	if m.Model == nil {
		return m, nil
	}

	var cmd tea.Cmd
	if m.Cleanup != nil {
		cmd = m.Cleanup()
	}

	m.Model = nil
	return m, cmd
}

func (m Widget) isRunning() bool {
	return m.Model != nil
}

func (m Widget) View() string {
	return widgetBorderStyle.Width(m.width).Render(
		widgetInnerStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				timeTitleStyle(m.Name),
				m.Model.View(),
			),
		),
	)
}
