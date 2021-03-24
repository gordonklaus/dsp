package dsp

import (
	"math/rand"
)

type WhiteNoise struct {
	rand *rand.Rand
}

func (n *WhiteNoise) Init(c Config) {
	n.rand = c.GetRand()
}

func (n *WhiteNoise) Process() float32 {
	return 2*n.rand.Float32() - 1
}
