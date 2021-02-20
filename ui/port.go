package ui

import (
	"image"
	"image/color"

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
	size := image.Pt(portSize, portSize)
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
						p.node.graph.focus = p.conns[0]
						p.conns[0].focusdst = true
					} else {
						p.node.graph.focusNearestPort(p.positionInGraph(), e.Name, nil)
					}
				case key.NameRightArrow:
					if p.port.Out && len(p.conns) > 0 {
						p.node.graph.focus = p.conns[0]
						p.conns[0].focusdst = false
					} else {
						p.node.graph.focusNearestPort(p.positionInGraph(), e.Name, nil)
					}
				case key.NameUpArrow, key.NameDownArrow:
					p.node.graph.focusNearestPort(p.positionInGraph(), e.Name, nil)
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
		}
	}

	defer op.Save(gtx.Ops).Load()
	op.Offset(p.pos.Sub(layout.FPt(size).Mul(.5))).Add(gtx.Ops)

	const r = portSize / 2
	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 0, G: 0, B: 0, A: 255},
		clip.UniformRRect(layout.FRect(rect), r).Op(gtx.Ops),
	)
	if p.focused {
		paint.FillShape(gtx.Ops,
			color.NRGBA{G: 128, B: 255, A: 255},
			clip.Border{
				Rect:  layout.FRect(rect),
				Width: 4,
				SE:    r, SW: r, NW: r, NE: r,
			}.Op(gtx.Ops),
		)
	}

	key.InputOp{Tag: p}.Add(gtx.Ops)

	return D{Size: size}
}

func (p *Port) positionInGraph() f32.Point {
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
