package tui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

const StatsWidgetName = "stats-widget"

type StatsWidgetModel struct {
	width      int
	nline      int
	occurenceE int
	progress   progress.Model
}

type UpdateStatsMsg struct {
	NLine      int
	OccurenceE int
	Progress   float64
}

func NewStatsWidget(width int) (m StatsWidgetModel) {
	m.progress = progress.New(progress.WithDefaultGradient())
	m.progress.Width = width
	m.width = width
	return
}

func (m StatsWidgetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.progress.Width = msg.Width
	case UpdateStatsMsg:
		m.nline = msg.NLine
		m.occurenceE = msg.OccurenceE
		return m, m.progress.SetPercent(msg.Progress)
	case progress.FrameMsg:
		// FrameMsg is sent when the progress bar wants to animate itself
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	default:
	}

	return m, nil
}

func (m StatsWidgetModel) Destroy() error {
	return nil
}

func (m StatsWidgetModel) Name() string {
	return StatsWidgetName
}

func (m StatsWidgetModel) Init() tea.Cmd {
	return nil
}

func (m StatsWidgetModel) View() string {
	t := table.New().Width(m.width).
		Row("E occurence", strconv.Itoa(m.occurenceE)).
		Row("progress", fmt.Sprintf("%f", m.progress.Percent())).
		Row("lines", strconv.Itoa(m.nline))

	return lipgloss.JoinVertical(lipgloss.Center,
		m.progress.View(),
		t.Render(),
	)
}
