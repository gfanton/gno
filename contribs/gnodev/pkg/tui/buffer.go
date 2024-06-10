package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// BufferModel is the Bubble Tea model for this viewport element.
type BufferModel struct {
	Width  int
	Height int

	sub chan struct{}

	defw  io.Writer
	buf   bytes.Buffer
	lines []string
	muBuf sync.Mutex
}

// NewBufferModel returns a new buffer model
func NewBufferModel(defaultWriter io.Writer) *BufferModel {
	var m BufferModel
	m.defw = defaultWriter
	m.sub = make(chan struct{}, 1)
	m.buf = bytes.Buffer{}
	m.muBuf = sync.Mutex{}
	return &m
}

func (m *BufferModel) updateActivity() {
	select {
	case m.sub <- struct{}{}:
	default:
	}
}

type BufferUpdateMsg struct{}

// Init exists to satisfy the tea.Model interface for composability purposes.
func (m *BufferModel) Init() tea.Cmd {
	return m.NextLine
}

// NextLine is the command used to advance the spinner one frame. Use this command
// to effectively start the spinner.
func (m *BufferModel) NextLine() tea.Msg {
	return BufferUpdateMsg{}
}

func (m *BufferModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case BufferUpdateMsg:
	default:
		return nil
	}

	m.muBuf.Lock()
	defer m.muBuf.Unlock()

	cmd, err := m.printNextLine()
	if err != nil {
		panic(fmt.Errorf("unable to read next buffer line: %w", err))
	}

	if m.buf.Len() > 0 {
		return tea.Sequence(
			cmd,
			m.NextLine,
		)
	}

	return tea.Sequence(
		cmd,
		waitForActivity(m.sub),
	)
}

// var CLRF =  []byte{'\r', '\n'}
func (m *BufferModel) Write(buf []byte) (n int, err error) {
	m.muBuf.Lock()
	defer m.muBuf.Unlock()

	n, err = m.buf.Write(buf)
	if err != nil {
		panic(err)
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

func (m *BufferModel) printNextLine() (tea.Cmd, error) {
	if m.buf.Len() == 0 {
		return nil, nil
	}

	line, err := m.buf.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unable to read buffer: %w", err)
	}

	// readline
	return tea.Println(strings.TrimRightFunc(line, unicode.IsSpace)), nil
}

func (m *BufferModel) View() string {
	return ""
}

// A command that waits for the activity on a channel.
func waitForActivity(sub chan struct{}) tea.Cmd {
	return func() tea.Msg {
		return BufferUpdateMsg(<-sub)
	}
}
