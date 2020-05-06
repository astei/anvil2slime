package main

import (
	"bytes"
	"encoding/binary"
	"github.com/astei/anvil2slime/nbt"
	"github.com/klauspost/compress/zstd"
	"io"
	"io/ioutil"
	"sort"
)

const slimeHeader = 0xB10B
const slimeLatestVersion = 3

func slimeChunkKey(coord ChunkCoord) int64 {
	return (int64(coord.Z) * 0x7fffffff) + int64(coord.X)
}

func (world *AnvilWorld) WriteAsSlime(writer io.Writer) error {
	zstdWriter, err := zstd.NewWriter(ioutil.Discard)
	if err != nil {
		return err
	}
	slimeWriter := &slimeWriter{writer: writer, world: world, zstdWriter: zstdWriter}
	return slimeWriter.writeWorld()
}

type slimeWriter struct {
	writer     io.Writer
	world      *AnvilWorld
	zstdWriter *zstd.Encoder
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
	_, err = w.writer.Write(used)
	return
}

func (w *slimeWriter) createChunkBitset(width int, depth int, minChunkXZ ChunkCoord) []byte {
	chunkCoords := w.world.getChunkKeys()
	populated := newFixedBitSet(width * depth)
	for _, currentChunk := range chunkCoords {
		relZ := currentChunk.Z - minChunkXZ.Z
		relX := currentChunk.X - minChunkXZ.X
		idx := relZ*width + relX
		populated.Set(idx)
	}

	return populated.Bytes()
}

func (w *slimeWriter) determineChunkBounds() (minChunkXZ ChunkCoord, width int, depth int) {
	// Slime uses its own order for chunks, but still requires us to determine the maximum/minimum XZ coordinates.
	minX, maxX, minZ, maxZ := w.world.getMinXZ()

	width = maxX - minX + 1
	depth = maxZ - minZ + 1
	return ChunkCoord{X: minX, Z: minZ}, width, depth
}

func (w *slimeWriter) writeChunks() (err error) {
	slimeSorted := w.world.getChunkKeys()
	sort.Slice(slimeSorted, func(one, two int) bool {
		k1 := slimeChunkKey(slimeSorted[one])
		k2 := slimeChunkKey(slimeSorted[two])
		return k1 < k2
	})

	var out bytes.Buffer
	for _, coord := range slimeSorted {
		chunk := w.world.chunks[coord]
		if err = w.writeChunkHeader(chunk, &out); err != nil {
			return
		}
		for _, section := range chunk.Sections {
			if err = w.writeChunkSection(section, &out); err != nil {
				return
			}
		}
	}

	return w.writeZstdCompressed(&out)
}

func (w *slimeWriter) writeChunkHeader(chunk MinecraftChunk, out io.Writer) (err error) {
	for _, heightEntry := range chunk.HeightMap {
		if err = binary.Write(out, binary.BigEndian, int32(heightEntry)); err != nil {
			return
		}
	}
	if _, err = out.Write(chunk.Biomes); err != nil {
		return
	}
	w.writeChunkSectionsPopulatedBitmask(chunk, out)
	return
}

func (w *slimeWriter) writeChunkSectionsPopulatedBitmask(chunk MinecraftChunk, out io.Writer) {
	sectionsPopulated := newFixedBitSet(16)
	for _, section := range chunk.Sections {
		sectionsPopulated.Set(int(section.Y))
	}
	_, _ = out.Write(sectionsPopulated.Bytes())
	return
}

func (w *slimeWriter) writeChunkSection(section MinecraftChunkSection, out io.Writer) (err error) {
	if _, err = out.Write(section.BlockLight); err != nil {
		return
	}
	if _, err = out.Write(section.Blocks); err != nil {
		return
	}
	if _, err = out.Write(section.Data); err != nil {
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

func (w *slimeWriter) writeZstdCompressed(buf *bytes.Buffer) (err error) {
	uncompressedSize := buf.Len()

	var compressedOutput bytes.Buffer
	w.zstdWriter.Reset(&compressedOutput)
	if _, err = buf.WriteTo(w.zstdWriter); err != nil {
		return
	}
	if err = w.zstdWriter.Close(); err != nil {
		return
	}
	w.zstdWriter.Reset(ioutil.Discard)

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
	return w.writeCompressedNbt(compound)
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

	if _, err = w.writer.Write([]byte{1}); err != nil {
		return
	}
	return w.writeCompressedNbt(compound)
}

func (w *slimeWriter) writeCompressedNbt(compound interface{}) (err error) {
	var buf bytes.Buffer
	if err = nbt.NewEncoder(&buf).Encode(compound); err != nil {
		return
	}
	return w.writeZstdCompressed(&buf)
}

func (w *slimeWriter) writeExtra() (err error) {
	// Write empty NBT tag compound
	var empty map[string]interface{}
	return w.writeCompressedNbt(empty)
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
