package logengine

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type Cursor struct {
	mu     sync.Mutex
	reader io.ReaderAt
	idx    *Index
	offset int64
	buf    []byte
}

func NewCursor(r io.ReaderAt, idx *Index, offset int64) *Cursor {
	return &Cursor{reader: r, idx: idx, offset: offset, buf: make([]byte, 64*1024)}
}

func (c *Cursor) Read(n int) ([]LogEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var results []LogEntry
	readPos := c.offset // current file read position
	var partial []byte
	var partialStart int64 // file offset where partial begins
	nextOffset := c.offset // tracks the file offset of the next unprocessed byte

	for len(results) < n {
		nr, err := c.reader.ReadAt(c.buf, readPos)
		if nr == 0 {
			if err != nil && err != io.EOF {
				return results, fmt.Errorf("read at offset %d: %w", readPos, err)
			}
			break
		}

		// data begins at partialStart (or readPos if no carry).
		var data []byte
		var dataStart int64
		if len(partial) > 0 {
			data = append(partial, c.buf[:nr]...)
			dataStart = partialStart
			partial = nil
		} else {
			// Copy from reusable buffer since c.buf is overwritten each iteration.
			data = make([]byte, nr)
			copy(data, c.buf[:nr])
			dataStart = readPos
		}

		pos := dataStart // tracks the absolute file offset of data[0]
		for len(data) > 0 && len(results) < n {
			nl := bytes.IndexByte(data, '\n')
			if nl < 0 {
				if err == io.EOF {
					// Last line with no trailing newline.
					if len(data) > 0 {
						results = append(results, LogEntry{
							Line:      string(data),
							Offset:    pos,
							Timestamp: ExtractTimestampNano(data),
							Level:     ExtractLevel(data),
						})
						pos += int64(len(data))
					}
					data = nil
				} else {
					// Need more data; carry the remainder forward.
					partial = make([]byte, len(data))
					copy(partial, data)
					partialStart = pos
					data = nil
				}
				break
			}

			line := data[:nl]
			lineStart := pos
			pos += int64(nl) + 1
			data = data[nl+1:]

			if len(line) == 0 {
				continue
			}

			results = append(results, LogEntry{
				Line:      string(line),
				Offset:    lineStart,
				Timestamp: ExtractTimestampNano(line),
				Level:     ExtractLevel(line),
			})
		}

		// pos now points to the first unprocessed byte in data (or end of data).
		// If we exited early (got enough entries) and data still has content,
		// the cursor should resume from pos, not from readPos+nr.
		if len(partial) > 0 {
			nextOffset = partialStart
		} else {
			nextOffset = pos
		}

		readPos += int64(nr)

		if err == io.EOF {
			break
		}
	}

	c.offset = nextOffset
	return results, nil
}

func (c *Cursor) SeekOffset(offset int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset = offset
}

// SeekTime sets the cursor to the first block whose TsMax >= targetNano.
// Uses a linear scan because blocks are ordered by file offset, not timestamp;
// log files with concurrent writers may not have monotonically increasing times.
func (c *Cursor) SeekTime(targetNano int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, b := range c.idx.Blocks {
		if b.TsMax >= targetNano {
			c.offset = int64(b.Offset)
			return
		}
	}
	// No matching block found — leave offset unchanged.
}

func (c *Cursor) Offset() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset
}
