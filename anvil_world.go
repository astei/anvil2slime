package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/astei/anvil2slime/nbt"
)

var blank [4096]byte

type ChunkCoord struct {
	X int
	Z int
}

type AnvilWorld struct {
	chunks map[ChunkCoord]MinecraftChunk
}

func OpenAnvilWorld(root string) (world *AnvilWorld, err error) {
	rootDirectory, err := os.Open(root)
	if err != nil {
		return
	}

	files, err := rootDirectory.Readdir(0)
	if err != nil {
		return
	}

	var regionReaders []*AnvilReader
	for _, possibleRegionFile := range files {
		fmt.Println("discovered ", possibleRegionFile.Name())
		if strings.HasSuffix(possibleRegionFile.Name(), ".mca") {
			file, err := os.Open(filepath.Join(root, possibleRegionFile.Name()))
			if err != nil {
				return nil, err
			}
			reader, err := NewAnvilReader(file)
			if err != nil {
				return nil, err
			}
			regionReaders = append(regionReaders, reader)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(regionReaders))
	resultChan := make(chan *map[ChunkCoord]MinecraftChunk, len(regionReaders))
	for _, reader := range regionReaders {
		go func(reader *AnvilReader, res chan *map[ChunkCoord]MinecraftChunk, wg *sync.WaitGroup) {
			defer wg.Done()
			result, err := tryToReadRegion(reader)
			if err != nil {
				fmt.Println("Unable to read chunks: " + err.Error())
				return
			}
			res <- result
		}(reader, resultChan, &wg)
	}

	wg.Wait()
	close(resultChan)

	allChunks := make(map[ChunkCoord]MinecraftChunk)
	for m := range resultChan {
		for k, v := range *m {
			allChunks[k] = v
		}
	}
	fmt.Printf("Discovered %d chunks in the world\n", len(allChunks))
	return &AnvilWorld{chunks: allChunks}, nil
}

func tryToReadRegion(reader *AnvilReader) (*map[ChunkCoord]MinecraftChunk, error) {
	byXZ := make(map[ChunkCoord]MinecraftChunk)
	for x := 0; x < 32; x++ {
		for z := 0; z < 32; z++ {
			if reader.ChunkExists(x, z) {
				chunkReader, err := reader.ReadChunk(x, z)
				if err != nil {
					return nil, fmt.Errorf("could not read chunk %d,%d in %s: %s", x, z, reader.Name, err.Error())
				}

				var anvilChunkRoot MinecraftChunkRoot
				if err = nbt.NewDecoder(chunkReader).Decode(&anvilChunkRoot); err != nil {
					return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: %s", x, z, reader.Name, err.Error())
				}

				var cleanedSections []MinecraftChunkSection
				for _, section := range anvilChunkRoot.Level.Sections {
					if !bytes.Equal(blank[:], section.Blocks) {
						cleanedSections = append(cleanedSections, section)
					}
				}
				anvilChunkRoot.Level.Sections = cleanedSections
				if len(anvilChunkRoot.Level.Sections) == 0 {
					continue
				}

				// further sanity checks...
				if len(anvilChunkRoot.Level.HeightMap) != 256 {
					return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: invalid height map size", x, z, reader.Name)
				}
				if len(anvilChunkRoot.Level.Biomes) != 256 {
					return nil, fmt.Errorf("could not deserialize chunk %d,%d in %s: invalid biome size", x, z, reader.Name)
				}

				coords := ChunkCoord{X: anvilChunkRoot.Level.X, Z: anvilChunkRoot.Level.Z}
				byXZ[coords] = anvilChunkRoot.Level
			}
		}
	}
	return &byXZ, nil
}
