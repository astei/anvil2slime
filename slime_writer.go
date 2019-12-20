package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/astei/anvil2slime/nbt"
	"github.com/klauspost/compress/zstd"
	"io"
	"math"
	"math/big"
	"sort"
)

const slimeHeader = 0xB10B
const slimeLatestVersion = 3

func slimeChunkKey(coord ChunkCoord) int64 {
	return (int64(coord.Z) * 0x7fffffff) + int64(coord.X)
}

func (world *AnvilWorld) WriteAsSlime(writer io.Writer) error {
	slimeWriter := &slimeWriter{writer: writer, world: world}
	return slimeWriter.writeWorld()
}

type slimeWriter struct {
	writer io.Writer
	world  *AnvilWorld
}

func (w *slimeWriter) writeWorld() (err error) {
	if err = w.writeHeader(); err != nil {
		return
	}
	if err = w.writeChunks(); err != nil {
		return
	}
	if err = w.writeTileEntities(); err != nil {
		return
	}
	if err = w.writeEntities(); err != nil {
		return
	}
	if err = w.writeExtra(); err != nil {
		return
	}

	return
}

func (w *slimeWriter) writeHeader() (err error) {
	minChunkXZ, width, depth := w.determineChunkBounds()
	used := w.createChunkBitset(width, depth, minChunkXZ)
	usedAsBytes := padBitSetByteArrayOutput(used, depth*width)

	var header struct {
		Magic   uint16
		Version uint8
		MinX    int16
		MinZ    int16
		Width   uint16
		Depth   uint16
	}
	header.Magic = slimeHeader
	header.Version = slimeLatestVersion
	header.MinX = int16(minChunkXZ.X)
	header.MinZ = int16(minChunkXZ.Z)
	header.Width = uint16(width)
	header.Depth = uint16(depth)

	if err = binary.Write(w.writer, binary.BigEndian, header); err != nil {
		return
	}
	_, err = w.writer.Write(usedAsBytes)
	return
}

func padBitSetByteArrayOutput(set *big.Int, expectedBits int) []byte {
	expectedBitSetSize := int(math.Ceil(float64(expectedBits) / float64(8)))
	usedAsBytes := set.Bytes()
	if len(usedAsBytes) < expectedBitSetSize {
		usedAsBytes = append(usedAsBytes, make([]byte, expectedBitSetSize-len(usedAsBytes))...)
	}
	return usedAsBytes
}

func (w *slimeWriter) createChunkBitset(width int, depth int, minChunkXZ ChunkCoord) *big.Int {
	slimeSorted := w.world.getChunkKeys()
	var populatedChunks big.Int
	for _, currentChunk := range slimeSorted {
		relZ := currentChunk.Z - minChunkXZ.Z
		relX := currentChunk.X - minChunkXZ.X
		populatedChunks.SetBit(&populatedChunks, relZ*width+relX, 1)
	}

	return &populatedChunks
}

func (w *slimeWriter) determineChunkBounds() (minChunkXZ ChunkCoord, width int, depth int) {
	// Slime uses its own order for chunks, but still requires us to determine the maximum/minimum XZ coordinates.
	minX, maxX, minZ, maxZ := w.world.getMinXZ()

	width = maxX - minX + 1
	depth = maxZ - minZ + 1
	return ChunkCoord{X: minX, Z: minZ}, width, depth
}

func (w *slimeWriter) writeChunks() (err error) {
	var out bytes.Buffer

	slimeSorted := w.world.getChunkKeys()
	minChunkXZ, width, _ := w.determineChunkBounds()
	barf := func(c ChunkCoord) int {
		relZ := c.Z - minChunkXZ.Z
		relX := c.X - minChunkXZ.X
		return relZ*width + relX
	}
	sort.Slice(slimeSorted, func(one, two int) bool {
		k1 := barf(slimeSorted[one])
		k2 := barf(slimeSorted[two])
		return k1 < k2
	})

	fmt.Println("KEYS SORTED:", slimeSorted)

	for _, coord := range slimeSorted {
		chunk := w.world.chunks[coord]
		if err = w.writeChunkHeader(chunk, out); err != nil {
			return
		}
		for _, section := range chunk.Sections {
			if err = w.writeChunkSection(chunk, section, &out); err != nil {
				return
			}
		}
	}

	return w.writeZstdCompressed(out)
}

func (w *slimeWriter) writeChunkSection(chunk MinecraftChunk, section MinecraftChunkSection, out io.Writer) (err error) {
	fmt.Println(chunk.X, ",", chunk.Z, "has section", section.Y)
	if _, err = out.Write(section.BlockLight); err != nil {
		return
	}
	if _, err = out.Write(section.Blocks); err != nil {
		return
	}
	if _, err = out.Write(section.BlockData); err != nil {
		return
	}
	if _, err = out.Write(section.SkyLight); err != nil {
		return
	}
	if err = binary.Write(out, binary.BigEndian, uint16(0)); err != nil {
		return
	}
	return
}

func (w *slimeWriter) writeChunkHeader(chunk MinecraftChunk, out bytes.Buffer) (err error) {
	for _, heightEntry := range chunk.HeightMap {
		if err = binary.Write(&out, binary.BigEndian, uint32(heightEntry)); err != nil {
			return
		}
	}
	if _, err = out.Write(chunk.Biomes); err != nil {
		return
	}

	var chunkSectionsPopulated uint16
	for _, section := range chunk.Sections {
		chunkSectionsPopulated |= 1 << section.Y
	}

	if err = binary.Write(&out, binary.BigEndian, chunkSectionsPopulated); err != nil {
		return
	}

	fmt.Println(chunk.X, ",", chunk.Z, "has", len(chunk.Sections), "sections. Raw bitmask:", chunkSectionsPopulated)
	return
}

func (w *slimeWriter) writeZstdCompressed(buf bytes.Buffer) (err error) {
	uncompressedSize := buf.Len()

	var compressedOutput bytes.Buffer
	zstdWriter, _ := zstd.NewWriter(&compressedOutput)
	if _, err = buf.WriteTo(zstdWriter); err != nil {
		return
	}
	if err = zstdWriter.Close(); err != nil {
		return
	}

	fmt.Printf("compressed %d bytes, uncompressed %d\n", compressedOutput.Len(), uncompressedSize)

	if err = binary.Write(w.writer, binary.BigEndian, uint32(compressedOutput.Len())); err != nil {
		return
	}
	if err = binary.Write(w.writer, binary.BigEndian, uint32(uncompressedSize)); err != nil {
		return
	}
	_, err = compressedOutput.WriteTo(w.writer)
	return
}

func (w *slimeWriter) writeTileEntities() (err error) {
	var tileEntities []interface{}
	for _, chunk := range w.world.chunks {
		tileEntities = append(tileEntities, chunk.TileEntities...)
	}

	var compound struct {
		Tiles []interface{} `nbt:"tiles"`
	}
	compound.Tiles = tileEntities

	var buf bytes.Buffer
	if err = nbt.NewEncoder(&buf).Encode(compound); err != nil {
		return
	}
	return w.writeZstdCompressed(buf)
}

func (w *slimeWriter) writeEntities() (err error) {
	var entities []interface{}
	for _, chunk := range w.world.chunks {
		entities = append(entities, chunk.Entities...)
	}

	var compound struct {
		Entities []interface{} `nbt:"entities"`
	}
	compound.Entities = entities

	var buf bytes.Buffer
	if err = nbt.NewEncoder(&buf).Encode(compound); err != nil {
		return
	}

	if _, err = w.writer.Write([]byte{1}); err != nil {
		return
	}
	return w.writeZstdCompressed(buf)
}

func (w *slimeWriter) writeExtra() (err error) {
	// Write an empty zstandard stream
	var empty bytes.Buffer
	return w.writeZstdCompressed(empty)
}

func (world *AnvilWorld) getChunkKeys() []ChunkCoord {
	var keys []ChunkCoord
	for coord := range world.chunks {
		keys = append(keys, coord)
	}
	return keys
}

func (world *AnvilWorld) getMinXZ() (minX int, maxX int, minZ int, maxZ int) {
	keys := world.getChunkKeys()

	sort.Slice(keys, func(one, two int) bool {
		c1 := keys[one]
		c2 := keys[two]
		return c1.X < c2.X
	})
	minX = keys[0].X
	maxX = keys[len(keys)-1].X
	sort.Slice(keys, func(one, two int) bool {
		return keys[one].Z < keys[two].Z
	})
	minZ = keys[0].Z
	maxZ = keys[len(keys)-1].Z
	return
}
