package desktop

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type gnomeHeader struct {
	dark        bool
	left, right []system.Action
}

var (
	headerClose, _    = widget.NewIcon(icons.NavigationClose)
	headerMinimize, _ = widget.NewIcon(icons.ContentRemove)
	headerMaximize, _ = widget.NewIcon(icons.NavigationFullscreen)
	headerRestore, _  = widget.NewIcon(icons.NavigationFullscreenExit)
)

func (h *gnomeHeader) layout(d *desktop, gtx layout.Context, deco *widget.Decorations) layout.Dimensions {
	bg, fg, button := hexColor(0xf6f5f4), hexColor(0x252525), hexColor(0xe6e5e4)
	if h.dark {
		bg, fg, button = hexColor(0x303030), hexColor(0xf4f4f4), hexColor(0x444444)
	}
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(43))
	controls := func(actions []system.Action) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(actions))
			for _, action := range actions {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					icon, label := headerClose, "Close"
					switch action {
					case system.ActionMinimize:
						icon, label = headerMinimize, "Minimize"
					case system.ActionMaximize:
						icon, label = headerMaximize, "Maximize"
						if deco.Maximized {
							icon, label = headerRestore, "Restore"
						}
					}
					style := material.IconButton(d.theme, deco.Clickable(action), icon, d.tr(label))
					style.Background, style.Color = button, fg
					style.Size, style.Inset = 16, layout.UniformInset(5)
					gtx.Constraints.Min = image.Point{}
					return layout.UniformInset(3).Layout(gtx, style.Layout)
				}))
			}
			gtx.Constraints.Min.Y = size.Y
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		}
	}

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: size}.Op())
	gtx.Constraints = layout.Exact(size)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Inset{Left: 6}.Layout(gtx, controls(h.left)) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, size.Y))
			return deco.LayoutMove(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, d.rawLabel(14, "Wireproxy Desktop", fg, font.Bold))
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: 6}.Layout(gtx, controls(h.right))
		}),
	)
}
