package tui

import (
	"bytes"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// BufferModel is the Bubble Tea model for this viewport element.
type BufferModel struct {
	Width  int
	Height int

	sub chan struct{}

	buf   *bytes.Buffer
	muBuf *sync.Mutex
}

// NewBufferModel returns a new buffer model
func NewBufferModel() BufferModel {
	var m BufferModel
	m.sub = make(chan struct{}, 1)
	m.buf = &bytes.Buffer{}
	m.muBuf = &sync.Mutex{}
	return m
}

func (m *BufferModel) updateActivity() {
	select {
	case m.sub <- struct{}{}:
	default:
	}
}

type BufferUpdateMsg struct{}

// Init exists to satisfy the tea.Model interface for composability purposes.
func (m BufferModel) Init() tea.Cmd {
	return waitForActivity(m.sub) // wait for activity
}

// WriteTick is the command used to advance the spinner one frame. Use this command
// to effectively start the spinner.
func (m BufferModel) WriteTick() tea.Msg {
	return BufferUpdateMsg{}
}

func (m BufferModel) Update(msg tea.Msg) (BufferModel, tea.Cmd) {
	m.muBuf.Lock()
	str := strings.TrimSpace(m.buf.String())
	m.buf.Reset()
	m.muBuf.Unlock()
	return m, tea.Batch(
		tea.Printf(str),
		waitForActivity(m.sub),
	)
}

func (m BufferModel) Write(buf []byte) (n int, err error) {
	m.muBuf.Lock()
	defer m.muBuf.Unlock()

	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		todo := len(buf)
		if i >= 0 {
			todo = i
		}

		var nn int
		if m.buf.Len() == 0 {
			// XXX: Wirte `soh` to avoid left trim from bubbletea
			nn, err = m.buf.WriteRune('\x01')
			n += nn
		}

		nn, err = m.buf.Write(buf[:todo])
		n += nn
		if err != nil {
			return n, err
		}
		buf = buf[todo:]

		if i >= 0 {
			if _, err = m.buf.WriteRune('\n'); err != nil {
				return n, err
			}
			n++
			buf = buf[1:]
		}
	}

	if m.buf.Len() > 0 {
		// Signal update
		select {
		case m.sub <- struct{}{}:
		default:
		}
	}

	return n, err

}

func (m BufferModel) View() string {
	return ""
}

// A command that waits for the activity on a channel.
func waitForActivity(sub chan struct{}) tea.Cmd {
	return func() tea.Msg {
		return BufferUpdateMsg(<-sub)
	}
}
