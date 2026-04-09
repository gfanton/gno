package nodedata

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/bft/wal"

	// Blank import triggers amino type registration for consensus WAL
	// message types (newRoundStepInfo, msgInfo, timeoutInfo).
	_ "github.com/gnolang/gno/tm2/pkg/bft/consensus"
)

// walMaxMsgSize matches the consensus maxMsgSize (unexported).
const walMaxMsgSize = 1048576 // 1MB

// WALDetail holds decoded WAL entries for a single height.
type WALDetail struct {
	Height   int64        `json:"height"`
	Messages []WALMessage `json:"messages"`
	Warnings []string     `json:"warnings,omitempty"`
}

// WALMessage is a single decoded WAL entry.
type WALMessage struct {
	Time string          `json:"time"`
	Type string          `json:"type"` // amino type URL or Go type name
	Data json.RawMessage `json:"data"` // amino JSON encoding of the message
}

// WALSummary returns a per-round digest of consensus activity at the
// given height, aggregated from the raw WAL messages.
func (d *DataDir) WALSummary(height int64) (*WALSummary, error) {
	detail, err := d.WALMessages(height)
	if err != nil {
		return nil, err
	}
	return SummarizeWAL(height, detail.Messages), nil
}

// WALFiltered returns WAL messages for the given height, filtered by
// round and/or message type. A nil round means "any round". An empty
// msgType means "any type". If limit > 0, at most limit messages are
// returned.
func (d *DataDir) WALFiltered(height int64, round *int, msgType string, limit int) (*WALDetail, error) {
	detail, err := d.WALMessages(height)
	if err != nil {
		return nil, err
	}

	filtered := FilterWALMessages(detail.Messages, round, msgType, height)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	// Slim down messages to remove heavy fields (signatures, proofs, block part bytes)
	// that blow up context without providing investigative value.
	slimmed := slimWALMessages(filtered)

	return &WALDetail{
		Height:   height,
		Messages: slimmed,
		Warnings: detail.Warnings,
	}, nil
}

// WALMessages reads all WAL entries for the given height. It opens WAL
// files read-only and scans for MetaMessage height markers, collecting
// messages between the target height marker and the next one.
func (d *DataDir) WALMessages(height int64) (*WALDetail, error) {
	walDir := filepath.Join(d.path, "wal", "cs.wal")

	files, err := listWALFiles(walDir)
	if err != nil {
		return nil, fmt.Errorf("list WAL files: %w", err)
	}

	// Scan backwards from the newest file; recent heights are most likely
	// near the end.
	var warnings []string
	for i := len(files) - 1; i >= 0; i-- {
		msgs, found, err := scanWALFile(files[i], height)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip %s: %v", filepath.Base(files[i]), err))
			continue
		}
		if found {
			return &WALDetail{Height: height, Messages: msgs, Warnings: warnings}, nil
		}
	}

	return nil, fmt.Errorf("no WAL entries found for height %d", height)
}

// listWALFiles returns WAL segment paths sorted oldest-first.
// The head file ("wal") sorts last (newest). Numbered segments
// ("wal.7878", "wal.7879", ...) sort by their numeric suffix.
func listWALFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type walFile struct {
		path  string
		index int // -1 for the head file (newest)
	}

	var wfs []walFile
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "wal") {
			continue
		}

		wf := walFile{path: filepath.Join(dir, name)}

		if name == "wal" {
			wf.index = -1 // sentinel: sorts after all numbered files
		} else if rest, ok := strings.CutPrefix(name, "wal."); ok {
			idx, err := strconv.Atoi(rest)
			if err != nil {
				continue // skip unexpected filenames
			}
			wf.index = idx
		} else {
			continue
		}

		wfs = append(wfs, wf)
	}

	// Sort: numbered files ascending, head file last.
	slices.SortFunc(wfs, func(a, b walFile) int {
		if a.index == -1 {
			return 1 // head always after numbered
		}
		if b.index == -1 {
			return -1
		}
		return cmp.Compare(a.index, b.index)
	})

	paths := make([]string, len(wfs))
	for i, wf := range wfs {
		paths[i] = wf.path
	}
	return paths, nil
}

// scanWALMaxHeight returns the highest height seen across all WAL files.
func scanWALMaxHeight(walDir string) (int64, error) {
	files, err := listWALFiles(walDir)
	if err != nil {
		return 0, err
	}

	var maxHeight int64
	// Scan from newest file backward, stop early.
	for i := len(files) - 1; i >= 0; i-- {
		h, err := scanFileMaxHeight(files[i])
		if err != nil {
			continue
		}
		if h > maxHeight {
			maxHeight = h
			break // newest file has the highest height
		}
	}
	return maxHeight, nil
}

// scanFileMaxHeight scans a single WAL file for the maximum MetaMessage height.
func scanFileMaxHeight(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := wal.NewWALReader(f, walMaxMsgSize)
	var maxHeight int64

	for {
		_, meta, err := reader.ReadMessage()
		if err != nil {
			break
		}
		if meta != nil && meta.Height > maxHeight {
			maxHeight = meta.Height
		}
	}
	return maxHeight, nil
}

// scanWALFile opens a single WAL segment read-only and collects all
// messages belonging to targetHeight. It tracks the current height via
// MetaMessage markers.
func scanWALFile(path string, targetHeight int64) ([]WALMessage, bool, error) {
	f, err := os.Open(path) // read-only
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	reader := wal.NewWALReader(f, walMaxMsgSize)

	var messages []WALMessage
	var currentHeight int64
	found := false

	for {
		msg, meta, err := reader.ReadMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			// Skip corrupt entries and keep reading.
			continue
		}

		if meta != nil {
			currentHeight = meta.Height
			if found && currentHeight > targetHeight {
				break // moved past our target height, done
			}
			continue
		}

		if msg == nil || currentHeight != targetHeight {
			continue
		}

		found = true

		// Derive the amino type URL; fall back to the Go type name.
		typeName := amino.GetTypeURL(msg.Msg)
		if typeName == "" {
			typeName = fmt.Sprintf("%T", msg.Msg)
		}

		jsonData, merr := amino.MarshalJSON(msg.Msg)
		if merr != nil {
			jsonData = []byte(fmt.Sprintf(`{"error":%q}`, merr.Error()))
		}

		messages = append(messages, WALMessage{
			Time: msg.Time.UTC().Format("2006-01-02T15:04:05.000Z"),
			Type: typeName,
			Data: json.RawMessage(jsonData),
		})
	}

	return messages, found, nil
}
