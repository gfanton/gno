package nodedata

import (
	"os"
	"testing"
)

func TestCompareBlocks_SameNode(t *testing.T) {
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

	// Compare the same node to itself — everything should match.
	result, err := CompareBlocks([]*DataDir{dd, dd}, ov.LatestHeight)
	if err != nil {
		t.Fatal("CompareBlocks:", err)
	}

	if result.Height != ov.LatestHeight {
		t.Errorf("expected height %d, got %d", ov.LatestHeight, result.Height)
	}
	if !result.AppHash.Match {
		t.Error("expected AppHash to match")
	}
	if !result.NumTxs.Match {
		t.Error("expected NumTxs to match")
	}
	if !result.ValidatorSetMatch {
		t.Error("expected ValidatorSetMatch to be true")
	}
	for i, td := range result.TxDiffs {
		if !td.Match {
			t.Errorf("expected tx %d to match", i)
		}
	}

	t.Logf("CompareBlocks: height=%d nodes=%d appHash_match=%v txDiffs=%d",
		result.Height, len(result.Nodes), result.AppHash.Match, len(result.TxDiffs))
}

func TestCompareBlocks_TooFewDirs(t *testing.T) {
	_, err := CompareBlocks(nil, 1)
	if err == nil {
		t.Fatal("expected error for nil dirs")
	}

	_, err = CompareBlocks([]*DataDir{{}}, 1)
	if err == nil {
		t.Fatal("expected error for single dir")
	}
}

func TestCompareBlocks_TooManyDirs(t *testing.T) {
	dirs := make([]*DataDir, 6)
	_, err := CompareBlocks(dirs, 1)
	if err == nil {
		t.Fatal("expected error for 6 dirs")
	}
}
