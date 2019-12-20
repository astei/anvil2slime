package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
)

const anvilMaxOffsets = 1024
const anvilSectorSize = 4096

var ErrNoChunk = errors.New("anvil: chunk not found")
var ErrInvalidChunkLength = errors.New("anvil: invalid chunk length")
var ErrInvalidCompression = errors.New("anvil: invalid compression format")

type AnvilCompressionLevel byte

const (
	AnvilCompressionLevelGzip    AnvilCompressionLevel = 1
	AnvilCompressionLevelDeflate                       = 2
)

// Struct AnvilReader allows you to read an Anvil region file and extract its components. The reader is not safe for
// concurrent access; usage should be protected by a mutex if concurrent access is desired.
type AnvilReader struct {
	source      io.ReadSeeker
	sectorTable []int32
	Name        string
}

// Creates an AnvilReader. The ownership of the source is transferred to this reader.
func NewAnvilReader(source io.ReadSeeker) (reader *AnvilReader, err error) {
	reader = &AnvilReader{
		source:      source,
		sectorTable: make([]int32, anvilMaxOffsets),
	}

	if file, ok := source.(*os.File); ok {
		reader.Name = file.Name()
	}
	err = reader.readSectorTable()
	return
}

func (world *AnvilReader) readSectorTable() (err error) {
	_, err = world.source.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	rawSectorData := make([]byte, anvilSectorSize)
	_, err = io.ReadFull(world.source, rawSectorData)
	if err != nil {
		return err
	}

	rawSectorIn := bytes.NewReader(rawSectorData)
	err = binary.Read(rawSectorIn, binary.BigEndian, world.sectorTable)
	return
}

// ReadChunk reads an Anvil chunk at the specified X and Z coordinates. Note that these coordinates are relative to the
// region file and are not chunk coordinates. If successful, the provided reader may be provided to an NBT deserialization
// routine.
func (world *AnvilReader) ReadChunk(x, z int) (chunk io.Reader, err error) {
	offset := world.sectorTable[x+z*32]

	sectorNumber := offset >> 8
	occupiedSectors := offset & 0xff
	if sectorNumber == 0 {
		err = ErrNoChunk
		return
	}

	if _, err = world.source.Seek(int64(sectorNumber*anvilSectorSize), io.SeekStart); err != nil {
		return
	}

	sectorData := make([]byte, occupiedSectors*anvilSectorSize)
	if _, err = io.ReadFull(world.source, sectorData); err != nil {
		return
	}

	sectorReader := bytes.NewReader(sectorData)
	var sectorHeader struct {
		Length      int32
		Compression AnvilCompressionLevel
	}
	if err = binary.Read(sectorReader, binary.BigEndian, &sectorHeader); err != nil {
		return
	}

	if sectorHeader.Length > int32(len(sectorData)-5) {
		return nil, ErrInvalidChunkLength
	}

	chunkStream := io.LimitReader(sectorReader, int64(sectorHeader.Length))
	switch sectorHeader.Compression {
	case AnvilCompressionLevelGzip:
		return gzip.NewReader(chunkStream)
	case AnvilCompressionLevelDeflate:
		return zlib.NewReader(chunkStream)
	default:
		return nil, ErrInvalidCompression
	}
}

func (world *AnvilReader) ChunkExists(x, z int) bool {
	return world.sectorTable[x+z*32] != 0
}

func (world *AnvilReader) Close() error {
	if closer, ok := world.source.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
