package nodedata

import (
	"os"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/crypto"
)

func TestOpen_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	ov, err := dd.Overview()
	if err != nil {
		t.Fatal("Overview:", err)
	}

	if ov.ChainID == "" {
		t.Error("expected non-empty ChainID")
	}
	if ov.LatestHeight <= 0 {
		t.Errorf("expected positive LatestHeight, got %d", ov.LatestHeight)
	}
	if ov.LatestBlockTime == "" {
		t.Error("expected non-empty LatestBlockTime")
	}
	if ov.BlockStoreHeight <= 0 {
		t.Errorf("expected positive BlockStoreHeight, got %d", ov.BlockStoreHeight)
	}

	t.Logf("Overview: %+v", ov)
}

func TestBlock_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	ov, err := dd.Overview()
	if err != nil {
		t.Fatal("Overview:", err)
	}

	detail, err := dd.Block(ov.LatestHeight)
	if err != nil {
		t.Fatal("Block:", err)
	}

	if detail.Height != ov.LatestHeight {
		t.Errorf("expected height %d, got %d", ov.LatestHeight, detail.Height)
	}
	if detail.NumTxs < 0 {
		t.Errorf("expected non-negative NumTxs, got %d", detail.NumTxs)
	}
	if detail.ChainID == "" {
		t.Error("expected non-empty ChainID")
	}
	if detail.Proposer == "" {
		t.Error("expected non-empty Proposer")
	}

	t.Logf("Block %d: chain=%s txs=%d proposer=%s validators=%d",
		detail.Height, detail.ChainID, detail.NumTxs, detail.Proposer, len(detail.Validators))
}

func TestBlock_NotFound(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	// Height far beyond any stored block should return an error.
	_, err = dd.Block(999_999_999)
	if err == nil {
		t.Fatal("expected error for nonexistent block, got nil")
	}
	t.Log("expected error:", err)
}

func TestWAL_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	ov, err := dd.Overview()
	if err != nil {
		t.Fatal("Overview:", err)
	}

	// Try a height near the latest that should still be in the WAL.
	height := ov.LatestHeight - 5
	if height < 1 {
		height = 1
	}

	detail, err := dd.WALMessages(height)
	if err != nil {
		t.Logf("WAL read failed (may be expected if height not in WAL): %v", err)
		t.Skip("WAL data not available for height", height)
	}

	if detail.Height != height {
		t.Errorf("expected height %d, got %d", height, detail.Height)
	}

	t.Logf("WAL height %d: %d messages", detail.Height, len(detail.Messages))
	for i, m := range detail.Messages {
		if i > 5 {
			break
		}
		data := string(m.Data)
		if len(data) > 200 {
			data = data[:200] + "..."
		}
		t.Logf("  %s [%s] %s", m.Time, m.Type, data)
	}
}

func TestAccountQuery_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	// Query a well-known genesis address (test1).
	result, err := dd.AccountQuery(0, "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")
	if err != nil {
		t.Fatal("AccountQuery:", err)
	}

	t.Logf("Account: height=%d exists=%v coins=%v seq=%d",
		result.Height, result.Exists, result.Coins, result.Sequence)

	if !result.Exists {
		t.Error("expected test1 account to exist")
	}
	if result.Height <= 0 {
		t.Errorf("expected positive height, got %d", result.Height)
	}
}

func TestPackageQuery_RealData(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	// Query a well-known stdlib package.
	result, err := dd.PackageQuery(0, "bufio")
	if err != nil {
		t.Fatal("PackageQuery:", err)
	}

	t.Logf("Package: height=%d path=%s exists=%v files=%d",
		result.Height, result.Path, result.Exists, result.NumFiles)

	if !result.Exists {
		t.Error("expected bufio package to exist")
	}
	if result.NumFiles <= 0 {
		t.Errorf("expected positive file count, got %d", result.NumFiles)
	}
	for _, f := range result.Files {
		t.Logf("  %s (%d bytes)", f.Name, f.Size)
	}

	// Query a non-existent package.
	noResult, err := dd.PackageQuery(0, "gno.land/r/nonexistent/pkg")
	if err != nil {
		t.Fatal("PackageQuery(nonexistent):", err)
	}
	if noResult.Exists {
		t.Error("expected non-existent package")
	}
}

func TestAccountQuery_NonExistent(t *testing.T) {
	const dataDir = "../../.data/gnoland-data"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Skip("test data directory not found:", dataDir)
	}

	dd, err := Open(dataDir)
	if err != nil {
		t.Fatal("Open:", err)
	}
	defer dd.Close()

	// Use an address generated from a zero address. AddressToBech32
	// produces a valid bech32 string.
	addr := crypto.AddressToBech32(crypto.Address{})
	result, err := dd.AccountQuery(0, addr)
	if err != nil {
		t.Fatal("AccountQuery:", err)
	}

	if result.Exists {
		t.Error("expected non-existent account")
	}
}

func TestOpen_InvalidDir(t *testing.T) {
	_, err := Open("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}
