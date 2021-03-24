package dsp

import (
	"go/types"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Graph struct {
	Name                     string
	InPorts, Nodes, OutPorts []*Node
}

type Node struct {
	Pkg, Name         string
	stateful          bool
	InPorts, OutPorts []*Port
	DelayWrite        *Node
}

type Port struct {
	Out   bool
	Name  string
	Node  *Node
	Conns []*Connection
}

type Connection struct {
	Src, Dst *Port
}

func NewNode(o types.Object) *Node {
	n := &Node{
		Pkg:  o.Pkg().Path(),
		Name: o.Name(),
	}
	switch o := o.(type) {
	case *types.TypeName:
		ms := types.NewMethodSet(types.NewPointer(o.Type()))
		ini := ms.Lookup(o.Pkg(), "Init")
		if ini == nil {
			return nil
		}
		sig, ok := ini.Type().(*types.Signature)
		if !ok {
			return nil
		}
		if sig.Params().Len() != 1 || sig.Results().Len() != 0 ||
			sig.Params().At(0).Type().String() != stdlib+".Config" {
			return nil
		}
		proc := ms.Lookup(o.Pkg(), "Process")
		if proc == nil {
			return nil
		}
		sig, ok = proc.Type().(*types.Signature)
		if !ok {
			return nil
		}
		n.stateful = true
		return n.init(sig)
	case *types.Func:
		return n.init(o.Type().(*types.Signature))
	}
	return nil
}

func (n *Node) init(sig *types.Signature) *Node {
	params := sig.Params()
	results := sig.Results()
	if params.Len() == 0 && results.Len() == 0 {
		return nil
	}
	for i := 0; i < params.Len(); i++ {
		v := params.At(i)
		n.InPorts = append(n.InPorts, &Port{
			Node: n,
			Name: v.Name()},
		)
		if t, ok := v.Type().(*types.Basic); !ok || t.Kind() != types.Float32 {
			return nil
		}
	}
	for i := 0; i < results.Len(); i++ {
		v := results.At(i)
		n.OutPorts = append(n.OutPorts, &Port{
			Out:  true,
			Node: n,
			Name: v.Name()},
		)
		if t, ok := v.Type().(*types.Basic); !ok || t.Kind() != types.Float32 {
			return nil
		}
	}
	return n
}

func NewPortNode(out bool) *Node {
	n := &Node{Name: "in-x"}
	if out {
		n.Name = "out-x"
	}
	p := &Port{Out: !out, Node: n}
	if out {
		n.InPorts = []*Port{p}
	} else {
		n.OutPorts = []*Port{p}
	}
	return n
}

func NewOperatorNode(op string) *Node {
	n := &Node{Name: op}
	n.InPorts = []*Port{{Node: n}, {Node: n}}
	n.OutPorts = []*Port{{Out: true, Node: n}}
	return n
}

const stdlib = "github.com/gordonklaus/dsp/dsp"

func NewDelayNode() *Node {
	n := &Node{
		Pkg:  stdlib,
		Name: "Delay",
	}
	n.DelayWrite = n
	n.InPorts = []*Port{{Node: n}, {Node: n}}
	n.OutPorts = []*Port{{Out: true, Node: n}}
	return n
}

func NewDelayReadNode(delay *Node) *Node {
	n := &Node{
		Pkg:        stdlib,
		Name:       "Delay",
		DelayWrite: delay.DelayWrite,
	}
	n.InPorts = []*Port{{Node: n}}
	n.OutPorts = []*Port{{Out: true, Node: n}}
	return n
}

func NewConstNode(text string) *Node {
	n := &Node{Name: text}
	n.OutPorts = []*Port{{Out: true, Node: n}}
	return n
}

func (n *Node) IsInport() bool     { return n.Pkg == "" && strings.HasPrefix(n.Name, "in-") }
func (n *Node) IsOutport() bool    { return n.Pkg == "" && strings.HasPrefix(n.Name, "out-") }
func (n *Node) IsDelay() bool      { return n.DelayWrite != nil }
func (n *Node) IsDelayWrite() bool { return n.DelayWrite == n }
func (n *Node) IsConst() bool {
	_, err := strconv.ParseFloat(n.Name, 64)
	return n.Pkg == "" && err == nil
}

func (n *Node) OutPortPos(p *Port) int {
	for i, p2 := range n.OutPorts {
		if p2 == p {
			return i
		}
	}
	panic("no such outport")
}

func (g *Graph) FileName() string   { return strings.ToLower(g.Name) + ".dsp" }
func (g *Graph) GoFileName() string { return g.FileName() + ".go" }

func (g *Graph) AllNodes() []*Node {
	return append(append(g.InPorts, g.Nodes...), g.OutPorts...)
}

func (g *Graph) Layers() ([][]*Node, map[*Node]int) {
	allNodes := g.AllNodes()
	if len(allNodes) == 0 {
		return nil, nil
	}

	nodeLayers := make(map[*Node]int, len(allNodes))
	firstLayer := 0
	var assignLayer func(n *Node, layer int)
	assignLayer = func(n *Node, layer int) {
		if nodeLayers[n] < layer {
			return
		}
		nodeLayers[n] = layer
		for _, p := range n.InPorts {
			for _, c := range p.Conns {
				assignLayer(c.Src.Node, layer-1)
			}
		}
		if firstLayer > layer {
			firstLayer = layer
		}
	}

	var sinks []*Node
sinks:
	for _, n := range allNodes {
		for _, p := range n.OutPorts {
			if len(p.Conns) > 0 {
				continue sinks
			}
		}
		sinks = append(sinks, n)
	}

	for _, n := range sinks {
		assignLayer(n, 0)
	}
	numLayers := 1 - firstLayer

	for _, n := range sinks {
		prevLayer := firstLayer - 1
		for _, p := range n.InPorts {
			for _, c := range p.Conns {
				if l := nodeLayers[c.Src.Node]; prevLayer < l {
					prevLayer = l
				}
			}
		}
		if prevLayer < firstLayer {
			nodeLayers[n] = firstLayer + numLayers/2
		} else {
			nodeLayers[n] = prevLayer + 1
		}
	}

	updatePorts := func(l1, l2 int, ports []*Node) bool {
		if len(ports) == 0 {
			return false
		}
		portsInLayer := 0
		for _, n := range ports {
			if nodeLayers[n] == l1 {
				portsInLayer++
			}
		}
		nodesInLayer := 0
		for _, n := range allNodes {
			if nodeLayers[n] == l1 {
				nodesInLayer++
			}
		}
		layerAdded := false
		if portsInLayer != nodesInLayer {
			l1 = l2
			layerAdded = true
		}
		for _, n := range ports {
			nodeLayers[n] = l1
		}
		return layerAdded
	}
	if updatePorts(firstLayer, firstLayer-1, g.InPorts) {
		firstLayer--
		numLayers++
	}
	if updatePorts(0, 1, g.OutPorts) {
		numLayers++
	}

	layers := make([][]*Node, numLayers)
	for _, n := range allNodes {
		nodeLayers[n] -= firstLayer
		l := nodeLayers[n]
		layers[l] = append(layers[l], n)
	}

	nodePositions := make(map[*Node]int, len(allNodes))
	for i, n := range g.InPorts {
		nodePositions[n] = i
	}
	for i, n := range g.OutPorts {
		nodePositions[n] = i
	}

	for i, l := range layers {
		if i == 0 && len(g.InPorts) > 0 {
			continue
		}
		if i == len(layers)-1 && len(g.OutPorts) > 0 {
			break
		}
		sort.Slice(l, func(i, j int) bool {
			n1 := l[i]
			n2 := l[j]
			if n1.Name != n2.Name {
				return n1.Name < n2.Name
			}
			if len(n1.InPorts) != len(n2.InPorts) {
				return len(n1.InPorts) < len(n2.InPorts)
			}
			if len(n1.OutPorts) != len(n2.OutPorts) {
				return len(n1.OutPorts) < len(n2.OutPorts)
			}
			for i, p1 := range n1.InPorts {
				p2 := n2.InPorts[i]
				if len(p1.Conns) != len(p2.Conns) {
					return len(p1.Conns) < len(p2.Conns)
				}
				for i, c1 := range p1.Conns {
					c2 := p2.Conns[i]
					if nodeLayers[c1.Src.Node] != nodeLayers[c2.Src.Node] {
						return nodeLayers[c1.Src.Node] < nodeLayers[c2.Src.Node]
					}
					if nodePositions[c1.Src.Node] != nodePositions[c2.Src.Node] {
						return nodePositions[c1.Src.Node] < nodePositions[c2.Src.Node]
					}
					n := c1.Src.Node
					if c1.Src != c2.Src {
						return n.OutPortPos(c1.Src) < n.OutPortPos(c2.Src)
					}
				}
			}
			for i, p1 := range n1.OutPorts {
				p2 := n2.OutPorts[i]
				if len(p1.Conns) != len(p2.Conns) {
					return len(p1.Conns) < len(p2.Conns)
				}
				for i, c1 := range p1.Conns {
					c2 := p2.Conns[i]
					if nodeLayers[c1.Src.Node] != nodeLayers[c2.Src.Node] {
						return nodeLayers[c1.Src.Node] < nodeLayers[c2.Src.Node]
					}
				}
			}
			return true
		})
		for i, n := range l {
			nodePositions[n] = i
		}
	}

	return layers, nodeLayers
}

func (g *Graph) Arrange() ([][]*Node, map[*Connection]*Connection) {
	layers, nodeLayers := g.Layers()
	if len(g.Nodes) == 0 {
		return layers, nil
	}

	fakeConns := map[*Connection]*Connection{}
	for _, n := range g.AllNodes() {
		layer := nodeLayers[n]
		for _, p := range n.InPorts {
			for _, c := range p.Conns {
				srcLayer := nodeLayers[c.Src.Node]
				if srcLayer < layer-1 {
					cc := &Connection{}
					fakeConns[c] = cc
					prevPort := c.Src
					for l := srcLayer + 1; l < layer; l++ {
						nn := &Node{}
						ip := &Port{Node: nn}
						ip.Conns = []*Connection{{
							Src: prevPort,
							Dst: ip,
						}}
						if l == srcLayer+1 {
							cc.Dst = ip
						} else {
							prevPort.Conns = []*Connection{ip.Conns[0]}
						}
						nn.InPorts = []*Port{ip}
						op := &Port{Out: true, Node: nn}
						nn.OutPorts = []*Port{op}
						layers[l] = append(layers[l], nn)
						prevPort = op
					}
					prevPort.Conns = []*Connection{{
						Src: prevPort,
						Dst: p,
					}}
					cc.Src = prevPort
				}
			}
		}
	}

	idst := func(c *Connection, l []*Node) int {
		i := 0
		for _, n := range l {
			for _, p := range n.InPorts {
				i++
				if p == c.Dst {
					return i
				}
			}
		}
		panic("unreached")
	}

	perms := make([][]int, len(layers))
	for i, l := range layers {
		perms[i] = make([]int, len(l))
	}
	bestPerms := make([][]int, len(layers))
	for i, l := range layers {
		bestPerms[i] = make([]int, len(l))
	}
	minCrossings := math.MaxInt64
perms:
	for {
		crossings := 0
		for i := range layers[:len(layers)-1] {
			l0 := getPerm(layers[i], perms[i])
			l1 := getPerm(layers[i+1], perms[i+1])
			i0a := 0
			for _, n := range l0 {
				for _, p := range n.OutPorts {
					i0a++
					for _, c := range p.Conns {
						if c2, ok := fakeConns[c]; ok {
							c = c2
						}
						i1a := idst(c, l1)

						i0b := 0
						for _, n := range l0 {
							for _, p := range n.OutPorts {
								i0b++
								for _, c := range p.Conns {
									if c2, ok := fakeConns[c]; ok {
										c = c2
									}
									i1b := idst(c, l1)

									if (i0b-i0a)*(i1b-i1a) < 0 {
										crossings++
									}
								}
							}
						}
					}
				}
			}
		}
		if minCrossings > crossings {
			minCrossings = crossings
			for i, p := range perms {
				copy(bestPerms[i], p)
			}
		}

		for i, p := range perms {
			if len(g.InPorts) > 0 && i == 0 {
				continue
			}

			nextPerm(p)
			if p[0] < len(p) {
				break
			}
			perms[i] = make([]int, len(p))

			lastLayer := len(perms) - 1
			if len(g.OutPorts) > 0 {
				lastLayer = len(perms) - 2
			}
			if i == lastLayer {
				break perms
			}
		}
	}

	for i, p := range bestPerms {
		layers[i] = getPerm(layers[i], p)
	}

	return layers, fakeConns
}

func nextPerm(p []int) {
	for i := len(p) - 1; i >= 0; i-- {
		if i == 0 || p[i] < len(p)-i-1 {
			p[i]++
			return
		}
		p[i] = 0
	}
}

func getPerm(orig []*Node, p []int) []*Node {
	result := append([]*Node{}, orig...)
	for i, v := range p {
		result[i], result[i+v] = result[i+v], result[i]
	}
	return result
}
