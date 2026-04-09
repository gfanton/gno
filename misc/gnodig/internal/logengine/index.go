package logengine

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
)

var gdxMagic = [4]byte{'G', 'D', 'X', '1'}

// BlockMeta holds per-block metadata in the index.
// Fixed 38 bytes per block on disk:
//
//	Offset(8) + Size(4) + TsMin(8) + TsMax(8) + LevelFlags(2) + Hash(8) = 38
//
// Fields are ordered for optimal struct alignment in memory.
// The on-disk serialization format is independent (see WriteTo / ReadIndex).
type BlockMeta struct {
	Offset     uint64
	TsMin      int64
	TsMax      int64
	Hash       uint64
	Size       uint32
	LevelFlags uint16
}

const blockMetaSize = 8 + 4 + 8 + 8 + 2 + 8 // 38 bytes

type Index struct {
	SourceSize int64
	Blocks     []BlockMeta
}

// WriteTo serializes the index to a .gdx file.
// Format: [magic 4B] [source_size 8B] [block_count 4B] [blocks...]
func (idx *Index) WriteTo(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create index %q: %w", path, err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// Header: magic(4) + source_size(8) + block_count(4) = 16 bytes.
	var hdr [16]byte
	copy(hdr[:4], gdxMagic[:])
	binary.LittleEndian.PutUint64(hdr[4:12], uint64(idx.SourceSize))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(len(idx.Blocks)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}

	// Blocks: reuse a single buffer for all blocks.
	var buf [blockMetaSize]byte
	for _, b := range idx.Blocks {
		binary.LittleEndian.PutUint64(buf[0:8], b.Offset)
		binary.LittleEndian.PutUint32(buf[8:12], b.Size)
		binary.LittleEndian.PutUint64(buf[12:20], uint64(b.TsMin))
		binary.LittleEndian.PutUint64(buf[20:28], uint64(b.TsMax))
		binary.LittleEndian.PutUint16(buf[28:30], b.LevelFlags)
		binary.LittleEndian.PutUint64(buf[30:38], b.Hash)
		if _, err := w.Write(buf[:]); err != nil {
			return err
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}
	return f.Sync()
}

func ReadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index %q: %w", path, err)
	}

	if len(data) < 16 {
		return nil, fmt.Errorf("index %q: file too short", path)
	}
	if [4]byte(data[:4]) != gdxMagic {
		return nil, fmt.Errorf("index %q: invalid magic", path)
	}

	sourceSize := int64(binary.LittleEndian.Uint64(data[4:12]))
	blockCount := binary.LittleEndian.Uint32(data[12:16])

	expected := 16 + int(blockCount)*blockMetaSize
	if len(data) < expected {
		return nil, fmt.Errorf("index %q: truncated (have %d, need %d)", path, len(data), expected)
	}

	blocks := make([]BlockMeta, blockCount)
	off := 16
	for i := range blocks {
		blocks[i].Offset = binary.LittleEndian.Uint64(data[off:])
		blocks[i].Size = binary.LittleEndian.Uint32(data[off+8:])
		blocks[i].TsMin = int64(binary.LittleEndian.Uint64(data[off+12:]))
		blocks[i].TsMax = int64(binary.LittleEndian.Uint64(data[off+20:]))
		blocks[i].LevelFlags = binary.LittleEndian.Uint16(data[off+28:])
		blocks[i].Hash = binary.LittleEndian.Uint64(data[off+30:])
		off += blockMetaSize
	}
	return &Index{SourceSize: sourceSize, Blocks: blocks}, nil
}

func (b *BlockMeta) OverlapsTimeRange(minNano, maxNano int64) bool {
	return b.TsMin <= maxNano && b.TsMax >= minNano
}
