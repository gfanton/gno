package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const TimelineWidgetName = "timeline-widget"

var (
	timeConnection = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).
			Render("====")

	timeActiveCell = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Render("(●)")

	timeInacativeCell = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).
				Render("( )")

	timeTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1).
			Render
	timeDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
			Render
)

type TimeCell struct {
	Title       string
	Descritpion string
	Data        any
}

type TimeAppendCellMsg struct {
	Cell TimeCell
}

type TimelineSelectionMsg struct {
	Sel  int
	Cell TimeCell
}

type TimelineModel struct {
	width int

	cells             []TimeCell
	lowBound, upBound int
	sel               int
}

func NewTimelineWidget(width int, cells []TimeCell, sel int) TimelineModel {
	m := TimelineModel{
		width: width,
		cells: cells,
		sel:   0,
	}

	m.evaluateBoundary()
	return m
}

func (m TimelineModel) Name() string {
	return TimelineWidgetName
}

func (m TimelineModel) Init() tea.Cmd {
	return nil
}

func (m TimelineModel) Current() TimeCell {
	return m.cells[m.sel]
}

func (m TimelineModel) Destroy() error {
	return nil
}

func (m TimelineModel) Append(cell TimeCell) TimelineModel {
	m.cells = append(m.cells, cell)
	return m
}

func (m TimelineModel) GetSelection() tea.Msg {
	return TimelineSelectionMsg{
		Sel:  m.sel,
		Cell: m.cells[m.sel],
	}
}

func (m *TimelineModel) maxVisibleCells() int {
	counterWidth := lipgloss.Width(m.renderCounter())
	return (m.width - counterWidth) / lipgloss.Width(timeConnection+timeActiveCell)
}

func (m *TimelineModel) evaluateBoundary() {
	maxcells := m.maxVisibleCells()
	m.upBound = m.lowBound + maxcells

	switch {
	case m.sel < m.lowBound:
		m.lowBound = max(0, m.sel)
		m.upBound = m.lowBound + maxcells
	case m.sel >= m.upBound:
		maxcells := m.maxVisibleCells()
		m.upBound = min(m.sel+1, len(m.cells))
		m.lowBound = m.upBound - maxcells
	}
}

func (m TimelineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.evaluateBoundary()
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, m.GetSelection
		case "right":
			if newsel := m.sel + 1; newsel < len(m.cells) {
				m.sel = newsel
			}

		case "left":
			if newsel := m.sel - 1; newsel >= 0 {
				m.sel = newsel
			}
		}
		m.evaluateBoundary()
	case TimeAppendCellMsg:
		return m.Append(msg.Cell), nil
	default:
	}

	return m, nil
}

var timeCountStyle = lipgloss.NewStyle().Margin(0, 1)

func (m TimelineModel) renderCounter() string {
	total := strconv.Itoa(len(m.cells))
	return timeCountStyle.Render(
		fmt.Sprintf(" %."+strconv.Itoa(len(total))+"d/%s", m.sel+1, total),
	)
}

func (m TimelineModel) View() string {
	var tline strings.Builder

	// Draw timeline
	for i := m.lowBound; i < m.upBound; i++ {
		// alternate between line and cell to animate if moving out of
		// the boundary
		if i != 0 && (i > m.lowBound || m.lowBound%2 != 0) {
			tline.WriteString(timeConnection)
		}

		if m.sel == i {
			tline.WriteString(timeActiveCell)
		} else {
			tline.WriteString(timeInacativeCell)
		}

		if m.upBound != len(m.cells) &&
			i == m.upBound-1 && m.lowBound%2 == 0 {
			tline.WriteString(timeConnection)
		}
	}

	currentCell := m.cells[m.sel]
	counter := m.renderCounter()
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, tline.String(), counter),
		lipgloss.NewStyle().MarginTop(1).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				timeTitleStyle(currentCell.Title),
				timeDescStyle(currentCell.Descritpion),
			)),
	)
}
