package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type MsgHandler func(tea.Msg) tea.Cmd

type ViewModel struct {
	commands   map[string]Command
	width      int
	height     int
	buffer     *BufferModel
	widget     Widget
	currentSel int
}

type addCommandMsg struct {
	cmds []Command
}

func AddCommandMsg(cmds ...Command) tea.Msg {
	return addCommandMsg{
		cmds: cmds,
	}
}

func NewViewModel(bm *BufferModel, cmds ...Command) ViewModel {
	mcmds := make(map[string]Command)
	for _, cmd := range cmds {
		mcmds[cmd.KeysMap] = cmd
	}

	return ViewModel{
		commands: mcmds,
		buffer:   bm,
	}
}

func (m ViewModel) Init() tea.Cmd {
	return m.buffer.Init()
}

func (m ViewModel) Write(buf []byte) (int, error) {
	return m.buffer.Write(buf)
}

func (m ViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case addCommandMsg:
		for _, cmd := range msg.cmds {
			m.commands[cmd.KeysMap] = cmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m.exit()
		}

		var cmd tea.Cmd
		// handle widget command
		if m.widget.isRunning() {
			if msg.String() == "q" {
				m.widget, cmd = m.widget.destroy()
			} else {
				m.widget, cmd = m.widget.update(msg)
			}

			return m, cmd
		}

		// handle app commands
		switch key {
		case "ctrl+l":
			return m, tea.ClearScreen
		case "h":
			return m.addWidget(m.helpWidget())
		case "q", "esc":
			return m.exit()
		default:
		}

		// handle users command, if any
		return m, m.execCommand(key)

	case newWidgetMsg:
		return m.addWidget(msg.widget)

	case BufferUpdateMsg:
		return m, m.buffer.Update(msg)
	}

	// forward messgae to widget
	var cmd tea.Cmd
	if m.widget.isRunning() {
		m.widget, cmd = m.widget.update(msg)
	}

	return m, cmd
}

func (m ViewModel) exit() (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.widget, cmd = m.widget.destroy()
	return m, tea.Sequence(
		cmd,
		tea.Println("Bye!"),
		tea.Quit,
	)
}

func (m ViewModel) ShortHelp() []key.Binding {
	keys := make([]key.Binding, 0, len(m.commands))
	for k, cmd := range m.commands {
		keys = append(keys, key.NewBinding(
			key.WithKeys(k),
			key.WithHelp(k, cmd.Name),
		))
	}

	sort.Slice(keys, func(i, j int) bool {
		cmp := strings.Compare(
			keys[i].Help().Key,
			keys[j].Help().Key,
		)
		return cmp > 0
	})

	keys = append(keys, key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "exit"),
	))

	return keys
}

// FullHelp returns an extended group of help items, grouped by columns.
// The help bubble will render the help in the order in which the help
// items are returned here.
func (m ViewModel) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

type HelpModelWrapper struct {
	help.Model
	KeysMap help.KeyMap
}

func (h HelpModelWrapper) Init() tea.Cmd {
	return nil
}

func (h HelpModelWrapper) View() string {
	return h.Model.View(h.KeysMap)
}

func (h HelpModelWrapper) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return h, nil
}

func (m ViewModel) helpWidget() Widget {
	model := help.New()
	model.ShowAll = true
	help := HelpModelWrapper{
		Model:   model,
		KeysMap: m,
	}

	return Widget{
		Name:  "Help",
		Model: help,
	}
}

func (m ViewModel) addWidget(w Widget) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.widget.isRunning() {
		_, cmd = m.widget.destroy()
	}

	m.widget = w
	if m.widget.InitCmd != nil {
		cmd = tea.Batch(cmd, m.widget.InitCmd())
	}

	var cmdSize tea.Cmd
	m.widget, cmdSize = m.widget.update(tea.WindowSizeMsg{
		Width:  m.width,
		Height: m.height,
	})

	return m, tea.Batch(cmd, cmdSize)
}

func (m ViewModel) execCommand(key string) tea.Cmd {
	if cmd, ok := m.commands[key]; ok {
		return cmd.Exec
	}

	return nil
}

func (m ViewModel) View() string {
	if !m.widget.isRunning() {
		return ""
	}

	return m.widget.View()
}
