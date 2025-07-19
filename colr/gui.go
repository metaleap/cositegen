package main

import (
	"image"
	"image/color"
	"time"

	g "github.com/AllenDang/giu"
)

var (
	idxImgSrc       = 0
	imgSrcShowFzoom = false
)

func guiMain() {
	wnd := g.NewMasterWindow(imgDstFilePath, 1920, 1200, g.MasterWindowFlagsMaximized)
	style := g.DefaultTheme()
	style.SetFontSize(30)
	wnd.SetStyle(style)
	keybinds := []g.WindowShortcut{
		g.WindowShortcut{g.KeyQ, g.ModControl, wnd.Close},
		g.WindowShortcut{g.KeyS, g.ModControl, imgDstSave},
		g.WindowShortcut{g.KeyY, g.ModControl, guiActionUndo},
		g.WindowShortcut{g.KeyY, g.ModControl | g.ModShift, guiActionRedo},
		g.WindowShortcut{g.KeyZ, g.ModControl, guiActionRedo},
		g.WindowShortcut{g.KeyPeriod, g.ModNone, guiActionFzoomIncr},
		g.WindowShortcut{g.KeyComma, g.ModNone, guiActionFzoomDecr},
		g.WindowShortcut{g.KeySlash, g.ModNone, guiActionFzoomToggle},
	}
	for letter := g.KeyA; letter <= g.KeyX; letter++ {
		keybinds = append(keybinds, g.WindowShortcut{letter, g.ModNone, guiActionColSel(int(letter-g.KeyA), -1)})
	}
	for digit := g.Key1; digit <= g.Key9; digit++ {
		keybinds = append(keybinds, g.WindowShortcut{digit, g.ModNone, guiActionColSel(-1, int(digit-g.Key1))})
	}
	wnd.RegisterKeyboardShortcuts(keybinds...)

	for i := 0; i < 10; i++ {
		g.EnqueueNewTextureFromRgba(imgSrc[i], func(tex *g.Texture) {
			imgSrcTexture[i] = tex
		})
	}
	g.EnqueueNewTextureFromRgba(imgDst, func(tex *g.Texture) {
		imgDstTexture = tex
	})
	go func() {
		for range time.Tick(time.Millisecond * 33) {
			g.Update()
		}
	}()
	wnd.Run(guiLoop)
}

func guiLoop() {
	pos := g.GetMousePos()
	widgets := []g.Widget{
		g.Label("F:100% |  B:0 | M:" + i2s(pos.X) + "," + i2s(pos.Y)),
		g.Label("F-Zoom: " + i2s(idxImgSrc) + "   [,][.][-]"),
		g.Separator(),
		g.Label("Color: " + colorLabels[idxColSelCur]),
		g.Separator(),
	}
	widgets = append(widgets,
		g.Custom(func() {
			canvas := g.GetCanvas()
			canvas.AddImage(imgDstTexture, image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			canvas.AddImage(imgSrcTexture[iIf(imgSrcShowFzoom, idxImgSrc, 0)], image.Pt(imgSize.Dx()/5, 55), image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5)))
			idx_color, sel_color := 0, color.RGBA{R: 255, G: 123, B: 0, A: 255}
			for i, btnw, btnh, btnph, btnpv := 0, 37, 28, 1, 12; i < 24; i++ {
				for j := 0; j < 9; j++ {
					ptmin, ptmax := image.Pt(4+j*(btnw+btnph), 123+i*(btnh+btnpv)), image.Pt(4+j*(btnw+btnph)+btnw, 123+i*(btnh+btnpv)+btnh)
					canvas.AddRectFilled(ptmin, ptmax, allColors[idx_color], 11, g.DrawFlagsRoundCornersAll)
					if idx_color == idxColSelCur {
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

func guiActionFzoomIncr() {
	if idxImgSrc < 9 {
		idxImgSrc++
	}
}

func guiActionFzoomDecr() {
	if idxImgSrc > 0 {
		idxImgSrc--
	}
}

func guiActionFzoomToggle() {
	imgSrcShowFzoom = !imgSrcShowFzoom
	println(imgSrcShowFzoom)
}

func guiActionUndo() {}
func guiActionRedo() {}
func guiActionColSel(letter int, digit int) func() {
	return func() {
		idxColSelLetter = iIf(letter < 0, idxColSelLetter, letter)
		idxColSelDigit = iIf(digit < 0, idxColSelDigit, digit)
		for l, idx := 0, 0; l < 24; l++ {
			for d := 0; d < 9; d++ {
				if l == idxColSelLetter && d == idxColSelDigit {
					idxColSelCur = idx
					return
				}
				idx++
			}
		}
	}
}
