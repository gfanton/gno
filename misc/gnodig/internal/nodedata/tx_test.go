package nodedata

import (
	"os"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/crypto/tmhash"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"
)

func buildTxBytes(t *testing.T, msgs []std.Msg, fee std.Fee) []byte {
	t.Helper()
	tx := std.Tx{Msgs: msgs, Fee: fee, Memo: "test memo"}
	bz, err := amino.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal tx: %v", err)
	}
	return bz
}

func TestDecodeTxSummary(t *testing.T) {
	caller := crypto.AddressFromPreimage([]byte("caller"))
	fee := std.Fee{GasWanted: 100000, GasFee: std.NewCoin("ugnot", 5000)}

	tests := []struct {
		name       string
		msgs       []std.Msg
		wantType   string
		wantSender string
	}{
		{
			name: "MsgCall",
			msgs: []std.Msg{vm.MsgCall{
				Caller:  caller,
				PkgPath: "gno.land/r/demo/boards",
				Func:    "CreateThread",
				Args:    []string{"1", "title", "body"},
			}},
			wantType:   "vm.m_call",
			wantSender: caller.String(),
		},
		{
			name: "MsgAddPackage",
			msgs: []std.Msg{vm.MsgAddPackage{
				Creator: caller,
				Package: &std.MemPackage{
					Name: "mypkg",
					Path: "gno.land/r/demo/mypkg",
					Files: []*std.MemFile{
						{Name: "mypkg.gno", Body: "package mypkg"},
					},
				},
			}},
			wantType:   "vm.m_addpkg",
			wantSender: caller.String(),
		},
		{
			name: "MsgSend",
			msgs: []std.Msg{bank.MsgSend{
				FromAddress: caller,
				ToAddress:   crypto.AddressFromPreimage([]byte("recipient")),
				Amount:      std.Coins{std.NewCoin("ugnot", 10000)},
			}},
			wantType:   "bank.MsgSend",
			wantSender: caller.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txBytes := buildTxBytes(t, tt.msgs, fee)

			summary, err := DecodeTxSummary(txBytes, 0)
			if err != nil {
				t.Fatalf("DecodeTxSummary: %v", err)
			}

			if summary.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", summary.Type, tt.wantType)
			}
			if summary.Sender != tt.wantSender {
				t.Errorf("Sender = %q, want %q", summary.Sender, tt.wantSender)
			}
			if summary.GasWanted != fee.GasWanted {
				t.Errorf("GasWanted = %d, want %d", summary.GasWanted, fee.GasWanted)
			}

			// Hash should match tmhash of the raw bytes.
			wantHash := tmhash.Sum(txBytes)
			if len(summary.Hash) != len(wantHash)*2 {
				t.Errorf("Hash length = %d, want %d hex chars", len(summary.Hash), len(wantHash)*2)
			}
		})
	}
}

func TestDecodeTxDetail(t *testing.T) {
	caller := crypto.AddressFromPreimage([]byte("caller"))
	recipient := crypto.AddressFromPreimage([]byte("recipient"))
	fee := std.Fee{GasWanted: 200000, GasFee: std.NewCoin("ugnot", 10000)}

	msgs := []std.Msg{
		vm.MsgCall{
			Caller:  caller,
			PkgPath: "gno.land/r/demo/boards",
			Func:    "CreateThread",
			Args:    []string{"1", "hello", "world"},
			Send:    std.Coins{std.NewCoin("ugnot", 500)},
		},
		bank.MsgSend{
			FromAddress: caller,
			ToAddress:   recipient,
			Amount:      std.Coins{std.NewCoin("ugnot", 1000)},
		},
	}

	txBytes := buildTxBytes(t, msgs, fee)

	detail, err := DecodeTxDetail(txBytes, 42, 3)
	if err != nil {
		t.Fatalf("DecodeTxDetail: %v", err)
	}

	if detail.Height != 42 {
		t.Errorf("Height = %d, want 42", detail.Height)
	}
	if detail.Index != 3 {
		t.Errorf("Index = %d, want 3", detail.Index)
	}
	if detail.Fee.GasWanted != 200000 {
		t.Errorf("Fee.GasWanted = %d, want 200000", detail.Fee.GasWanted)
	}
	if detail.Fee.GasFee != "10000ugnot" {
		t.Errorf("Fee.GasFee = %q, want %q", detail.Fee.GasFee, "10000ugnot")
	}
	if detail.Memo != "test memo" {
		t.Errorf("Memo = %q, want %q", detail.Memo, "test memo")
	}
	if len(detail.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(detail.Messages))
	}

	// Check first message (MsgCall).
	msg0 := detail.Messages[0]
	if msg0.Type != "call" {
		t.Errorf("Messages[0].Type = %q, want %q", msg0.Type, "call")
	}
	callDetail, ok := msg0.Detail.(MsgCallDetail)
	if !ok {
		t.Fatalf("Messages[0].Detail type = %T, want MsgCallDetail", msg0.Detail)
	}
	if callDetail.Func != "CreateThread" {
		t.Errorf("callDetail.Func = %q, want %q", callDetail.Func, "CreateThread")
	}
	if callDetail.PkgPath != "gno.land/r/demo/boards" {
		t.Errorf("callDetail.PkgPath = %q, want %q", callDetail.PkgPath, "gno.land/r/demo/boards")
	}
	if callDetail.Send != "500ugnot" {
		t.Errorf("callDetail.Send = %q, want %q", callDetail.Send, "500ugnot")
	}

	// Check second message (MsgSend).
	msg1 := detail.Messages[1]
	if msg1.Type != "send" {
		t.Errorf("Messages[1].Type = %q, want %q", msg1.Type, "send")
	}
	sendDetail, ok := msg1.Detail.(MsgSendDetail)
	if !ok {
		t.Fatalf("Messages[1].Detail type = %T, want MsgSendDetail", msg1.Detail)
	}
	if sendDetail.Amount != "1000ugnot" {
		t.Errorf("sendDetail.Amount = %q, want %q", sendDetail.Amount, "1000ugnot")
	}
}

func TestDecodeTxSummary_InvalidBytes(t *testing.T) {
	_, err := DecodeTxSummary([]byte("not a valid tx"), 0)
	if err == nil {
		t.Fatal("expected error for invalid tx bytes")
	}
}

func TestTxByIndex_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Skip("cannot open data dir (may be locked):", err)
	}
	defer dd.Close()

	// Scan backward from tip to find a block with transactions.
	tip := dd.BlockStore().Height()
	var foundHeight int64
	for h := tip; h >= 1 && h > tip-1000; h-- {
		detail, err := dd.Block(h)
		if err != nil {
			continue
		}
		if detail.NumTxs > 0 {
			foundHeight = h
			break
		}
	}
	if foundHeight == 0 {
		t.Skip("no block with transactions found in last 1000 blocks")
	}

	detail, err := dd.TxByIndex(foundHeight, 0)
	if err != nil {
		t.Fatalf("TxByIndex(%d, 0): %v", foundHeight, err)
	}

	if detail.Height != foundHeight {
		t.Errorf("Height = %d, want %d", detail.Height, foundHeight)
	}
	if detail.Hash == "" {
		t.Error("expected non-empty Hash")
	}
	if len(detail.Messages) == 0 {
		t.Error("expected at least one message")
	}

	t.Logf("Tx at height %d index 0: hash=%s type=%s msgs=%d success=%v",
		detail.Height, detail.Hash[:16]+"...", detail.Messages[0].Type, len(detail.Messages), detail.Success)
}
