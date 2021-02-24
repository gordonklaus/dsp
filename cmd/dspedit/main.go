package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"github.com/gordonklaus/dsp/ui"
)

func main() {
	go Main()
	app.Main()
}

func Main() {
	w := app.NewWindow(app.Title("DSP"), app.Size(unit.Dp(800), unit.Dp(600)))

	filename := ""
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}
	graph := ui.NewGraph(filename)

	var ops op.Ops
	for e := range w.Events() {
		switch e := e.(type) {
		case system.DestroyEvent:
			if e.Err != nil {
				log.Fatal(e.Err)
			}
			return
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			graph.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
