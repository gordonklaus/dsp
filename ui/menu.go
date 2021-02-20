package ui

import (
	"go/types"
	"image/color"
	"log"
	"os"
	"sort"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/eventx"
	"github.com/gordonklaus/dsp"
	"golang.org/x/tools/go/packages"
)

type Menu struct {
	active        bool
	items         []*menuItem
	filteredItems []*menuItem
	selectedItem  *menuItem

	editor widget.Editor
	list   layout.List
}

type menuItem struct {
	pkg, name string
	obj       types.Object
}

func (it *menuItem) Layout(gtx C, selected bool) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			if !selected {
				return D{}
			}
			size := gtx.Constraints.Min
			paint.FillShape(gtx.Ops, color.NRGBA{R: 32, G: 0, B: 128, A: 255}, clip.Rect{Max: size}.Op())
			return D{Size: size}
		}),
		layout.Stacked(func(gtx C) D {
			children := []layout.FlexChild{}
			if it.pkg != "" {
				pkg := it.pkg
				if it.name != "" {
					pkg += "."
				}
				lbl := material.Body1(th, pkg)
				lbl.MaxLines = 1
				children = append(children, layout.Rigid(lbl.Layout))
			}
			if it.name != "" {
				lbl := material.Body1(th, it.name)
				lbl.Color = color.NRGBA{G: 255, A: 255}
				lbl.MaxLines = 1
				children = append(children, layout.Rigid(lbl.Layout))
			}
			children = append(children, layout.Flexed(1, func(gtx C) D {
				return D{Size: gtx.Constraints.Min}
			}))
			return layout.Flex{}.Layout(gtx, children...)
		}),
	)
}

func NewMenu() *Menu {
	items := initItems()
	return &Menu{
		items:         items,
		filteredItems: items,
		selectedItem:  items[0],
		editor: widget.Editor{
			SingleLine: true,
			Submit:     true,
		},
		list: layout.List{
			Axis: layout.Vertical,
		},
	}
}

func initItems() []*menuItem {
	cfg := &packages.Config{
		Mode: packages.NeedName,
		Dir:  "/Users/gordon/testpkgs",
		Env:  append(os.Environ(), "GO111MODULE=on"),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Println(err)
		return nil
	}

	cfg.Mode |= packages.NeedTypes
	thisPkgs, err := packages.Load(cfg, ".")
	if err != nil {
		log.Println(err)
		return nil
	}
	thisPkg := thisPkgs[0]

	var items []*menuItem
	for _, name := range thisPkg.Types.Scope().Names() {
		if o := thisPkg.Types.Scope().Lookup(name); dsp.NewNode(o) != nil {
			items = append(items, &menuItem{name: name, obj: o})
		}
	}
	for _, p := range pkgs {
		if p.ID == thisPkg.ID {
			continue
		}
		items = append(items, &menuItem{pkg: p.PkgPath})
	}
	return items
}

func (m *Menu) activate(e key.EditEvent) {
	m.active = true
	m.editor.SetText("")
	m.editor.Insert(e.Text)
}

func (m *Menu) filterItems() {
	m.filteredItems = nil
	for _, it := range m.items {
		if strings.Contains(strings.ToLower(it.name), strings.ToLower(m.editor.Text())) ||
			strings.Contains(strings.ToLower(it.pkg), strings.ToLower(m.editor.Text())) {
			m.filteredItems = append(m.filteredItems, it)
		} else if it == m.selectedItem {
			m.selectedItem = nil
			if len(m.filteredItems) > 0 {
				m.selectedItem = m.filteredItems[len(m.filteredItems)-1]
			}
		}
	}
	if m.selectedItem == nil && len(m.filteredItems) > 0 {
		m.selectedItem = m.filteredItems[0]
	}
}

func (m *Menu) expandPackage(item *menuItem) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax,
		Dir:  "/Users/gordon/testpkgs",
		Env:  append(os.Environ(), "GO111MODULE=on"),
	}
	pkgs, err := packages.Load(cfg, item.pkg)
	if err != nil {
		log.Println(err)
		return
	}
	pkg := pkgs[0]

	any := false
	for _, name := range pkg.Types.Scope().Names() {
		if o := pkg.Types.Scope().Lookup(name); dsp.NewNode(o) != nil {
			it := &menuItem{pkg: o.Pkg().Name(), name: name, obj: o}
			m.items = append(m.items, it)
			any = true
		}
	}
	if !any {
		return
	}
	for i, it := range m.items {
		if it == item {
			m.items = append(m.items[:i], m.items[i+1:]...)
			break
		}
	}
	sort.Slice(m.items, func(i, j int) bool {
		i1 := m.items[i]
		i2 := m.items[j]
		if (i1.obj == nil) != (i2.obj == nil) {
			return i1.obj != nil
		}
		if i1.pkg != i2.pkg {
			return i1.pkg < i2.pkg
		}
		return i1.name < i2.name
	})
	m.filterItems()
	m.selectedItem = m.filteredItems[0]
}

func (m *Menu) moveSelection(next bool) {
	for i, it := range m.filteredItems {
		if it == m.selectedItem {
			i--
			if next {
				i += 2
			}
			i = (i + len(m.filteredItems)) % len(m.filteredItems)
			m.selectedItem = m.filteredItems[i]

			if p := &m.list.Position; i <= p.First {
				p.First = i
				p.Offset = 0
			} else if p.First <= i-(p.Count-1) {
				p.First = i - (p.Count - 1)
				if p.Offset > 0 {
					p.First++
				}
				p.Offset = 0
			}

			break
		}
	}
}

func (m *Menu) Layout(gtx C) *dsp.Node {
	if !m.active {
		return nil
	}

	m.editor.Focus()

	for _, e := range m.editor.Events() {
		switch e.(type) {
		case widget.ChangeEvent:
			m.filterItems()
		case widget.SubmitEvent:
			if m.selectedItem != nil {
				if m.selectedItem.obj != nil {
					m.active = false
					return dsp.NewNode(m.selectedItem.obj)
				} else {
					m.expandPackage(m.selectedItem)
				}
			}
		}
	}

	layout.Flex{
		Spacing:   layout.SpaceSides,
		Alignment: layout.Baseline,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return D{
				Size:     gtx.Constraints.Min,
				Baseline: gtx.Constraints.Min.Y / 2,
			}
		}),
		layout.Rigid(func(gtx C) D {
			dims := widget.Border{
				Color:        color.NRGBA{255, 255, 255, 255},
				CornerRadius: unit.Dp(4),
				Width:        unit.Dp(1),
			}.Layout(gtx, func(gtx C) D {
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Px(unit.Sp(256))
					gtx.Constraints.Max.X = gtx.Px(unit.Sp(256))
					gtx.Constraints.Min.Y = 0
					gtx.Constraints.Max.Y /= 2
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(m.layoutEditor),
						layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
						layout.Rigid(func(gtx C) D {
							itemHeight := (&menuItem{pkg: " "}).Layout(gtx, false).Size.Y
							gtx.Constraints.Max.Y -= gtx.Constraints.Max.Y % itemHeight
							return m.list.Layout(gtx, len(m.filteredItems), func(gtx C, i int) D {
								it := m.filteredItems[i]
								return it.Layout(gtx, it == m.selectedItem)
							})
						}),
					)
				})
			})
			dims.Baseline = dims.Size.Y - 32 // TODO: figure out how to propagate baseline from editor outwards to here
			return dims
		}),
	)

	return nil
}

func (m *Menu) layoutEditor(gtx C) D {
	spy, gtx := eventx.Enspy(gtx)

	dims := material.Editor(th, &m.editor, "").Layout(gtx)

	for _, e := range spy.AllEvents() {
		for _, e := range e.Items {
			switch e := e.(type) {
			case key.Event:
				if e.State == key.Press {
					switch e.Name {
					case key.NameUpArrow, key.NameDownArrow:
						m.moveSelection(e.Name == key.NameDownArrow)
					case key.NameEscape:
						m.active = false
					}
				}
			}
		}
	}

	return dims
}
