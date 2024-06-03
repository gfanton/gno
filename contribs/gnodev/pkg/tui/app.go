package tui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	io.Writer
	vm ViewModel
}

func (a *App) Run(handler MsgHandler) (err error) {
	a.vm.setHandler(handler)
	p := tea.NewProgram(a.vm)
	_, err = p.Run()
	return
}

func NewApp() *App {
	bm := NewBufferModel()
	vm := NewViewModel(bm)
	return &App{
		Writer: bm,
		vm:     vm,
	}
}
