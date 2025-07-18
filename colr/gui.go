package main

import (
	"image"

	g "github.com/AllenDang/giu"
)

func guiMain() {
	wnd := g.NewMasterWindow(imgDstFilePath, 1920, 1200, g.MasterWindowFlagsMaximized)
	style := g.DefaultTheme()
	style.SetFontSize(30)
	wnd.SetStyle(style)
	// wnd.RegisterKeyboardShortcuts()

	g.EnqueueNewTextureFromRgba(imgSrc, func(tex *g.Texture) {
		imgSrcTexture = tex
	})
	g.EnqueueNewTextureFromRgba(imgDst, func(tex *g.Texture) {
		imgDstTexture = tex
	})
	wnd.Run(loop)
}

func loop() {
	widgets := []g.Widget{g.Label("F: 100%"), g.Label("B: 0"), g.Separator(), g.Label("Color: " + colorLabels[idxCurColor]), g.Separator()}
	widgets = append(widgets,
		g.Custom(func() {
			canvas := g.GetCanvas()
			// pos := g.GetCursorScreenPos()
			canvas.AddImage(imgDstTexture, image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			canvas.AddImage(imgSrcTexture, image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			const btnw = 37
			const btnh = 28
			const btnph = 1
			const btnpv = 12
			idx_color := 0
			for i := 0; i < 24; i++ {
				for j := 0; j < 9; j++ {
					ptmin, ptmax := image.Pt(4+j*(btnw+btnph), 123+i*(btnh+btnpv)), image.Pt(4+j*(btnw+btnph)+btnw, 123+i*(btnh+btnpv)+btnh)
					if idx_color == idxCurColor {
						canvas.AddRect(ptmin, ptmax, allColors[idx_color], 11, g.DrawFlagsRoundCornersAll, 4)
					} else {
						canvas.AddRectFilled(ptmin, ptmax, allColors[idx_color], 11, g.DrawFlagsRoundCornersAll)
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
