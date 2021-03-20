package node

import "math/rand"

// A Node processes signals.
// In addition to Init, a Node must have a method named Process whose parameters and results are all of type float32.
type Node interface {
	Init(Config)
}

type Config struct {
	SampleRate float32
	Rand       *rand.Rand
}

func (c *Config) GetRand() *rand.Rand {
	if c.Rand == nil {
		c.Rand = rand.New(rand.NewSource(1))
	}
	return c.Rand
}
