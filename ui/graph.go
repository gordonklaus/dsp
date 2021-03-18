package ui

import (
	"image"
	"image/color"
	"log"
	"math"
	"unicode"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/gordonklaus/dsp"
)

type Graph struct {
	graph *dsp.Graph

	offset f32.Point

	ports struct {
		in, out portsGroup
	}
	nodes []*Node
	conns []*Connection

	focus   interface{}
	focused bool

	menu *Menu
}

func NewGraph(name string) (*Graph, error) {
	g := &Graph{
		menu: NewMenu(),
	}
	g.ports.in.graph = g
	g.ports.out.graph = g
	g.ports.out.out = true
	g.focus = g

	if err := g.loadGraph(name); err != nil {
		return nil, err
	}

	return g, nil
}

func (g *Graph) loadGraph(name string) error {
	graph, err := dsp.LoadGraph(name)
	if err != nil {
		return err
	}
	g.graph = graph

	for _, n := range graph.InPorts {
		g.ports.in.nodes = append(g.ports.in.nodes, NewNode(n, g))
	}
	for _, n := range graph.OutPorts {
		g.ports.out.nodes = append(g.ports.out.nodes, NewNode(n, g))
	}
	for _, n := range graph.Nodes {
		g.nodes = append(g.nodes, NewNode(n, g))
	}
	connConns := map[*dsp.Connection]*Connection{}
	for _, n := range g.allNodes() {
		for _, p := range n.inports {
			for _, c := range p.port.Conns {
				cc, ok := connConns[c]
				if ok {
					cc.dst = p
				} else {
					cc = NewConnection(c, g, nil, p)
					g.conns = append(g.conns, cc)
					connConns[c] = cc
				}
				p.conns = append(p.conns, cc)
			}
		}
		for _, p := range n.outports {
			for _, c := range p.port.Conns {
				cc, ok := connConns[c]
				if ok {
					cc.src = p
				} else {
					cc = NewConnection(c, g, p, nil)
					g.conns = append(g.conns, cc)
					connConns[c] = cc
				}
				p.conns = append(p.conns, cc)
			}
		}
	}
	g.arrange()
	return nil
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
					center := layout.FPt(gtx.Constraints.Min).Mul(.5).Sub(g.offset)
					g.focusNearest(dppt(gtx, center), e.Name)
				}
			}
		case key.EditEvent:
			g.editEvent(e)
		case pointer.Event:
			switch e.Type {
			case pointer.Scroll:
				g.offset = g.offset.Sub(e.Scroll)
			}
		}
	}

	key.InputOp{Tag: g}.Add(gtx.Ops)
	if g.focus != nil {
		key.FocusOp{Tag: g.focus}.Add(gtx.Ops)
	}
	pointer.InputOp{
		Tag:   g,
		Types: pointer.Scroll,
	}.Add(gtx.Ops)

	layoutNodes, borderRect := g.recordNodeLayout(gtx)
	g.constrainOffset(gtx, borderRect)

	paint.Fill(gtx.Ops, black)

	st := op.Save(gtx.Ops)
	op.Offset(g.offset).Add(gtx.Ops)
	col := gray
	if g.focused {
		col = blue
	}
	r := float32(px(gtx, 4))
	paint.FillShape(gtx.Ops,
		col,
		clip.Stroke{
			Path:  clip.UniformRRect(layout.FRect(borderRect), r).Path(gtx.Ops),
			Style: clip.StrokeStyle{Width: r},
		}.Op(),
	)
	for _, c := range g.conns {
		c.Layout(gtx)
	}
	layoutNodes.Add(gtx.Ops)
	g.ports.in.layout(gtx, borderRect)
	g.ports.out.layout(gtx, borderRect)
	st.Load()

	if n := g.menu.Layout(gtx); n != nil {
		g.addNode(n)
	}

	return D{Size: gtx.Constraints.Min}
}

const layerGap = 128

func (g *Graph) recordNodeLayout(gtx C) (op.CallOp, image.Rectangle) {
	m := op.Record(gtx.Ops)
	r := image.ZR
	for _, n := range g.allNodes() {
		d := n.Layout(gtx)
		pos := pxpt(gtx, n.pos)
		r = r.Union(image.Rectangle{Max: d.Size}.Add(image.Pt(int(pos.X), int(pos.Y))))
	}

	marginX := px(gtx, layerGap)
	marginY := px(gtx, 32)
	nodeWidth := px(gtx, nodeWidth)
	if len(g.graph.InPorts) > 0 {
		r.Min.X += nodeWidth
	} else {
		r.Min.X -= marginX
	}
	if len(g.graph.OutPorts) > 0 {
		r.Max.X -= nodeWidth
	} else if len(g.graph.InPorts) > 0 || len(g.nodes) > 0 {
		r.Max.X += marginX
	}
	r.Min.Y -= marginY
	r.Max.Y += marginY
	return m.Stop(), r
}

func (g *Graph) constrainOffset(gtx C, borderRect image.Rectangle) {
	rect := image.Rectangle{Max: gtx.Constraints.Min}.Sub(image.Pt(int(g.offset.X), int(g.offset.Y)))
	marginRect := borderRect.Inset(px(gtx, -128))
	if marginRect.Dx() < rect.Dx() {
		g.offset.X = float32(rect.Dx()/2 - (marginRect.Max.X+marginRect.Min.X)/2)
	} else if marginRect.Min.X > rect.Min.X {
		g.offset.X = -float32(marginRect.Min.X)
	} else if marginRect.Max.X < rect.Max.X {
		g.offset.X = float32(rect.Dx() - marginRect.Max.X)
	}
	if marginRect.Dy() < rect.Dy() {
		g.offset.Y = float32(rect.Dy()/2 - (marginRect.Max.Y+marginRect.Min.Y)/2)
	} else if marginRect.Min.Y > rect.Min.Y {
		g.offset.Y = -float32(marginRect.Min.Y)
	} else if marginRect.Max.Y < rect.Max.Y {
		g.offset.Y = float32(rect.Dy() - marginRect.Max.Y)
	}
}

type portsGroup struct {
	graph   *Graph
	out     bool
	focused bool
	pos     f32.Point
	nodes   []*Node
}

func (p *portsGroup) position() f32.Point { return p.pos }

func (pg *portsGroup) new(by *Node, after bool) {
	i := 0
	for j, n := range pg.nodes {
		if n == by {
			i = j
			if after {
				i++
			}
			break
		}
	}
	n := dsp.NewPortNode(pg.out)
	if pg.out {
		pg.graph.graph.OutPorts = append(pg.graph.graph.OutPorts[:i], append([]*dsp.Node{n}, pg.graph.graph.OutPorts[i:]...)...)
	} else {
		pg.graph.graph.InPorts = append(pg.graph.graph.InPorts[:i], append([]*dsp.Node{n}, pg.graph.graph.InPorts[i:]...)...)
	}
	nn := NewNode(n, pg.graph)
	pg.nodes = append(pg.nodes[:i], append([]*Node{nn}, pg.nodes[i:]...)...)
	pg.graph.arrange()
	nn.edit()
}

func (pg *portsGroup) layout(gtx C, rect image.Rectangle) {
	for _, e := range gtx.Events(pg) {
		switch e := e.(type) {
		case key.FocusEvent:
			pg.focused = e.Focus
		case key.Event:
			if e.State == key.Press {
				switch e.Name {
				case key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow:
					pg.graph.focusNearest(pg.pos, e.Name)
				case key.NameEscape:
					pg.graph.focus = pg.graph
				}
			}
		case key.EditEvent:
			if e.Text == "," || e.Text == "<" {
				pg.new(nil, false)
			} else {
				pg.graph.editEvent(e)
			}
		}
	}

	if len(pg.nodes) == 0 {
		frect := layout.FRect(rect)
		pt := f32.Pt(frect.Min.X, (frect.Max.Y+frect.Min.Y)/2)
		if pg.out {
			pt.X = frect.Max.X
		}
		pg.pos = pt

		col := gray
		if pg.focused {
			col = blue
		}
		r := float32(px(gtx, 8))
		rr := f32.Pt(r, r)
		paint.FillShape(gtx.Ops,
			col,
			clip.Stroke{
				Path:  clip.UniformRRect(f32.Rectangle{Min: pt.Sub(rr), Max: pt.Add(rr)}, r).Path(gtx.Ops),
				Style: clip.StrokeStyle{Width: r},
			}.Op(),
		)
	}

	key.InputOp{Tag: pg}.Add(gtx.Ops)
}

func (g *Graph) editEvent(e key.EditEvent) {
	switch e.Text {
	case "+", "-", "*", "/":
		g.addNode(dsp.NewOperatorNode(e.Text))
	case "=":
		g.addNode(dsp.NewDelayNode())
	default:
		c := []rune(e.Text)[0]
		if unicode.IsLetter(c) {
			g.menu.activate(e)
		} else if unicode.IsNumber(c) {
			g.addNode(dsp.NewConstNode(e.Text)).edit()
		}
	}
}

func (g *Graph) addNode(n *dsp.Node) *Node {
	g.graph.Nodes = append(g.graph.Nodes, n)
	nn := NewNode(n, g)
	g.nodes = append(g.nodes, nn)
	g.arrange()
	g.focus = nn
	return nn
}

func (g *Graph) deleteNode(n *Node) {
	del := func(nodes *[]*Node, nnodes *[]*dsp.Node) {
		for i, n2 := range *nodes {
			if n2 == n {
				*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
				break
			}
		}
		for i, n2 := range *nnodes {
			if n2 == n.node {
				*nnodes = append((*nnodes)[:i], (*nnodes)[i+1:]...)
				break
			}
		}
	}
	for _, p := range append(n.inports, n.outports...) {
		for len(p.conns) > 0 {
			p.conns[0].delete()
		}
	}
	if n.node.IsInport() {
		del(&g.ports.in.nodes, &g.graph.InPorts)
	} else if n.node.IsOutport() {
		del(&g.ports.out.nodes, &g.graph.OutPorts)
	} else {
		del(&g.nodes, &g.graph.Nodes)
	}

	if n.node.IsDelayWrite() {
		for i := 0; i < len(g.nodes); i++ {
			n2 := g.nodes[i]
			if n2.node.DelayWrite == n.node {
				g.deleteNode(n2)
				i--
			}
		}
	}
}

func (g *Graph) arrange() {
	if g.graph.Name != "" {
		if err := g.graph.Save(); err != nil {
			log.Println(err)
		}
	}

	layers, fakeConns := g.graph.Arrange()
	nodeNodes := map[*dsp.Node]*Node{}
	for _, n := range g.allNodes() {
		nodeNodes[n.node] = n
	}
	for _, c := range g.conns {
		c.via = nil
	}
	for i, l := range layers {
		const layerWidth = nodeWidth + layerGap
		x := layerWidth * (float32(i) - float32(len(layers))/2)
		height := 0
		ys := make([]int, len(l))
		for i, n := range l {
			if i > 0 {
				height += 16
			}
			ys[i] = height
			if nn, ok := nodeNodes[n]; ok {
				height += nn.Layout(C{Ops: new(op.Ops)}).Size.Y
			}
		}

		for i, n := range l {
			y := float32(ys[i]) - float32(height/2)
			if nn, ok := nodeNodes[n]; ok {
				nn.target = f32.Pt(x, y)
				continue
			}

		outer:
			for {
				prev := n.InPorts[0].Conns[0].Src.Node
				if nn, ok := nodeNodes[prev]; ok {
					for _, p := range nn.outports {
						for _, c := range p.conns {
							if cc, ok := fakeConns[c.conn]; ok && cc.Dst.Node == n {
								c.via = append(c.via, f32.Pt(x+nodeWidth/2, y))
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

	delayCounts := map[*dsp.Node]int{}
	for _, n := range g.nodes {
		if n.node.IsDelay() {
			delayCounts[n.node.DelayWrite]++
		}
	}
	hue := 0.
	delayColors := map[*dsp.Node]color.NRGBA{}
	for _, n := range g.nodes {
		if delayCounts[n.node.DelayWrite] > 1 {
			if _, ok := delayColors[n.node.DelayWrite]; !ok {
				delayColors[n.node.DelayWrite] = hsv2rgb(hue, .5, 1)
				hue = math.Mod(hue+math.Phi, 1)
			}
			n.delayColor = delayColors[n.node.DelayWrite]
		} else {
			n.delayColor = color.NRGBA{}
		}
	}
}

func hsv2rgb(h, s, v float64) color.NRGBA {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := v - c

	var r, g, b float64
	switch int(h * 6) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.NRGBA{
		R: uint8(255 * (r + m)),
		G: uint8(255 * (g + m)),
		B: uint8(255 * (b + m)),
		A: 255,
	}
}

func (g *Graph) focusNearest(pt f32.Point, dir string) {
	all := g.allPorts()
	if len(g.ports.in.nodes) == 0 {
		all = append(all, &g.ports.in)
	}
	if len(g.ports.out.nodes) == 0 {
		all = append(all, &g.ports.out)
	}
	if x := g.nearest(all, pt, dir, nil); x != nil {
		g.focus = x
	}
}

type positioner interface {
	position() f32.Point
}

func (g *Graph) allNodes() []*Node {
	return append(append(g.ports.in.nodes, g.nodes...), g.ports.out.nodes...)
}

func (g *Graph) allPorts() []positioner {
	var ports []positioner
	for _, n := range g.allNodes() {
		for _, p := range append(n.inports, n.outports...) {
			ports = append(ports, p)
		}
	}
	return ports
}

func (g *Graph) nearestPort(pt f32.Point, dir string, filter func(*Port) bool) *Port {
	filter2 := func(p positioner) bool { return filter(p.(*Port)) }
	if filter == nil {
		filter2 = nil
	}
	nearest, _ := g.nearest(g.allPorts(), pt, dir, filter2).(*Port)
	return nearest
}

func (g *Graph) nearest(pp []positioner, pt f32.Point, dir string, filter func(positioner) bool) positioner {
	var nearest positioner
	minDistance := float32(math.MaxFloat32)
	for _, p := range pp {
		d := p.position().Sub(pt)
		if !inDirection(d, dir) || filter != nil && !filter(p) {
			continue
		}
		d2 := d.X*d.X + d.Y*d.Y
		if d2 < minDistance {
			minDistance = d2
			nearest = p
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
