package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/gordonklaus/dsp"
)

type Graph struct {
	graph dsp.Graph

	nodes []*Node
	conns []*Connection

	focus   interface{}
	focused bool

	menu *Menu
}

func NewGraph() *Graph {
	g := &Graph{
		menu: NewMenu(),
	}
	g.focus = g
	return g
}

func (g *Graph) Layout(gtx C) D {
	for _, e := range gtx.Events(g) {
		switch e := e.(type) {
		case key.FocusEvent:
			g.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow:
					g.focusNearestPort(layout.FPt(gtx.Constraints.Min).Mul(.5), e.Name, nil)
				}
			}
		case key.EditEvent:
			g.editEvent(e)
		}
	}

	paint.FillShape(gtx.Ops,
		color.NRGBA{A: 255},
		clip.Rect{Max: gtx.Constraints.Min}.Op(),
	)
	if g.focused {
		paint.FillShape(gtx.Ops,
			color.NRGBA{G: 128, B: 255, A: 255},
			clip.Border{
				Rect:  layout.FRect(image.Rectangle{Max: gtx.Constraints.Min}),
				Width: 4,
				SE:    4, SW: 4, NW: 4, NE: 4,
			}.Op(gtx.Ops),
		)
	}
	for _, c := range g.conns {
		c.Layout(gtx)
	}
	for _, n := range g.nodes {
		n.Layout(gtx)
	}

	key.InputOp{Tag: g}.Add(gtx.Ops)
	key.FocusOp{Tag: g.focus}.Add(gtx.Ops)

	if n := g.menu.Layout(gtx); n != nil {
		g.graph.Nodes = append(g.graph.Nodes, n)
		g.nodes = append(g.nodes, NewNode(n, g))
		g.arrange()
	}

	return D{Size: gtx.Constraints.Min}
}

func (g *Graph) editEvent(e key.EditEvent) {
	switch e.Text {
	case "+", "-", "*", "/":
		n := dsp.NewOperatorNode(e.Text)
		g.graph.Nodes = append(g.graph.Nodes, n)
		nn := NewNode(n, g)
		g.nodes = append(g.nodes, nn)
		g.arrange()
		g.focus = nn
	default:
		g.menu.activate(e)
	}
}

func (g *Graph) arrange() {
	layers, fakeConns := g.graph.Arrange()
	nodeNodes := make(map[*dsp.Node]*Node, len(g.nodes))
	for _, n := range g.nodes {
		nodeNodes[n.node] = n
	}
	for _, c := range g.conns {
		c.via = nil
	}
	for i, l := range layers {
		x := 400 + 192*(float32(i)-float32(len(layers)-1)/2)
		for i, n := range l {
			y := 300 + 64*(float32(i)-float32(len(l)-1)/2)
			if nn, ok := nodeNodes[n]; ok {
				nn.target = f32.Pt(x, y)
			} else {
			outer:
				for {
					prev := n.InPorts[0].Conns[0].Src.Node
					if nn, ok := nodeNodes[prev]; ok {
						for _, p := range nn.outports {
							for _, c := range p.conns {
								if cc, ok := fakeConns[c.conn]; ok && cc.Dst.Node == n {
									c.via = append(c.via, f32.Pt(x, y))
									break outer
								}
							}
						}
						panic("unreached")
					}
					n = prev
				}
			}
		}
	}
}

func (g *Graph) deleteConn(c *Connection) {
	c.src.disconnect(c)
	c.dst.disconnect(c)
	for i, c2 := range g.conns {
		if c2 == c {
			g.conns = append(g.conns[:i], g.conns[i+1:]...)
			break
		}
	}
}

func (g *Graph) focusNearestPort(pt f32.Point, dir string, filter func(*Port) bool) {
	if x := g.nearestPort(pt, dir, filter); x != nil {
		g.focus = x
	}
}

func (g *Graph) nearestPort(pt f32.Point, dir string, filter func(*Port) bool) *Port {
	var nearest *Port
	minDistance := float32(math.MaxFloat32)
	for _, n := range g.nodes {
		for _, p := range append(n.inports, n.outports...) {
			d := p.positionInGraph().Sub(pt)
			if !inDirection(d, dir) || filter != nil && !filter(p) {
				continue
			}
			d2 := d.X*d.X + d.Y*d.Y
			if d2 < minDistance {
				minDistance = d2
				nearest = p
			}
		}
	}
	return nearest
}

func inDirection(p f32.Point, dir string) bool {
	switch dir {
	case key.NameLeftArrow:
		return p.X < 0 && -p.X > abs(p.Y)
	case key.NameRightArrow:
		return p.X > 0 && p.X > abs(p.Y)
	case key.NameUpArrow:
		return p.Y < 0 && -p.Y > abs(p.X)
	case key.NameDownArrow:
		return p.Y > 0 && p.Y > abs(p.X)
	}
	return false
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
