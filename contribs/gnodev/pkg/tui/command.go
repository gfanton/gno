package tui

import tea "github.com/charmbracelet/bubbletea"

type Command struct {
	KeysMap         string
	Name            string
	HelpDescription string
	Exec            func() tea.Msg
}
