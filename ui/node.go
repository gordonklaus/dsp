package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/gordonklaus/dsp"
)

type Node struct {
	node *dsp.Node

	graph     *Graph
	height    float32
	target    f32.Point
	pos       f32.Point
	focused   bool
	drag      gesture.Drag
	dragStart f32.Point

	inports, outports []*Port
}

const nodeWidth = 64

func NewNode(node *dsp.Node, graph *Graph) *Node {
	n := &Node{
		node:  node,
		graph: graph,
	}

	maxPorts := len(node.InPorts)
	if maxPorts < len(node.OutPorts) {
		maxPorts = len(node.OutPorts)
	}
	const textHeight = 20
	n.height = textHeight + 1.5*portSize*float32(maxPorts)

	y := textHeight + float32(.75*portSize)
	if d := maxPorts - len(node.InPorts); d > 0 {
		y += .75 * portSize * float32(d)
	}
	for _, p := range node.InPorts {
		n.inports = append(n.inports, NewPort(p, n, f32.Pt(0, y)))
		y += 1.5 * portSize
	}

	y = textHeight + .75*portSize
	if d := maxPorts - len(node.OutPorts); d > 0 {
		y += .75 * portSize * float32(d)
	}
	for _, p := range node.OutPorts {
		n.outports = append(n.outports, NewPort(p, n, f32.Pt(nodeWidth, y)))
		y += 1.5 * portSize
	}

	return n
}

func (n *Node) Layout(gtx C) D {
	size := image.Pt(nodeWidth, int(n.height))
	rect := image.Rectangle{Max: size}

	if d := n.target.Sub(n.pos); d.X*d.X+d.Y*d.Y > float32(gtx.Px(unit.Dp(1))) && !n.drag.Dragging() {
		n.pos = n.pos.Add(d.Mul(.1))
		op.InvalidateOp{}.Add(gtx.Ops)
	}

	for _, e := range gtx.Events(n) {
		switch e := e.(type) {
		case key.FocusEvent:
			n.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow:
					n.graph.focusNearestPort(n.pos.Add(layout.FPt(size).Mul(.5)), e.Name, nil)
				case key.NameEscape:
					n.graph.focus = n.graph
				}
			}
		case key.EditEvent:
			n.graph.editEvent(e)
		}
	}

	for _, e := range n.drag.Events(gtx.Metric, gtx, gesture.Both) {
		switch e.Type {
		case pointer.Press:
			n.dragStart = e.Position
		case pointer.Drag:
			n.pos = n.pos.Add(e.Position.Sub(n.dragStart))
		}
	}

	defer op.Save(gtx.Ops).Load()
	op.Offset(n.pos).Add(gtx.Ops)

	key.InputOp{Tag: n}.Add(gtx.Ops)
	pointer.Rect(rect).Add(gtx.Ops)
	n.drag.Add(gtx.Ops)

	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		clip.Rect(rect).Op(),
	)
	gtx.Constraints.Min = image.Pt(nodeWidth, 20)
	lbl := material.Body1(th, n.node.Name)
	lbl.Color = color.NRGBA{A: 255}
	layout.Center.Layout(gtx, lbl.Layout)
	if n.focused {
		paint.FillShape(gtx.Ops,
			color.NRGBA{G: 128, B: 255, A: 255},
			clip.Border{
				Rect:  layout.FRect(rect),
				Width: 4,
			}.Op(gtx.Ops),
		)
	}

	for _, p := range append(n.inports, n.outports...) {
		p.Layout(gtx)
	}

	return D{Size: size}
}
