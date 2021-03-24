package node

import "math"

type Delay struct {
	sampleRate float32
	x          []float32
	i          int
}

func (d *Delay) Init(c Config) {
	d.sampleRate = c.SampleRate
	d.x = make([]float32, 4)
}

func (d *Delay) FeedbackRead(t float32) float32 {
	i, f := math.Modf(float64(t * d.sampleRate))
	return d.read(int(i)-1, float32(f))
}

func (d *Delay) Write(x float32) {
	d.i++
	if d.i == len(d.x) {
		d.i = 0
	}
	d.x[d.i] = x
}

func (d *Delay) Read(t float32) float32 {
	i, f := math.Modf(float64(t * d.sampleRate))
	return d.read(int(i), float32(f))
}

func (d *Delay) read(i int, f float32) float32 {
	if i < 0 {
		return d.ReadSample(0)
	}
	if i == 0 {
		return interp3(f, d.ReadSample(0), d.ReadSample(0), d.ReadSample(1), d.ReadSample(2))
	}
	return interp3(f, d.ReadSample(i-1), d.ReadSample(i), d.ReadSample(i+1), d.ReadSample(i+2))
}

func (d *Delay) ReadSample(i int) float32 {
	for 2*i >= len(d.x) {
		d.x = append(d.x, d.x...)
	}
	i = d.i - i
	if i < 0 {
		i += len(d.x)
	}
	return d.x[i]
}

// Hermite cubic interpolation between x1 and x2 (t=0..1).
func interp3(t, x0, x1, x2, x3 float32) float32 {
	c0 := x1
	c1 := (x2 - x0) / 2
	c2 := x0 - 2.5*x1 + 2*x2 - x3/2
	c3 := 1.5*(x1-x2) + (x3-x0)/2
	return c0 + t*(c1+t*(c2+t*c3))
}
