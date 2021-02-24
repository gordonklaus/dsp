package dsp

import (
	"encoding/gob"
	"fmt"
	"io"
)

func WriteGraph(w io.Writer, g *Graph) error {
	gg := graphGob{}
	nodeIndex := map[*Node]int{}
	portIndex := map[*Port]int{}
	nodes := append(append(g.InPorts, g.Nodes...), g.OutPorts...)
	for i, n := range nodes {
		nodeIndex[n] = i
		for pi, p := range n.OutPorts {
			portIndex[p] = pi
		}
	}
	for i, n := range nodes {
		gg.Nodes = append(gg.Nodes, nodeGob{Name: n.Name})
		for pi, p := range n.InPorts {
			for _, c := range p.Conns {
				gg.Conns = append(gg.Conns, connGob{
					Src:     nodeIndex[c.Src.Node],
					SrcPort: portIndex[c.Src],
					Dst:     i,
					DstPort: pi,
				})
			}
		}
	}
	return gob.NewEncoder(w).Encode(gg)
}

func ReadGraph(r io.Reader) (*Graph, error) {
	gg := graphGob{}
	if err := gob.NewDecoder(r).Decode(&gg); err != nil {
		return nil, err
	}
	g := &Graph{}
	nodes := make([]*Node, len(gg.Nodes))
	for i, gn := range gg.Nodes {
		n, err := newNode(gn.Name)
		if err != nil {
			return nil, err
		}
		nodes[i] = n
		if n.Name == "inport" {
			g.InPorts = append(g.InPorts, n)
		} else if n.Name == "outport" {
			g.OutPorts = append(g.OutPorts, n)
		} else {
			g.Nodes = append(g.Nodes, n)
		}
	}
	for _, c := range gg.Conns {
		if c.Src >= len(nodes) || c.Dst >= len(nodes) {
			return nil, fmt.Errorf("src (%d) or dst (%d) out of range (%d)", c.Src, c.Dst, len(nodes))
		}
		src := nodes[c.Src]
		dst := nodes[c.Dst]
		if c.SrcPort > len(src.OutPorts) {
			return nil, fmt.Errorf("src port index (%d) out of range (%d(", c.SrcPort, len(src.OutPorts))
		}
		if c.DstPort > len(dst.InPorts) {
			return nil, fmt.Errorf("dst port index (%d) out of range (%d(", c.DstPort, len(dst.InPorts))
		}
		cc := &Connection{
			Src: src.OutPorts[c.SrcPort],
			Dst: dst.InPorts[c.DstPort],
		}
		cc.Src.Conns = append(cc.Src.Conns, cc)
		cc.Dst.Conns = append(cc.Dst.Conns, cc)
	}
	return g, nil
}

func newNode(name string) (*Node, error) {
	switch name {
	case "inport":
		return NewPortNode(false), nil
	case "outport":
		return NewPortNode(true), nil
	case "+", "-", "*", "/":
		return NewOperatorNode(name), nil
	}
	return nil, fmt.Errorf("unknown node %q", name)
}

type graphGob struct {
	Nodes []nodeGob
	Conns []connGob
}

type nodeGob struct {
	Name string
}

type connGob struct {
	Src, SrcPort int
	Dst, DstPort int
}
