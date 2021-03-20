package node

import (
	"math/rand"
)

type Noise struct {
	rand *rand.Rand
}

func (n *Noise) Init(c Config) {
	n.rand = c.GetRand()
}

func (n *Noise) Process() float32 {
	return 2*n.rand.Float32() - 1
}
