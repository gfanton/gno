package main

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gnolang/gno/contribs/gnodev/pkg/tui"
)

type TUIApp struct {
	p *tea.Program
}

func (t *TUIApp) AddCommands(cmds ...tui.Command) {
	t.p.Send(tui.AddCommandMsg(cmds...))
}

func (t *TUIApp) Run() error {
	_, err := t.p.Run()
	return err
}

func NewTUIApp(ctx context.Context, w io.Writer) (App, io.Writer) {
	bm := tui.NewBufferModel(w)
	p := tea.NewProgram(tui.NewViewModel(bm),
		tea.WithOutput(w),
		tea.WithContext(ctx),
	)

	return &TUIApp{p: p}, bm
}
