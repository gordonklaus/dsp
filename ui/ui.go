package ui

import (
	"image/color"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/unit"
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

func px(gtx C, v float32) int {
	return gtx.Px(unit.Dp(v))
}

func pxpt(gtx C, pt f32.Point) f32.Point {
	return f32.Point{
		X: float32(px(gtx, pt.X)),
		Y: float32(px(gtx, pt.Y)),
	}
}

func dppt(gtx C, pt f32.Point) f32.Point {
	return f32.Point{
		X: pt.X / gtx.Metric.PxPerDp,
		Y: pt.Y / gtx.Metric.PxPerDp,
	}
}
