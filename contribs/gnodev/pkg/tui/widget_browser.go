package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const useHighPerformanceRenderer = true
const gnoPrefix = "gno.land/r/"

var (
	mdBoxRoundedStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		return lipgloss.NewStyle().
			BorderStyle(b).
			Padding(0, 2)
	}()

	mdInputStyleLeft = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().
			BorderStyle(b).
			Padding(0, 2)
	}()

	mdInfoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return mdBoxRoundedStyle.Copy().BorderStyle(b)
	}()
)

type BrowserWidgetModel struct {
	ready         bool
	width, height int
	altscreen     bool
	urlInput      textinput.Model
	viewport      viewport.Model
}

type browserUpdateContentMsg struct {
	content string
}

func BrowserUpdateContent(content string) tea.Cmd {
	return func() tea.Msg {
		return browserUpdateContentMsg{
			content: content,
		}
	}
}

// public event
type BrowserUpdateInputMsg struct {
	Input string
}

func browserUpdateInput(input string) tea.Cmd {
	return func() tea.Msg {
		return BrowserUpdateInputMsg{
			Input: input,
		}
	}
}

func NewBrowserWidget(placeholder string) (m BrowserWidgetModel) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.PromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF06B7"))
	ti.Prompt = "gno.land/r/"

	return BrowserWidgetModel{
		urlInput: ti,
	}
}

func (m BrowserWidgetModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m, browserUpdateInput(m.urlInput.Prompt + m.urlInput.Value())
		default:
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			return m, cmd
		}
	case browserUpdateContentMsg:
		m.viewport.SetContent(msg.content)
		return m, nil
	case WidgetAltScreenModeUpdateMsg:
		m.altscreen = bool(msg)
		return m.resize()
	case tea.WindowSizeMsg:
		m.width = msg.Width - 10
		m.height = msg.Height
		return m.resize()

	default:
	}

	return m, nil
}

func (m BrowserWidgetModel) resize() (tea.Model, tea.Cmd) {
	width, height := m.width, m.height
	if !m.altscreen {
		height /= 2
	}

	// headerHeight := lipgloss.Height(m.headerView())
	footerHeight := lipgloss.Height(m.urlView())
	// verticalMarginHeight := headerHeight + footerHeight
	verticalMarginHeight := footerHeight

	if !m.ready {
		// Since this program is using the full size of the viewport we
		// need to wait until we've received the window dimensions before
		// we can initialize the viewport. The initial dimensions come in
		// quickly, though asynchronously, which is why we wait for them
		// here.
		m.viewport = viewport.New(width, height-verticalMarginHeight)
		// m.viewport.HighPerformanceRendering = useHighPerformanceRenderer
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height - verticalMarginHeight
	}

	return m, viewport.Sync(m.viewport)
}

func (m BrowserWidgetModel) urlView() string {
	return mdBoxRoundedStyle.
		Width(m.width).
		Render(m.urlInput.View())
}

// func (m BrowserWidgetModel) footerView() string {
// 	info := mdInfoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
// 	line := strings.Repeat("─", m.width-lipgloss.Width(info))
// 	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
// }

func (m BrowserWidgetModel) bodyView() string {
	return m.viewport.View()
}

func (m BrowserWidgetModel) Destroy() error {
	return nil
}

func (m BrowserWidgetModel) Init() tea.Cmd {
	return nil
}

func (m BrowserWidgetModel) View() string {
	if m.altscreen {
		return lipgloss.JoinVertical(lipgloss.Center,
			m.bodyView(),
			m.urlView(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		m.urlView(),
		m.bodyView(),
	)
}
