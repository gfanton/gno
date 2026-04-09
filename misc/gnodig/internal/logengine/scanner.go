package logengine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sync/errgroup"

	"github.com/gnolang/gno/misc/gnodig/internal/driver"
)

// ScanConfig controls the parallel index-building pipeline.
type ScanConfig struct {
	BlockSize   int // bytes per read; segments split at newline boundaries
	Concurrency int // number of worker goroutines
}

func (c *ScanConfig) blockSize() int {
	if c.BlockSize > 0 {
		return c.BlockSize
	}
	return 256 * 1024
}

func (c *ScanConfig) concurrency() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return runtime.NumCPU()
}

// segment is a newline-aligned slice of the source file, ready for metadata
// extraction. offset is the absolute byte position in the source file.
type segment struct {
	seq    int
	offset int64
	data   []byte
}

// BuildIndex reads src in BlockSize chunks, splits at newline boundaries,
// dispatches segments to worker goroutines for metadata extraction, and
// assembles the results into an ordered Index.
func BuildIndex(ctx context.Context, src driver.LogSource, cfg ScanConfig) (*Index, error) {
	r, size, err := src.Reader(ctx)
	if err != nil {
		return nil, fmt.Errorf("open reader: %w", err)
	}

	if size == 0 {
		return &Index{SourceSize: 0, Blocks: nil}, nil
	}

	blockSize := cfg.blockSize()
	workers := cfg.concurrency()

	segments := make(chan segment, workers*2)
	g, ctx := errgroup.WithContext(ctx)

	// ---- Reader goroutine
	g.Go(func() error {
		defer close(segments)
		var (
			filePos    int64
			seq        int
			carry      []byte
			carryStart int64
		)

		readBuf := make([]byte, blockSize)

		for filePos < size {
			readSize := min(int64(blockSize), size-filePos)

			n, readErr := r.ReadAt(readBuf[:readSize], filePos)
			if n == 0 {
				if readErr != nil && readErr != io.EOF {
					return fmt.Errorf("read at offset %d: %w", filePos, readErr)
				}
				break
			}
			buf := readBuf[:n]

			// Prepend any data carried from the previous iteration.
			var segStart int64
			if len(carry) > 0 {
				buf = append(carry, buf...)
				segStart = carryStart
				carry = nil
			} else {
				segStart = filePos
			}

			lastNL := bytes.LastIndexByte(buf, '\n')
			if lastNL == -1 {
				// No newline found — carry the entire buffer forward.
				carry = make([]byte, len(buf))
				copy(carry, buf)
				carryStart = segStart
				filePos += int64(n)
				continue
			}

			// Split: everything after the last newline becomes carry.
			if lastNL < len(buf)-1 {
				carry = make([]byte, len(buf)-lastNL-1)
				copy(carry, buf[lastNL+1:])
				carryStart = segStart + int64(lastNL+1)
			}
			// Copy segment data since readBuf is reused next iteration.
			segData := make([]byte, lastNL+1)
			copy(segData, buf[:lastNL+1])

			select {
			case <-ctx.Done():
				return ctx.Err()
			case segments <- segment{seq: seq, offset: segStart, data: segData}:
			}

			filePos += int64(n)
			seq++
		}

		// Flush any remaining carry (file without trailing newline).
		if len(carry) > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case segments <- segment{seq: seq, offset: carryStart, data: carry}:
			}
		}

		return nil
	})

	// ---- Workers
	type result struct {
		seq  int
		meta BlockMeta
	}
	results := make(chan result, workers*2)

	for range workers {
		g.Go(func() error {
			for seg := range segments {
				meta := processSegment(seg)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- result{seq: seg.seq, meta: meta}:
				}
			}
			return nil
		})
	}

	// Close results when all goroutines (reader + workers) finish.
	go func() {
		g.Wait()
		close(results)
	}()

	// ---- Collector
	collected := make(map[int]BlockMeta)
	for r := range results {
		collected[r.seq] = r.meta
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	blocks := make([]BlockMeta, len(collected))
	for i := range blocks {
		b, ok := collected[i]
		if !ok {
			return nil, fmt.Errorf("missing segment %d", i)
		}
		blocks[i] = b
	}

	return &Index{SourceSize: size, Blocks: blocks}, nil
}

// processSegment extracts metadata from a single segment's lines.
func processSegment(seg segment) BlockMeta {
	h := xxhash.New()
	h.Write(seg.data)

	meta := BlockMeta{
		Offset: uint64(seg.offset),
		Size:   uint32(len(seg.data)),
		TsMin:  1<<63 - 1,
		TsMax:  -(1 << 63),
		Hash:   h.Sum64(),
	}

	remaining := seg.data
	for len(remaining) > 0 {
		nl := bytes.IndexByte(remaining, '\n')
		var line []byte
		if nl >= 0 {
			line = remaining[:nl]
			remaining = remaining[nl+1:]
		} else {
			line = remaining
			remaining = nil
		}

		if len(line) == 0 {
			continue
		}

		ts := ExtractTimestampNano(line)
		if ts != 0 {
			if ts < meta.TsMin {
				meta.TsMin = ts
			}
			if ts > meta.TsMax {
				meta.TsMax = ts
			}
		}

		lvlStr := ExtractLevel(line)
		lvl := ParseLevel(lvlStr)
		if lvl != 0 {
			meta.LevelFlags |= lvl
		}
	}

	// Reset sentinels when no valid timestamps were found.
	if meta.TsMin == 1<<63-1 {
		meta.TsMin = 0
		meta.TsMax = 0
	}

	return meta
}
