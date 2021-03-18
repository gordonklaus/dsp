package ui

import (
	"go/token"
	"image"
	"image/color"
	"strconv"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/eventx"
	"github.com/gordonklaus/dsp"
)

type Node struct {
	node *dsp.Node

	graph      *Graph
	height     float32
	target     f32.Point
	pos        f32.Point
	focused    bool
	editor     *widget.Editor
	oldText    string
	oldCaret   int
	drag       gesture.Drag
	dragStart  f32.Point
	delayColor color.NRGBA

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
	n.height = 1.5 * portSize * float32(maxPorts)

	y := float32(.75 * portSize)
	if d := maxPorts - len(node.InPorts); d > 0 {
		y += .75 * portSize * float32(d)
	}
	for _, p := range node.InPorts {
		n.inports = append(n.inports, NewPort(p, n, f32.Pt(0, y)))
		y += 1.5 * portSize
	}

	y = .75 * portSize
	if n.node.IsDelayWrite() {
		y += 1.5 * portSize
	} else if d := maxPorts - len(node.OutPorts); d > 0 {
		y += .75 * portSize * float32(d)
	}
	for _, p := range node.OutPorts {
		n.outports = append(n.outports, NewPort(p, n, f32.Pt(nodeWidth, y)))
		y += 1.5 * portSize
	}

	return n
}

func (n *Node) Layout(gtx C) D {
	size := image.Pt(px(gtx, nodeWidth), px(gtx, n.height))
	rect := image.Rectangle{Max: size}

	if d := n.target.Sub(n.pos); d.X*d.X+d.Y*d.Y > 1 {
		if !n.drag.Dragging() {
			n.pos = n.pos.Add(d.Mul(.1))
			op.InvalidateOp{}.Add(gtx.Ops)
		}
	} else {
		n.pos = n.target
	}

	for _, e := range gtx.Events(n) {
		switch e := e.(type) {
		case key.FocusEvent:
			n.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow:
					n.graph.focusNearest(n.pos.Add(layout.FPt(size).Mul(.5)), e.Name)
				case key.NameDeleteBackward, key.NameDeleteForward:
					n.graph.deleteNode(n)
					n.graph.arrange()
					n.graph.focus = n.graph
				case key.NameReturn:
					if n.node.IsInport() || n.node.IsOutport() || n.node.IsConst() {
						n.edit()
					}
				case key.NameEscape:
					n.graph.focus = n.graph
				}
			}
		case key.EditEvent:
			if e.Text == "," || e.Text == "<" {
				if n.node.IsInport() {
					n.graph.ports.in.new(n, e.Text == ",")
					break
				} else if n.node.IsOutport() {
					n.graph.ports.out.new(n, e.Text == ",")
					break
				}
			} else if e.Text == "=" && n.node.IsDelay() {
				n.graph.addNode(dsp.NewDelayReadNode(n.node))
				break
			}

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
	op.Offset(pxpt(gtx, n.pos)).Add(gtx.Ops)

	key.InputOp{Tag: n}.Add(gtx.Ops)
	pointer.Rect(rect).Add(gtx.Ops)
	n.drag.Add(gtx.Ops)

	paint.FillShape(gtx.Ops, white, clip.Rect(rect).Op())
	if n.delayColor != (color.NRGBA{}) {
		paint.FillShape(gtx.Ops, n.delayColor,
			clip.Circle{
				Center: layout.FPt(size).Mul(.5),
				Radius: float32(px(gtx, 8)),
			}.Op(gtx.Ops),
		)
	}
	gtx.Constraints.Min = size
	layout.Center.Layout(gtx, n.layoutText)
	if n.focused {
		r := float32(px(gtx, 4))
		paint.FillShape(gtx.Ops, blue,
			clip.Stroke{
				Path:  clip.UniformRRect(layout.FRect(rect.Inset(px(gtx, -2))), r).Path(gtx.Ops),
				Style: clip.StrokeStyle{Width: r},
			}.Op(),
		)
	}

	for _, p := range append(n.inports, n.outports...) {
		p.Layout(gtx)
	}

	return D{Size: size}
}

func (n *Node) layoutText(gtx C) D {
	if n.editor == nil {
		return material.Body1(th, n.name()).Layout(gtx)
	}

	for _, e := range n.editor.Events() {
		switch e := e.(type) {
		case widget.ChangeEvent:
			n.validateEditor()
		case widget.SubmitEvent:
			n.setName(e.Text)
			n.editor = nil
			n.graph.arrange()
			n.graph.focus = n
			return D{}
		}
	}
	_, n.oldCaret = n.editor.CaretPos()

	spy, gtx := eventx.Enspy(gtx)
	dims := material.Editor(th, n.editor, "").Layout(gtx)

	for _, e := range spy.AllEvents() {
		for _, e := range e.Items {
			switch e := e.(type) {
			case key.Event:
				if e.State == key.Press {
					switch e.Name {
					case key.NameEscape:
						n.editor = nil
						n.graph.focus = n
					}
				}
			}
		}
	}

	return dims
}

func (n *Node) edit() {
	n.editor = &widget.Editor{
		Alignment:  text.Middle,
		SingleLine: true,
		Submit:     true,
	}
	name := n.name()
	n.editor.SetText(name)
	n.editor.SetCaret(n.editor.Len(), n.editor.Len())
	n.editor.Focus()
	n.graph.focus = nil
	n.oldText = name
	_, n.oldCaret = n.editor.CaretPos()
}

func (n *Node) validateEditor() {
	if n.node.IsInport() || n.node.IsOutport() {
		if !token.IsIdentifier(n.editor.Text()) || n.editor.Text() == "_" {
			if n.editor.Text() == "" {
				n.editor.SetText("x")
				n.editor.SetCaret(0, 1)
			} else {
				n.editor.SetText(n.oldText)
				n.editor.SetCaret(n.oldCaret, n.oldCaret)
			}
		}
	} else if n.node.IsConst() {
		if _, err := strconv.ParseFloat(n.editor.Text(), 64); err != nil {
			if n.editor.Text() == "" {
				n.editor.SetText("0")
				n.editor.SetCaret(0, 1)
			} else {
				n.editor.SetText(n.oldText)
				n.editor.SetCaret(n.oldCaret, n.oldCaret)
			}
		}
	}
	n.oldText = n.editor.Text()
}

func (n *Node) name() string {
	switch {
	case n.node.IsInport():
		return n.node.Name[3:]
	case n.node.IsOutport():
		return n.node.Name[4:]
	case n.node.IsDelay():
		return "="
	}
	return n.node.Name
}

func (n *Node) setName(name string) {
	switch {
	case n.node.IsInport():
		n.node.Name = "in-" + name
	case n.node.IsOutport():
		n.node.Name = "out-" + name
	default:
		n.node.Name = name
	}
}
