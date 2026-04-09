package nodedata

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/crypto/tmhash"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// TxSummary holds lightweight info about a transaction for block-level display.
type TxSummary struct {
	Hash      string `json:"hash"`
	Type      string `json:"type"`
	Sender    string `json:"sender"`
	GasWanted int64  `json:"gas_wanted"`
}

// TxDetail holds the full decoded payload of a transaction.
type TxDetail struct {
	Hash     string      `json:"hash"`
	Height   int64       `json:"height"`
	Index    int         `json:"index"`
	Fee      FeeDetail   `json:"fee"`
	Memo     string      `json:"memo,omitempty"`
	Messages []MsgDetail `json:"messages"`
	// ABCI response fields (merged from block results).
	Success bool   `json:"success"`
	GasUsed int64  `json:"gas_used"`
	Log     string `json:"log,omitempty"`
	Error   string `json:"error,omitempty"`
}

// FeeDetail represents a transaction fee.
type FeeDetail struct {
	GasWanted int64  `json:"gas_wanted"`
	GasFee    string `json:"gas_fee"`
}

// MsgDetail holds a single message's type and decoded detail.
type MsgDetail struct {
	Type   string `json:"type"`
	Detail any    `json:"detail"`
}

// MsgAddPkgDetail holds decoded fields for vm.MsgAddPackage.
type MsgAddPkgDetail struct {
	Creator string   `json:"creator"`
	PkgPath string   `json:"pkg_path"`
	PkgName string   `json:"pkg_name"`
	Files   []string `json:"files"`
	Send    string   `json:"send,omitempty"`
}

// MsgCallDetail holds decoded fields for vm.MsgCall.
type MsgCallDetail struct {
	Caller  string   `json:"caller"`
	PkgPath string   `json:"pkg_path"`
	Func    string   `json:"func"`
	Args    []string `json:"args,omitempty"`
	Send    string   `json:"send,omitempty"`
}

// MsgRunDetail holds decoded fields for vm.MsgRun.
type MsgRunDetail struct {
	Caller string   `json:"caller"`
	Files  []string `json:"files"`
	Send   string   `json:"send,omitempty"`
}

// MsgSendDetail holds decoded fields for bank.MsgSend.
type MsgSendDetail struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
}

// DecodeTxSummary decodes raw transaction bytes into a lightweight summary.
func DecodeTxSummary(txBytes []byte, index int) (*TxSummary, error) {
	var tx std.Tx
	if err := amino.Unmarshal(txBytes, &tx); err != nil {
		return nil, fmt.Errorf("decode tx at index %d: %w", index, err)
	}

	summary := &TxSummary{
		Hash:      hex.EncodeToString(tmhash.Sum(txBytes)),
		GasWanted: tx.Fee.GasWanted,
	}

	if len(tx.Msgs) > 0 {
		summary.Type = msgTypeName(tx.Msgs[0])
		signers := tx.Msgs[0].GetSigners()
		if len(signers) > 0 {
			summary.Sender = signers[0].String()
		}
	}

	return summary, nil
}

// DecodeTxDetail fully decodes raw transaction bytes into a detailed view.
func DecodeTxDetail(txBytes []byte, height int64, index int) (*TxDetail, error) {
	var tx std.Tx
	if err := amino.Unmarshal(txBytes, &tx); err != nil {
		return nil, fmt.Errorf("decode tx at index %d: %w", index, err)
	}

	detail := &TxDetail{
		Hash:   hex.EncodeToString(tmhash.Sum(txBytes)),
		Height: height,
		Index:  index,
		Fee: FeeDetail{
			GasWanted: tx.Fee.GasWanted,
			GasFee:    formatCoin(tx.Fee.GasFee),
		},
		Memo:     tx.Memo,
		Messages: make([]MsgDetail, len(tx.Msgs)),
	}

	for i, msg := range tx.Msgs {
		detail.Messages[i] = decodeMsgDetail(msg)
	}

	return detail, nil
}

func decodeMsgDetail(msg std.Msg) MsgDetail {
	switch m := msg.(type) {
	case vm.MsgAddPackage:
		files := make([]string, 0, len(m.Package.Files))
		for _, f := range m.Package.Files {
			files = append(files, f.Name)
		}
		return MsgDetail{
			Type: "add_package",
			Detail: MsgAddPkgDetail{
				Creator: m.Creator.String(),
				PkgPath: m.Package.Path,
				PkgName: m.Package.Name,
				Files:   files,
				Send:    formatCoins(m.Send),
			},
		}
	case vm.MsgCall:
		return MsgDetail{
			Type: "call",
			Detail: MsgCallDetail{
				Caller:  m.Caller.String(),
				PkgPath: m.PkgPath,
				Func:    m.Func,
				Args:    m.Args,
				Send:    formatCoins(m.Send),
			},
		}
	case vm.MsgRun:
		files := make([]string, 0, len(m.Package.Files))
		for _, f := range m.Package.Files {
			files = append(files, f.Name)
		}
		return MsgDetail{
			Type: "run",
			Detail: MsgRunDetail{
				Caller: m.Caller.String(),
				Files:  files,
				Send:   formatCoins(m.Send),
			},
		}
	case bank.MsgSend:
		return MsgDetail{
			Type: "send",
			Detail: MsgSendDetail{
				From:   m.FromAddress.String(),
				To:     m.ToAddress.String(),
				Amount: formatCoins(m.Amount),
			},
		}
	default:
		// Fallback: use amino type URL and JSON representation.
		typeName := msgTypeName(msg)
		jsonBytes, err := amino.MarshalJSON(msg)
		if err != nil {
			return MsgDetail{Type: typeName, Detail: fmt.Sprintf("(unmarshalable: %v)", err)}
		}
		return MsgDetail{Type: typeName, Detail: string(jsonBytes)}
	}
}

// msgTypeName returns a short type name for a message.
func msgTypeName(msg std.Msg) string {
	url := amino.GetTypeURL(msg)
	if url != "" {
		// Type URLs look like "/vm.MsgCall" — strip the leading slash.
		return strings.TrimPrefix(url, "/")
	}
	return msg.Type()
}

// formatCoin formats a single coin as "1000ugnot".
func formatCoin(c std.Coin) string {
	return fmt.Sprintf("%d%s", c.Amount, c.Denom)
}

// formatCoins formats a coin set as "1000ugnot,500foo".
func formatCoins(coins std.Coins) string {
	if len(coins) == 0 {
		return ""
	}
	parts := make([]string, len(coins))
	for i, c := range coins {
		parts[i] = formatCoin(c)
	}
	return strings.Join(parts, ",")
}
