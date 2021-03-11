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
	for _, n := range append(append(g.nodes, g.ports.in.nodes...), g.ports.out.nodes...) {
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
					g.focusNearest(layout.FPt(gtx.Constraints.Min).Mul(.5).Sub(g.offset), e.Name)
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

	m := op.Record(gtx.Ops)
	borderRect := image.ZR
	for _, n := range append(append(g.nodes, g.ports.in.nodes...), g.ports.out.nodes...) {
		d := n.Layout(gtx)
		borderRect = borderRect.Union(image.Rectangle{Max: d.Size}.Add(image.Pt(int(n.pos.X), int(n.pos.Y))))
	}
	borderRect = borderRect.Inset(-32)
	if len(g.graph.InPorts) > 0 {
		borderRect.Min.X += 96
		if len(g.graph.OutPorts) == 0 {
			borderRect.Max.X += 96
		}
	}
	if len(g.graph.OutPorts) > 0 {
		borderRect.Max.X -= 96
		if len(g.graph.InPorts) == 0 {
			borderRect.Min.X -= 96
		}
	}
	g.constraintOffset(gtx, borderRect)
	call := m.Stop()

	paint.Fill(gtx.Ops, color.NRGBA{A: 255})

	st := op.Save(gtx.Ops)
	op.Offset(g.offset).Add(gtx.Ops)
	col := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	if g.focused {
		col = color.NRGBA{G: 128, B: 255, A: 255}
	}
	paint.FillShape(gtx.Ops,
		col,
		clip.Border{
			Rect:  layout.FRect(borderRect),
			Width: 4,
			SE:    4, SW: 4, NW: 4, NE: 4,
		}.Op(gtx.Ops),
	)
	for _, c := range g.conns {
		c.Layout(gtx)
	}
	call.Add(gtx.Ops)
	g.ports.in.layout(gtx, borderRect)
	g.ports.out.layout(gtx, borderRect)
	st.Load()

	if n := g.menu.Layout(gtx); n != nil {
		g.addNode(n)
	}

	return D{Size: gtx.Constraints.Min}
}

func (g *Graph) constraintOffset(gtx C, borderRect image.Rectangle) {
	rect := (image.Rectangle{Max: gtx.Constraints.Min}).Sub(image.Pt(int(g.offset.X), int(g.offset.Y)))
	marginRect := borderRect.Inset(-128)
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
		pt := image.Pt(rect.Min.X, (rect.Max.Y+rect.Min.Y)/2)
		if pg.out {
			pt.X = rect.Max.X
		}
		pg.pos = layout.FPt(pt)
		const rr = 8
		r := image.Rectangle{Min: pt, Max: pt}.Inset(-rr)
		col := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
		if pg.focused {
			col = color.NRGBA{G: 128, B: 255, A: 255}
		}
		paint.FillShape(gtx.Ops,
			col,
			clip.Border{
				Rect:  layout.FRect(r),
				Width: 2,
				SE:    rr, SW: rr, NW: rr, NE: rr,
			}.Op(gtx.Ops),
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
		if unicode.IsLetter([]rune(e.Text)[0]) {
			g.menu.activate(e)
		}
	}
}

func (g *Graph) addNode(n *dsp.Node) {
	g.graph.Nodes = append(g.graph.Nodes, n)
	nn := NewNode(n, g)
	g.nodes = append(g.nodes, nn)
	g.arrange()
	g.focus = nn
}

func (g *Graph) arrange() {
	if g.graph.Name != "" {
		if err := g.graph.Save(); err != nil {
			log.Println(err)
		}
	}

	layers, fakeConns := g.graph.Arrange()
	nodeNodes := map[*dsp.Node]*Node{}
	for _, n := range append(append(g.ports.in.nodes, g.nodes...), g.ports.out.nodes...) {
		nodeNodes[n.node] = n
	}
	for _, c := range g.conns {
		c.via = nil
	}
	for i, l := range layers {
		x := 192 * (float32(i) - float32(len(layers))/2)
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

func (g *Graph) allPorts() []positioner {
	var ports []positioner
	for _, n := range append(append(g.nodes, g.ports.in.nodes...), g.ports.out.nodes...) {
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
