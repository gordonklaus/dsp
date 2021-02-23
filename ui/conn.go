package ui

import (
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/gordonklaus/dsp"
)

type Connection struct {
	conn *dsp.Connection

	graph             *Graph
	src, dst          *Port
	via               []f32.Point
	focused, focusdst bool

	editing      bool
	originalPort *Port
}

func NewConnection(conn *dsp.Connection, graph *Graph, src, dst *Port) *Connection {
	return &Connection{
		conn:  conn,
		graph: graph,
		src:   src,
		dst:   dst,
	}
}

func (c *Connection) Layout(gtx C) {
	for _, e := range gtx.Events(c) {
		switch e := e.(type) {
		case key.FocusEvent:
			c.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				if c.editing {
					switch e.Name {
					case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow:
						pt := c.srcPt()
						if c.focusdst {
							pt = c.dstPt()
						}
						p := c.graph.nearestPort(pt, e.Name, func(p *Port) bool { return p.port.Out != c.focusdst })
						if p != nil {
							*c.focusedPort() = p
						}
					case key.NameEscape:
						if c.originalPort == nil && *c.focusedPort() == nil {
							c.graph.focus = c.nonfocusedPort()
							c.delete()
							break
						}
						*c.focusedPort() = c.originalPort
						if c.originalPort != nil {
							c.editing = false
						}
					case key.NameReturn:
						if *c.focusedPort() == nil {
							break
						}
						if *c.focusedPort() != c.originalPort {
							if c.originalPort != nil {
								c.originalPort.disconnect(c)
							} else {
								c.nonfocusedPort().connect(c)
							}
							(*c.focusedPort()).connect(c)
							c.graph.arrange()
						}
						c.editing = false
					}
					break
				}

				switch e.Name {
				case key.NameLeftArrow:
					if c.focusdst {
						c.focusdst = false
					} else if c.src != nil {
						c.graph.focus = c.src
					}
				case key.NameRightArrow:
					if !c.focusdst {
						c.focusdst = true
					} else if c.dst != nil {
						c.graph.focus = c.dst
					}
				case key.NameUpArrow, key.NameDownArrow:
				case key.NameReturn:
					c.originalPort = *c.focusedPort()
					c.editing = true
				case key.NameDeleteBackward:
					c.graph.focus = c.src
					c.graph.deleteConn(c)
					c.graph.arrange()
				case key.NameDeleteForward:
					c.graph.focus = c.dst
					c.graph.deleteConn(c)
					c.graph.arrange()
				case key.NameEscape:
					c.graph.focus = *c.focusedPort()
				}
			}
		case key.EditEvent:
			c.graph.editEvent(e)
		}
	}

	defer op.Save(gtx.Ops).Load()

	path := clip.Path{}
	path.Begin(gtx.Ops)
	path.MoveTo(c.srcPt())
	for _, p := range c.via {
		path.Cube(f32.Pt(64, 0), p.Sub(f32.Pt(64, 0)).Sub(path.Pos()), p.Sub(path.Pos()))
	}
	if c.editing && *c.focusedPort() == nil {
		path.LineTo(c.dstPt())
	} else {
		path.Cube(f32.Pt(64, 0), c.dstPt().Sub(f32.Pt(64, 0)).Sub(path.Pos()), c.dstPt().Sub(path.Pos()))
	}
	clip.Stroke{
		Path: path.End(),
		Style: clip.StrokeStyle{
			Width: 2,
			Cap:   clip.RoundCap,
		},
	}.Op().Add(gtx.Ops)
	if c.focused {
		col1 := color.NRGBA{R: 0, G: 128, B: 255, A: 255}
		if c.editing {
			col1 = color.NRGBA{R: 255, G: 128, B: 0, A: 255}
		}
		col2 := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		if c.focusdst {
			col1, col2 = col2, col1
		}
		paint.LinearGradientOp{
			Stop1:  c.srcPt(),
			Stop2:  c.dstPt(),
			Color1: col1,
			Color2: col2,
		}.Add(gtx.Ops)
	} else {
		paint.ColorOp{Color: color.NRGBA{R: 255, G: 255, B: 255, A: 255}}.Add(gtx.Ops)
	}
	paint.PaintOp{}.Add(gtx.Ops)

	key.InputOp{Tag: c}.Add(gtx.Ops)
}

func (c *Connection) srcPt() f32.Point {
	if c.src != nil {
		return c.src.position()
	}
	return c.dst.position().Sub(f32.Pt(64, 0))
}

func (c *Connection) dstPt() f32.Point {
	if c.dst != nil {
		return c.dst.position()
	}
	return c.src.position().Add(f32.Pt(64, 0))
}

func (c *Connection) focusedPort() **Port {
	if c.focusdst {
		return &c.dst
	}
	return &c.src
}

func (c *Connection) nonfocusedPort() *Port {
	if c.focusdst {
		return c.src
	}
	return c.dst
}

func (c *Connection) delete() {
	for i, c2 := range c.graph.conns {
		if c2 == c {
			c.graph.conns = append(c.graph.conns[:i], c.graph.conns[i+1:]...)
			break
		}
	}
	if c.src != nil {
		c.src.disconnect(c)
	}
	if c.dst != nil {
		c.dst.disconnect(c)
	}
}
