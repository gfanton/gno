package main

import (
	"github.com/gnolang/gno/contribs/gnodev/pkg/tui"
)

type App interface {
	AddCommands(cmds ...tui.Command)
	Run() error
}
