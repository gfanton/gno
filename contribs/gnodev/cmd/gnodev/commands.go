package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/gnolang/gno/contribs/gnodev/pkg/address"
	"github.com/gnolang/gno/contribs/gnodev/pkg/dev"
	"github.com/gnolang/gno/contribs/gnodev/pkg/emitter"
	"github.com/gnolang/gno/contribs/gnodev/pkg/events"
	"github.com/gnolang/gno/contribs/gnodev/pkg/tui"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/std"
)

func newNodeReloadCommand(ctx context.Context, logger *slog.Logger, dnode *dev.Node) tui.Command {
	return tui.Command{
		Name:            "Reload",
		HelpDescription: "Reload the node",
		KeysMap:         "r",
		Exec: func() tea.Msg {
			logger.WithGroup(NodeLogName).Info("reloading...")
			if err := dnode.Reload(ctx); err != nil {
				logger.WithGroup(NodeLogName).Error("reloading", "err", err)
			} else {
				logger.WithGroup(NodeLogName).Info("reloading success !")
			}
			return nil
		},
	}
}

func newNodeResetCommand(ctx context.Context, logger *slog.Logger, dnode *dev.Node) tui.Command {
	return tui.Command{
		Name:            "Reset",
		HelpDescription: "Reset all packages to take change into account",
		KeysMap:         "ctrl+r",
		Exec: func() tea.Msg {
			logger.WithGroup(NodeLogName).Info("reseting...")
			if err := dnode.Reset(ctx); err != nil {
				logger.WithGroup(NodeLogName).Error("reseting", "err", err)
			} else {
				logger.WithGroup(NodeLogName).Info("reseting success !")
			}
			return nil
		},
	}
}

func newAccountCommand(ctx context.Context, logger *slog.Logger, bk *address.Book) tui.Command {
	cols := []string{"KeyName", "Address", "Balance"}
	return tui.Command{
		Name:            "Account",
		HelpDescription: "Show accounts status",
		KeysMap:         "a",
		Exec: func() tea.Msg {
			entries := bk.List()
			rows := make([][]string, len(entries))
			for i, entry := range entries {
				address := entry.Address.String()
				qres, err := client.NewLocal().ABCIQuery("auth/accounts/"+address, []byte{})
				if err != nil {
					return fmt.Errorf("unable to query account %q: %w", address, err)
				}

				var qret struct{ BaseAccount std.BaseAccount }
				if err = amino.UnmarshalJSON(qres.Response.Data, &qret); err != nil {
					return fmt.Errorf("unable to unmarshal query response: %w", err)
				}

				if len(entry.Names) == 0 {
					rows[i] = []string{"_", address,
						qret.BaseAccount.GetCoins().String(),
					}
					continue
				}

				for _, name := range entry.Names {
					// Insert row with name, address, and balance amount.
					rows[i] = []string{name, address,
						qret.BaseAccount.GetCoins().String(),
					}
				}
			}
			return tui.RunWidget(tui.Widget{
				Name:  "Account",
				Model: tui.NewTableWidget(cols, rows...),
			})
		},
	}
}

func newTimelineCommand(ctx context.Context, logger *slog.Logger, emitter *emitter.LocalServer, node *dev.Node) tui.Command {
	type nextEventMsg struct {
		evt events.Event
	}

	waitNextEvent := func() tea.Msg {
		evt, ok := <-emitter.Recv(ctx)
		if !ok {
			return nil
		}

		return nextEventMsg{
			evt: evt,
		}
	}

	return tui.Command{
		Name:            "Timeline",
		HelpDescription: "cross the timeline",
		KeysMap:         "t",
		Exec: func() tea.Msg {
			state, index, err := node.ExportCurrentState(ctx)
			if err != nil {
				logger.Error("unable to load txs", "err", err)
				return nil
			}

			timecells := generateTimelineCells(state...)
			return tui.RunWidget(tui.Widget{
				InitCmd: func() tea.Cmd {
					return waitNextEvent
				},
				Handler: func(msg tea.Msg) tea.Cmd {
					switch msg := msg.(type) {
					case nextEventMsg:

						switch evt := msg.evt.(type) {
						case events.TxResult:
							cells := generateTimelineCells(evt.Tx)
							return tea.Batch(func() tea.Msg {
								return tui.TimeAppendCellMsg{
									Cell: cells[0],
								}
							}, waitNextEvent)
						case events.Reload, events.Reset:
							return tea.Batch(func() tea.Msg {
								state, _, err := node.ExportCurrentState(ctx)
								if err != nil {
									logger.Error("unable to load txs", "err", err)
									return nil
								}
								cells := generateTimelineCells(state...)
								return tui.TimeSetCellsMsg{
									Cells: cells,
								}
							}, waitNextEvent)
						}

					case tui.TimelineSelectionMsg:
						logger.Info("time traveling to tx", "index", msg.Sel)
						return func() tea.Msg {
							if err := node.MoveTo(ctx, msg.Sel); err != nil {
								logger.Error("unable to move in time", "sel", msg.Sel, "err", err)
							} else {
								logger.Info("moved to tx success", "index", msg.Sel)
							}
							return nil
						}
					}

					return nil
				},
				Model: tui.NewTimelineWidget(timecells, index),
			})
		},
	}
}

func generateTimelineCells(txs ...std.Tx) []tui.TimeCell {
	cells := make([]tui.TimeCell, len(txs))
	for i, tx := range txs {
		cell := &cells[i]
		if len(tx.Msgs) == 0 {
			cell.Title = "Emtpy"
			continue
		}

		switch msg := tx.Msgs[0].(type) {
		case vm.MsgCall:
			cell.Title = fmt.Sprintf("MsgCall")
			cell.Descritpion = fmt.Sprintf("%s.%s(%s)", msg.PkgPath, msg.Func,
				strings.Join(msg.Args, ","))
		case vm.MsgRun:
			cell.Title = "msgrun"
		case vm.MsgAddPackage:
			cell.Title = "addpkg"
		default:
		}

	}
	return cells
}

func newRealmCommand(ctx context.Context, emitter *emitter.LocalServer) tui.Command {
	var width int

	updateCmd := func(input string) tea.Cmd {
		return func() tea.Msg {
			<-emitter.Recv(ctx)
			return tui.BrowserUpdateInputMsg{
				Input: input,
			}
		}
	}

	return tui.Command{
		Name:            "Realm",
		HelpDescription: "Display realm",
		KeysMap:         "v",
		Exec: func() tea.Msg {
			return tui.RunWidget(tui.Widget{
				Handler: func(msg tea.Msg) tea.Cmd {
					switch msg := msg.(type) {
					case tea.WindowSizeMsg:
						width = msg.Width
					case tui.BrowserUpdateInputMsg:
						md, err := renderRealmMarkdown(msg.Input, width)
						if err != nil {
							md = err.Error()
						}

						return tea.Batch(
							tui.BrowserUpdateContent(md),
							updateCmd(msg.Input),
						)
					}

					return nil
				},
				Model: tui.NewBrowserWidget(""),
			})
		},
	}
}

func renderRealmMarkdown(realm string, width int) (string, error) {
	args := "" // XXX
	path := "vm/qrender"
	data := []byte(fmt.Sprintf("%s\n%s", realm, args))

	qres, err := client.NewLocal().ABCIQuery(path, data)
	if err != nil {
		return "", fmt.Errorf("unable to render realm %q: %w", realm, err)
	}

	if qres.Response.Error != nil {
		return "", fmt.Errorf("render failed: %w", qres.Response.Error)
	}

	tr, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(glamour.DraculaStyle),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", fmt.Errorf("unable to get render view: %w", err)
	}

	view, err := tr.RenderBytes(qres.Response.Data)
	if err != nil {
		return "", fmt.Errorf("uanble to render markdown view: %w", err)
	}

	return string(view), nil
}
