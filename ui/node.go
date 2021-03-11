package ui

import (
	"go/token"
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
	"gioui.org/text"
	"gioui.org/unit"
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
	size := image.Pt(nodeWidth, int(n.height))
	rect := image.Rectangle{Max: size}

	if d := n.target.Sub(n.pos); int(d.X*d.X+d.Y*d.Y) > gtx.Px(unit.Dp(1)) {
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
					if n.node.IsInport() || n.node.IsOutport() {
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
	op.Offset(n.pos).Add(gtx.Ops)

	key.InputOp{Tag: n}.Add(gtx.Ops)
	pointer.Rect(rect).Add(gtx.Ops)
	n.drag.Add(gtx.Ops)

	paint.FillShape(gtx.Ops,
		color.NRGBA{R: 255, G: 255, B: 255, A: 255},
		clip.Rect(rect).Op(),
	)
	if n.delayColor != (color.NRGBA{}) {
		paint.FillShape(gtx.Ops,
			n.delayColor,
			clip.RRect{
				Rect: f32.Rectangle{Max: f32.Pt(8, 8)},
				SE:   8,
			}.Op(gtx.Ops),
		)
	}
	gtx.Constraints.Min = image.Pt(nodeWidth, 20)
	if n.editor != nil {
		n.layoutEditor(gtx)
	} else {
		lbl := material.Body1(th, n.name())
		lbl.Color = color.NRGBA{A: 255}
		layout.Center.Layout(gtx, lbl.Layout)
	}
	if n.focused {
		paint.FillShape(gtx.Ops,
			color.NRGBA{G: 128, B: 255, A: 255},
			clip.Border{
				Rect:  layout.FRect(rect.Inset(-2)),
				Width: 4,
			}.Op(gtx.Ops),
		)
	}

	for _, p := range append(n.inports, n.outports...) {
		p.Layout(gtx)
	}

	return D{Size: size}
}

func (n *Node) layoutEditor(gtx C) {
	for _, e := range n.editor.Events() {
		switch e := e.(type) {
		case widget.ChangeEvent:
			n.validateEditor()
		case widget.SubmitEvent:
			n.setName(e.Text)
			n.editor = nil
			n.graph.arrange()
			n.graph.focus = n
			return
		}
	}

	spy, gtx := eventx.Enspy(gtx)
	th := th.WithPalette(material.Palette{
		Fg: color.NRGBA{A: 255},
	})
	layout.Center.Layout(gtx, material.Editor(&th, n.editor, "").Layout)

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
}

func (n *Node) edit() {
	n.editor = &widget.Editor{
		Alignment:  text.Middle,
		SingleLine: true,
		Submit:     true,
	}
	name := n.name()
	if name == "" {
		name = "x"
	}
	n.editor.SetText(name)
	n.editor.SetCaret(0, n.editor.Len())
	n.editor.Focus()
	n.graph.focus = nil
	n.oldText = name
}

func (n *Node) validateEditor() {
	if n.node.IsInport() || n.node.IsOutport() {
		if !token.IsIdentifier(n.editor.Text()) || n.editor.Text() == "_" {
			if n.editor.Text() == "" {
				n.editor.SetText("x")
				n.editor.SetCaret(0, 1)
			} else {
				n.editor.SetText(n.oldText)
			}
		}
	}
	n.oldText = n.editor.Text()
}

func (n *Node) name() string {
	if n.node.IsInport() {
		return n.node.Name[3:]
	}
	if n.node.IsOutport() {
		return n.node.Name[4:]
	}
	return n.node.Name
}

func (n *Node) setName(name string) {
	if n.node.IsInport() {
		n.node.Name = "in-" + name
	}
	if n.node.IsOutport() {
		n.node.Name = "out-" + name
	}
}
