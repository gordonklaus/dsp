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
	th.ContrastBg = color.NRGBA{R: 160, G: 224, B: 224, A: 255}
}

var (
	black = color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	gray  = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	blue  = color.NRGBA{R: 0, G: 128, B: 255, A: 255}
	red   = color.NRGBA{R: 255, G: 128, B: 0, A: 255}
)

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
