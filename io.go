package dsp

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

func (g *Graph) Save() error {
	nodes := []*Node{}
	layers, _ := g.Layers()
	for _, l := range layers {
		for _, n := range l {
			nodes = append(nodes, n)
		}
	}

	gg := graphGob{Name: g.Name}
	nodeIndex := map[*Node]int{}
	portIndex := map[*Port]int{}
	for i, n := range nodes {
		nodeIndex[n] = i
		for pi, p := range n.OutPorts {
			portIndex[p] = pi
		}
	}
	for i, n := range nodes {
		gg.Nodes = append(gg.Nodes, nodeGob{
			Pkg:  n.Pkg,
			Name: n.Name,
		})
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

	f, err := os.Create(g.FileName())
	if err != nil {
		return err
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(gg); err != nil {
		return err
	}

	dir, _ := filepath.Split(g.FileName())
	pkgName, err := pkgName(dir)
	if err != nil {
		return err
	}

	gof, err := os.Create(g.GoFileName())
	if err != nil {
		return err
	}
	defer gof.Close()

	fmt.Fprintf(gof, "package %s\n\n", pkgName)

	pkgNames := map[string]string{}
	pkgCount := 0
	for _, n := range nodes {
		if n.Pkg != "" {
			pkgNames[n.Pkg] = fmt.Sprint("p", pkgCount)
			pkgCount++
		}
	}
	if len(pkgNames) > 0 {
		imports := []string{}
		for p := range pkgNames {
			imports = append(imports, p)
		}
		sort.Strings(imports)
		fmt.Fprint(gof, "import (\n")
		for _, p := range imports {
			fmt.Fprintf(gof, "\t%s %q\n", pkgNames[p], p)
		}
		fmt.Fprint(gof, ")\n\n")
	}

	vars := map[*Port]string{}
	varCount := 0
	newVar := func(p *Port) string {
		v := fmt.Sprintf("v%d", varCount)
		vars[p] = v
		varCount++
		return v
	}
	getVar := func(p *Port) string {
		if len(p.Conns) > 0 {
			return vars[p.Conns[0].Src]
		}
		return "float32(0)"
	}

	fmt.Fprintf(gof, "func %s(", g.Name)
	if len(g.InPorts) > 0 {
		for i, n := range g.InPorts {
			if i > 0 {
				fmt.Fprint(gof, ", ")
			}
			fmt.Fprint(gof, newVar(n.OutPorts[0]))
		}
		fmt.Fprintf(gof, " float32")
	}
	fmt.Fprintf(gof, ") (")
	if len(g.OutPorts) > 0 {
		for i, n := range g.OutPorts {
			if i > 0 {
				fmt.Fprint(gof, ", ")
			}
			fmt.Fprint(gof, newVar(n.InPorts[0]))
		}
		fmt.Fprintf(gof, " float32")
	}
	fmt.Fprintf(gof, ") {")
	for _, n := range nodes[len(g.InPorts) : len(nodes)-len(g.OutPorts)] {
		fmt.Fprintf(gof, "\n\t")
		if len(n.OutPorts) > 0 {
			any := false
			for i, p := range n.OutPorts {
				if i > 0 {
					fmt.Fprint(gof, ", ")
				}
				if len(p.Conns) > 0 {
					fmt.Fprint(gof, newVar(p))
					any = true
				} else {
					fmt.Fprint(gof, "_")
				}
			}
			if any {
				fmt.Fprintf(gof, " := ")
			} else {
				fmt.Fprintf(gof, " = ")
			}
		}
		switch n.Name {
		case "+", "-", "*", "/":
			fmt.Fprintf(gof, "%s %s %s", getVar(n.InPorts[0]), n.Name, getVar(n.InPorts[1]))
		default:
			if pn, ok := pkgNames[n.Pkg]; ok {
				fmt.Fprintf(gof, "%s.%s(", pn, n.Name)
			} else {
				fmt.Fprintf(gof, "%s(", n.Name)
			}
			for i, p := range n.InPorts {
				if i > 0 {
					fmt.Fprint(gof, ", ")
				}
				fmt.Fprint(gof, getVar(p))
			}
			fmt.Fprint(gof, ")")
		}
	}
	if len(g.OutPorts) > 0 {
		fmt.Fprint(gof, "\n\treturn ")
		for i, n := range g.OutPorts {
			if i > 0 {
				fmt.Fprint(gof, ", ")
			}
			fmt.Fprint(gof, getVar(n.InPorts[0]))
		}
	}
	fmt.Fprintf(gof, "\n}")

	return nil
}

func pkgName(dir string) (string, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return "", err
	}
	if pkgs[0].Name == "" {
		dir, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		return filepath.Base(dir), nil
	}
	return pkgs[0].Name, nil
}

func LoadGraph(name string) (*Graph, error) {
	g := &Graph{}
	if name == "" {
		return g, nil
	}

	filename := name
	if !strings.HasSuffix(name, ".dsp") {
		g.Name = name
		filename = g.FileName()
	}

	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		if g.Name != "" {
			return g, nil
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gg := graphGob{}
	if err := gob.NewDecoder(f).Decode(&gg); err != nil {
		return nil, err
	}
	g.Name = gg.Name
	nodes := make([]*Node, len(gg.Nodes))
	for i, gn := range gg.Nodes {
		n, err := newNode(gn.Pkg, gn.Name)
		if err != nil {
			return nil, err
		}
		nodes[i] = n
		if n.Pkg == "" && n.Name == "in" {
			g.InPorts = append(g.InPorts, n)
		} else if n.Pkg == "" && n.Name == "out" {
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

func newNode(pkg, name string) (*Node, error) {
	if pkg != "" {
		cfg := &packages.Config{
			Mode: packages.NeedName | packages.NeedTypes,
			Dir:  ".",
		}
		pkgs, err := packages.Load(cfg, pkg)
		if err != nil {
			return nil, err
		}
		if len(pkgs[0].Errors) > 0 {
			return nil, fmt.Errorf("errors in package %q: %s", pkg, pkgs[0].Errors)
		}
		obj := pkgs[0].Types.Scope().Lookup(name)
		if obj == nil {
			return nil, fmt.Errorf("no name %q in package %q", name, pkg)
		}
		return NewNode(obj), nil
	}

	switch name {
	case "in":
		return NewPortNode(false), nil
	case "out":
		return NewPortNode(true), nil
	case "+", "-", "*", "/":
		return NewOperatorNode(name), nil
	}
	return nil, fmt.Errorf("unknown node %q", name)
}

type graphGob struct {
	Name  string
	Nodes []nodeGob
	Conns []connGob
}

type nodeGob struct {
	Pkg, Name string
}

type connGob struct {
	Src, SrcPort int
	Dst, DstPort int
}