package main

import (
	"image"
	"image/color"
	"strconv"

	g "github.com/AllenDang/giu"
)

var (
	idxImgSrc = 0
)

func guiMain() {
	wnd := g.NewMasterWindow(imgDstFilePath, 1920, 1200, g.MasterWindowFlagsMaximized)
	style := g.DefaultTheme()
	style.SetFontSize(30)
	wnd.SetStyle(style)
	wnd.RegisterKeyboardShortcuts(
		g.WindowShortcut{Key: g.KeyPeriod, Callback: func() {
			if idxImgSrc < 9 {
				idxImgSrc++
			}
		}},
		g.WindowShortcut{Key: g.KeyComma, Callback: func() {
			if idxImgSrc > 0 {
				idxImgSrc--
			}
		}},
	)

	for i := 0; i < 10; i++ {
		g.EnqueueNewTextureFromRgba(imgSrc[i], func(tex *g.Texture) {
			imgSrcTexture[i] = tex
		})
	}
	g.EnqueueNewTextureFromRgba(imgDst, func(tex *g.Texture) {
		imgDstTexture = tex
	})
	wnd.Run(loop)
}

func loop() {
	widgets := []g.Widget{
		g.Label("F: 100%  B: 0"),
		g.Label("F-Zoom: " + strconv.Itoa(idxImgSrc) + "   [,][.]"),
		g.Separator(),
		g.Label("Color: " + colorLabels[idxCurColor]),
		g.Separator(),
	}
	widgets = append(widgets,
		g.Custom(func() {
			canvas := g.GetCanvas()
			// pos := g.GetCursorScreenPos()
			canvas.AddImage(imgDstTexture, image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			canvas.AddImage(imgSrcTexture[idxImgSrc], image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			idx_color, sel_color := 0, color.RGBA{R: 255, G: 123, B: 0, A: 255}
			for i, btnw, btnh, btnph, btnpv := 0, 37, 28, 1, 12; i < 24; i++ {
				for j := 0; j < 9; j++ {
					ptmin, ptmax := image.Pt(4+j*(btnw+btnph), 123+i*(btnh+btnpv)), image.Pt(4+j*(btnw+btnph)+btnw, 123+i*(btnh+btnpv)+btnh)
					canvas.AddRectFilled(ptmin, ptmax, allColors[idx_color], 11, g.DrawFlagsRoundCornersAll)
					if idx_color == idxCurColor {
						canvas.AddRect(ptmin, ptmax, sel_color, 11, g.DrawFlagsRoundCornersAll, 4)
					}
					idx_color++
				}
			}
		}),
	)
	for i := 0; i < 24; i++ {
		var cells []g.Widget
		for j := 0; j < 9; j++ {
			letter, digit := 'A'+i, '1'+j
			cells = append(cells, g.Label(string(letter)+string(digit)))
		}
		widgets = append(widgets, g.Row(cells...))
	}

	g.SingleWindow().Layout(widgets...)
}
