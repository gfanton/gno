package tui

import (
	"fmt"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var tableBaseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.HiddenBorder()).
	BorderForeground(lipgloss.Color("240"))

type TableWidgetModel struct {
	cols  []string
	rows  [][]string
	width int
}

func NewTableWidget(columns []string, rows ...[]string) TableWidgetModel {
	// cols := make([]table.Column, len(columns))
	// for i, col := range columns {
	// 	cols[i] = table.Column{
	// 		Title: col,
	// 	}
	// }

	// rs := make([]table.Row, len(rows))
	// for i, row := range rows {
	// 	rs[i] = table.Row(row)
	// }

	// t := table.New(
	// 	table.WithColumns(cols),
	// 	table.WithRows(rs),
	// )
	// s := table.DefaultStyles()
	// s.Header = s.Header.
	// 	Bold(false)

	return TableWidgetModel{
		cols: columns,
		rows: rows,
	}
}

func (m TableWidgetModel) Init() tea.Cmd { return nil }

func (m TableWidgetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// maxWidth := m.width / len(m.cols)
		// ncols := len(m.cols)
		// widths := make([]int, ncols)
		// for _, row := range m.table.Rows() {
		// 	for i, col := range row {
		// 		widths[i] = min(max(len(col), widths[i]), maxWidth)
		// 	}
		// }
		// for i, col := range m.cols {
		// 	col.Width = widths[i]
		// 	m.cols[i] = table.Column{
		// 		Title: col.Title,
		// 		Width: maxWidth,
		// 	}
		// }

		// m.table.SetColumns(m.cols)
		// m.table.SetWidth(m.width)
		// m.table.SetHeight(len(m.table.Rows()))

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
			// case "enter":
			// 	return m, tea.Batch(
			// 		tea.Printf("Let's go to %s!", m.table.SelectedRow()[1]),
			// 	)
		}
	}

	// m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m TableWidgetModel) View() string {
	var tab strings.Builder
	tabw := tabwriter.NewWriter(&tab, 0, 0, 2, ' ', tabwriter.TabIndent)
	fmt.Fprintln(tabw, strings.Join(m.cols, "\t")) // Table header.
	for _, row := range m.rows {
		// Insert row with name, address, and balance amount.
		fmt.Fprintf(tabw, strings.Join(row, "\t")+"\n")
	}

	// Flush table.
	tabw.Flush()
	return lipgloss.NewStyle().MaxWidth(m.width).Render(tab.String())
}
