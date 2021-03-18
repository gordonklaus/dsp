package ui

import (
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
						p := c.graph.nearestPort(pt, e.Name, func(p *Port) bool {
							if p.port.Out == c.focusdst || !p.port.Out && len(p.conns) > 0 && p != c.originalPort {
								return false
							}
							srcNode, dstNode := p.node, c.nonfocusedPort().node
							if c.focusdst {
								srcNode, dstNode = dstNode, srcNode
							}
							return !isOrPrecedes(dstNode, srcNode)
						})
						if p != nil {
							*c.focusedPort() = p
						}
					case key.NameDeleteBackward:
						c.graph.focus = c.dst
						if c.src != nil {
							c.graph.focus = c.src
						}
						c.delete()
						c.graph.arrange()
					case key.NameDeleteForward:
						c.graph.focus = c.src
						if c.dst != nil {
							c.graph.focus = c.dst
						}
						c.delete()
						c.graph.arrange()
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
				case key.NameLeftArrow, key.NameRightArrow:
					if e.Name == key.NameLeftArrow == c.focusdst {
						c.focusdst = !c.focusdst
					} else {
						p := c.graph.nearestPort((*c.focusedPort()).position(), e.Name, func(*Port) bool { return true })
						if p != nil {
							if c2 := p.centralConn(); c2 != nil {
								c.graph.focus = c2
								c2.focusdst = !c.focusdst
							} else {
								c.graph.focus = p
							}
						}
					}
				case key.NameUpArrow, key.NameDownArrow:
					next := (*c.focusedPort()).nextConn(c, e.Name == key.NameUpArrow)
					if next != nil {
						c.graph.focus = next
						next.focusdst = c.focusdst
					} else {
						p := c.graph.nearestPort((*c.focusedPort()).position(), e.Name, func(*Port) bool { return true })
						if p != nil {
							if c2 := p.firstConn(e.Name == key.NameUpArrow); c2 != nil {
								c.graph.focus = c2
								c2.focusdst = c.focusdst
							} else {
								c.graph.focus = p
							}
						}
					}
				case key.NameReturn:
					c.originalPort = *c.focusedPort()
					c.editing = true
				case key.NameDeleteBackward:
					c.graph.focus = c.src
					c.delete()
					c.graph.arrange()
				case key.NameDeleteForward:
					c.graph.focus = c.dst
					c.delete()
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
	path.MoveTo(pxpt(gtx, c.srcPt()))
	if c.editing {
		path.LineTo(pxpt(gtx, c.dstPt()))
	} else {
		d := f32.Pt(nodeWidth+layerGap, 0)
		pts := append([]f32.Point{c.srcPt().Sub(d), c.srcPt()}, append(c.via, c.dstPt(), c.dstPt().Add(d))...)
		var ctrl1 f32.Point
		for i := 1; i < len(pts)-1; i++ {
			p0 := pxpt(gtx, pts[i-1])
			p1 := pxpt(gtx, pts[i])
			p2 := pxpt(gtx, pts[i+1])
			d := p2.Sub(p0).Mul(1. / 6)
			ctrl2 := p1.Sub(d)
			if i > 1 {
				path.CubeTo(ctrl1, ctrl2, p1)
			}
			ctrl1 = p1.Add(d)
		}
	}
	clip.Stroke{
		Path: path.End(),
		Style: clip.StrokeStyle{
			Width: float32(px(gtx, 2)),
			Cap:   clip.RoundCap,
		},
	}.Op().Add(gtx.Ops)
	if c.focused {
		col1 := blue
		if c.editing {
			col1 = red
		}
		col2 := white
		if c.focusdst {
			col1, col2 = col2, col1
		}
		paint.LinearGradientOp{
			Stop1:  pxpt(gtx, c.srcPt()),
			Stop2:  pxpt(gtx, c.dstPt()),
			Color1: col1,
			Color2: col2,
		}.Add(gtx.Ops)
	} else {
		paint.ColorOp{Color: white}.Add(gtx.Ops)
	}
	paint.PaintOp{}.Add(gtx.Ops)

	key.InputOp{Tag: c}.Add(gtx.Ops)
}

func isOrPrecedes(n1, n2 *Node) bool {
	if n1 == n2 {
		return true
	}
	for _, p := range n1.outports {
		for _, c := range p.conns {
			if isOrPrecedes(c.dst.node, n2) {
				return true
			}
		}
	}
	return false
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
