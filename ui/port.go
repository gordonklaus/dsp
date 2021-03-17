package ui

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/gordonklaus/dsp"
)

type Port struct {
	port  *dsp.Port
	conns []*Connection

	node    *Node
	pos     f32.Point
	focused bool
}

func NewPort(port *dsp.Port, node *Node, pos f32.Point) *Port {
	return &Port{
		port: port,
		node: node,
		pos:  pos,
	}
}

const portSize = 16

func (p *Port) Layout(gtx C) D {
	size := image.Pt(px(gtx, portSize), px(gtx, portSize))
	rect := image.Rectangle{Max: size}

	for _, e := range gtx.Events(p) {
		switch e := e.(type) {
		case key.FocusEvent:
			p.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NameLeftArrow:
					if !p.port.Out && len(p.conns) > 0 {
						p.focusCentralConn()
					} else {
						p.node.graph.focusNearest(p.position(), e.Name)
					}
				case key.NameRightArrow:
					if p.port.Out && len(p.conns) > 0 {
						p.focusCentralConn()
					} else {
						p.node.graph.focusNearest(p.position(), e.Name)
					}
				case key.NameUpArrow, key.NameDownArrow:
					p.node.graph.focusNearest(p.position(), e.Name)
				case key.NameDeleteBackward, key.NameDeleteForward:
					for len(p.conns) > 0 {
						p.conns[0].delete()
					}
					p.node.graph.arrange()
				case key.NameEscape:
					p.node.graph.focus = p.node
				case key.NameReturn:
					c := &dsp.Connection{}
					cc := NewConnection(c, p.node.graph, nil, nil)
					if p.port.Out {
						c.Src = p.port
						cc.src = p
					} else {
						c.Dst = p.port
						cc.dst = p
					}
					p.node.graph.conns = append(p.node.graph.conns, cc)
					p.node.graph.focus = cc
					cc.focusdst = p.port.Out
					cc.editing = true
				}
			}
		case key.EditEvent:
			if e.Text == "," || e.Text == "<" {
				if p.node.node.IsInport() {
					p.node.graph.ports.in.new(p.node, e.Text == ",")
				} else if p.node.node.IsOutport() {
					p.node.graph.ports.out.new(p.node, e.Text == ",")
				}
			} else {
				p.node.graph.editEvent(e)
			}
		}
	}

	defer op.Save(gtx.Ops).Load()
	op.Offset(pxpt(gtx, p.pos).Sub(layout.FPt(size).Mul(.5))).Add(gtx.Ops)

	r := float32(size.X) / 2
	paint.FillShape(gtx.Ops,
		black,
		clip.UniformRRect(layout.FRect(rect), r).Op(gtx.Ops),
	)
	if p.focused {
		paint.FillShape(gtx.Ops,
			blue,
			clip.Border{
				Rect:  layout.FRect(rect),
				Width: float32(px(gtx, 4)),
				SE:    r, SW: r, NW: r, NE: r,
			}.Op(gtx.Ops),
		)
	}

	key.InputOp{Tag: p}.Add(gtx.Ops)

	return D{Size: size}
}

func (p *Port) centralConn() *Connection {
	var c *Connection
	dist := math.MaxFloat32
	for _, c2 := range p.conns {
		if d := math.Abs(float64(connY(c2, p.port.Out) - p.position().Y)); d < dist {
			dist = d
			c = c2
		}
	}
	return c
}

func (p *Port) focusCentralConn() {
	if c := p.centralConn(); c != nil {
		p.node.graph.focus = c
		c.focusdst = !p.port.Out
	}
}

func (p *Port) nextConn(c *Connection, prev bool) *Connection {
	var next *Connection
	dist := float32(math.MaxFloat32)
	for _, c2 := range p.conns {
		d := connY(c2, p.port.Out) - connY(c, p.port.Out)
		if prev {
			d = -d
		}
		if d > 0 && d < dist {
			dist = d
			next = c2
		}
	}
	if next == nil {
		if p2 := p.nextPort(prev); p2 != nil {
			next = p2.firstConn(prev)
		}
	}
	return next
}

func (p *Port) nextPort(prev bool) *Port {
	ports := p.node.inports
	if p.port.Out {
		ports = p.node.outports
	}
	for i, p2 := range ports {
		if p2 == p {
			if prev && i > 0 {
				return ports[i-1]
			}
			if !prev && i+1 < len(ports) {
				return ports[i+1]
			}
			return nil
		}
	}
	panic("unreached")
}

func (p *Port) firstConn(last bool) *Connection {
	var c *Connection
	y := float32(0)
	for _, c2 := range p.conns {
		y2 := connY(c2, p.port.Out)
		if c == nil || !last && y2 < y || last && y2 > y {
			y = y2
			c = c2
		}
	}
	return c
}

func connY(c *Connection, out bool) float32 {
	if out {
		if len(c.via) > 0 {
			return c.via[0].Y
		}
		return c.dst.position().Y
	}
	if len(c.via) > 0 {
		return c.via[len(c.via)-1].Y
	}
	return c.src.position().Y
}

func (p *Port) position() f32.Point {
	return p.pos.Add(p.node.pos)
}

func (p *Port) connect(c *Connection) {
	if p.port.Out {
		c.conn.Src = p.port
	} else {
		c.conn.Dst = p.port
	}
	p.conns = append(p.conns, c)
	p.port.Conns = append(p.port.Conns, c.conn)
}

func (p *Port) disconnect(c *Connection) {
	for i, c2 := range p.conns {
		if c2 == c {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			break
		}
	}
	for i, c2 := range p.port.Conns {
		if c2 == c.conn {
			p.port.Conns = append(p.port.Conns[:i], p.port.Conns[i+1:]...)
			break
		}
	}
}
