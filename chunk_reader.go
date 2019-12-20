package main

type MinecraftChunkRoot struct {
	Level MinecraftChunk
}

type MinecraftChunk struct {
	X int `nbt:"xPos"`
	Z int `nbt:"zPos"`

	// We just need to store these - we do not care much about the actual content
	Entities     []interface{}
	TileEntities []interface{}

	Biomes    []byte
	HeightMap []int

	Sections []MinecraftChunkSection
}

type MinecraftChunkSection struct {
	Y          uint8
	BlockLight []byte
	Blocks     []byte
	BlockData  []byte
	SkyLight   []byte
}
