package main

import "math"

type fixedBitSet struct {
	set []byte
}

func newFixedBitSet(max int) *fixedBitSet {
	return &fixedBitSet{set: make([]byte, int(math.Ceil(float64(max)/float64(8))))}
}

func (set *fixedBitSet) Set(idx int) {
	mapIdx := idx / 8
	inMapIdx := idx % 8

	set.set[mapIdx] = set.set[mapIdx] | (1 << inMapIdx)
}

func (set *fixedBitSet) Bytes() []byte {
	return append([]byte(nil), set.set...)
}
