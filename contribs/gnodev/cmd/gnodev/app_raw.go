package main

import (
	"context"
	"io"

	"github.com/gnolang/gno/contribs/gnodev/pkg/tui"
)

type RawApp struct {
	ctx context.Context
}

func (r *RawApp) Run() error {
	<-r.ctx.Done()
	return context.Cause(r.ctx)
}

func (r *RawApp) AddCommands(cmds ...tui.Command) {
	// noop
}

func NewRawApp(ctx context.Context, w io.Writer) (App, io.Writer) {
	return &RawApp{}, w
}
