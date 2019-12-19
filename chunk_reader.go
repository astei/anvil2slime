package main

type MinecraftChunk struct {
	// We just need to store these - we do not care much about the actual content
	Entities     []interface{}
	TileEntities []interface{}

	Biomes    []byte
	HeightMap []int

	Sections []MinecraftChunkSection
}

type MinecraftChunkSection struct {
	Y          int
	BlockLight []byte
	Blocks     []byte
	BlockData  []byte
	SkyLight   []byte
}
