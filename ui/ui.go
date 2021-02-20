package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var th = material.NewTheme(gofont.Collection())

func init() {
	th.Fg = color.NRGBA{255, 255, 255, 255}
	th.Bg = color.NRGBA{0, 0, 0, 255}
}
