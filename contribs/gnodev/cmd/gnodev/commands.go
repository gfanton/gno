package main

import (
	"context"
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gnolang/gno/contribs/gnodev/pkg/address"
	"github.com/gnolang/gno/contribs/gnodev/pkg/dev"
	"github.com/gnolang/gno/contribs/gnodev/pkg/tui"
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
