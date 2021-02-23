package dsp

import (
	"go/types"
	"math"
)

type Graph struct {
	InPorts, Nodes, OutPorts []*Node
}

type Node struct {
	Name              string
	InPorts, OutPorts []*Port
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
		Name: o.Name(),
	}
	switch o := o.(type) {
	case *types.Func:
		sig := o.Type().(*types.Signature)
		params := sig.Params()
		results := sig.Results()
		if params.Len() == 0 || results.Len() == 0 {
			return nil
		}
		for i := 0; i < params.Len(); i++ {
			v := params.At(i)
			n.InPorts = append(n.InPorts, &Port{
				Node: n,
				Name: v.Name()},
			)
			if t, ok := v.Type().(*types.Basic); !ok || t.Kind() != types.Float64 {
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
			if t, ok := v.Type().(*types.Basic); !ok || t.Kind() != types.Float64 {
				return nil
			}
		}
		return n
	}
	return nil
}

func NewOperatorNode(op string) *Node {
	n := &Node{Name: op}
	n.InPorts = []*Port{{Node: n}, {Node: n}}
	n.OutPorts = []*Port{{Out: true, Node: n}}
	return n
}

func NewPortNode(out bool) *Node {
	n := &Node{Name: "inport"}
	if out {
		n.Name = "outport"
	}
	p := &Port{Out: !out, Node: n}
	if out {
		n.InPorts = []*Port{p}
	} else {
		n.OutPorts = []*Port{p}
	}
	return n
}

func (g *Graph) Arrange() ([][]*Node, map[*Connection]*Connection) {
	allNodes := append(append(g.InPorts, g.Nodes...), g.OutPorts...)
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

	if len(g.Nodes) == 0 {
		return layers, nil
	}

	fakeConns := map[*Connection]*Connection{}
	for _, n := range allNodes {
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

	inPortIndex := func(p *Port, l []*Node) int {
		for i, n := range l {
			for _, p2 := range n.InPorts {
				if p2 == p {
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
			for i0a, n := range l0 {
				for _, p := range n.OutPorts {
					for _, ca := range p.Conns {
						if c, ok := fakeConns[ca]; ok {
							ca = c
						}
						i1a := inPortIndex(ca.Dst, l1)

						for i0b, n := range l0[:i0a] {
							for _, p := range n.OutPorts {
								for _, cb := range p.Conns {
									if c, ok := fakeConns[cb]; ok {
										cb = c
									}
									if cb == ca {
										continue
									}

									i1b := inPortIndex(cb.Dst, l1)
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
