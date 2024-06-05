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
		widgetBorderStyle.Render(""),
	)
)

type Widget struct {
	Name    string
	Model   tea.Model
	InitCmd func() tea.Cmd
	Handler func(msg tea.Msg) tea.Cmd
	Cleanup func() tea.Cmd

	altScreen     bool
	width, height int
}

type WidgetAltScreenModeUpdateMsg bool

func widgetUpdateAltScreen(alt bool) tea.Cmd {
	return func() tea.Msg { return WidgetAltScreenModeUpdateMsg(alt) }
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

func (m Widget) update(tmsg tea.Msg) (Widget, tea.Cmd) {
	if !m.isRunning() {
		return m, nil
	}

	var cmd tea.Cmd
	switch msg := tmsg.(type) {
	case tea.KeyMsg:
		if m.isRunning() && msg.String() == "ctrl+f" {
			return m.switchAltScreen()
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height-widgetBorderSize
		// if !m.altScreen {
		// 	msg.Height = m.height / 2
		// }

		tmsg = msg
	default:
	}

	m.Model, cmd = m.Model.Update(tmsg)
	if m.Handler == nil {
		return m, cmd
	}

	return m, tea.Batch(cmd, m.Handler(tmsg))
}

func (m Widget) destroy() (Widget, tea.Cmd) {
	if m.Model == nil {
		return m, nil
	}

	var cmd tea.Cmd
	if m.altScreen {
		m, cmd = m.switchAltScreen()
	}

	if m.Cleanup != nil {
		cmd = tea.Batch(cmd, m.Cleanup())
	}

	m.Model = nil
	return m, cmd
}

func (m Widget) isRunning() bool {
	return m.Model != nil
}

func (m Widget) switchAltScreen() (Widget, tea.Cmd) {
	if m.altScreen {
		m.altScreen = false
		return m, tea.Sequence(tea.ExitAltScreen, widgetUpdateAltScreen(m.altScreen))
	}
	m.altScreen = true
	return m, tea.Sequence(tea.EnterAltScreen, widgetUpdateAltScreen(m.altScreen))
}

func (m Widget) View() string {
	var body string
	if m.Name != "" {
		body = lipgloss.JoinVertical(lipgloss.Left,
			timeTitleStyle(m.Name),
			m.Model.View(),
		)
	} else {
		body = m.Model.View()
	}

	return widgetBorderStyle.Width(m.width).Render(
		widgetInnerStyle.Render(body),
	)
}
