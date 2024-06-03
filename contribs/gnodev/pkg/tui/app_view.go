package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var cells []TimeCell

var (
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle           = lipgloss.NewStyle().Margin(1, 2)
	checkMark           = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
)

type MsgHandler func(tea.Msg) tea.Cmd

type ViewModel struct {
	width      int
	height     int
	buffer     BufferModel
	widget     IWidget
	currentSel int
}

func NewViewModel(bm BufferModel) ViewModel {
	return ViewModel{
		buffer: bm,
	}
}

func (m ViewModel) Init() tea.Cmd {
	return m.buffer.WriteTick
}

func (m ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		var cmd tea.Cmd
		if m.widget != nil {
			if msg.String() == "q" {
				m.widget = nil
			} else {
				m.widget, cmd = m.widget.UpdateWidget(msg)
			}

			return m, cmd
		}

		switch msg.String() {
		case "t":
			m.widget = NewTimelineWidget(m.width, cells, m.currentSel)
			return m, nil
		case "s":
			m.widget = NewStatsWidget(m.width)
			return m, nil
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}

		return m, nil

	case TimelineSelectionMsg:
		m.widget = nil
		m.currentSel = msg.Sel
		return m, tea.Printf("SELECT: %d (%s)", msg.Sel, msg.Cell.Descritpion)

	case BufferUpdateMsg:
		var cmd tea.Cmd
		m.buffer, cmd = m.buffer.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	if m.widget != nil {
		m.widget, cmd = m.widget.UpdateWidget(msg)
	}

	return m, cmd
}

func (m ViewModel) View() string {
	if m.widget == nil {
		return ""
	}

	return borderStyle.Width(m.width - 2).Render(m.widget.View())
}
