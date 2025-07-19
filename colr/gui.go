package main

import (
	"image"
	"image/color"
	"time"

	g "github.com/AllenDang/giu"
)

type Mode int

const (
	brushSizeMin = 3

	ModeNone Mode = iota
	ModeBrush
	ModeFill
)

var (
	idxImgSrc        = 0
	imgSrcShowFzoom  = false
	imgScreenPosMin  image.Point
	imgScreenPosMax  image.Point
	imgScreenPosRect image.Rectangle
	idxCurPanel      = -1
	mode             = ModeNone
	brushSize        = 11
	brushRecording   struct {
		is       bool
		idxPanel int
		moves    []image.Point
	}
)

func guiMain() {
	imgScreenPosMin = image.Pt(imgSize.Dx()/5, 55)
	imgScreenPosMax = image.Pt(5*(imgSize.Dx()/5), 55+4*(imgSize.Dy()/5))
	imgScreenPosRect = image.Rect(imgScreenPosMin.X, imgScreenPosMin.Y, imgScreenPosMax.X, imgScreenPosMax.Y)

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
		g.WindowShortcut{g.KeyLeft, g.ModNone, guiActionColSel(-1, 10)},
		g.WindowShortcut{g.KeyRight, g.ModNone, guiActionColSel(-1, 11)},
		g.WindowShortcut{g.KeyUp, g.ModNone, guiActionColSel(25, -1)},
		g.WindowShortcut{g.KeyDown, g.ModNone, guiActionColSel(26, -1)},
		g.WindowShortcut{g.KeyPageDown, g.ModNone, guiActionBrushDecr},
		g.WindowShortcut{g.KeyPageUp, g.ModNone, guiActionBrushIncr},
		g.WindowShortcut{g.KeyTab, g.ModNone, guiActionModeToggle},
		g.WindowShortcut{g.KeyEnter, g.ModNone, guiActionOnKeyEnter},
		g.WindowShortcut{g.KeyEscape, g.ModNone, guiActionOnKeyEscape},
		g.WindowShortcut{g.KeySpace, g.ModNone, guiActionOnKeySpace},
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
	wnd.SetTargetFPS(60)
	go func() {
		for range time.Tick(time.Millisecond * 16) {
			g.Update()
		}
	}()
	wnd.Run(guiLoop)
}

func guiLoop() {
	pos_mouse, pos_in_img := g.GetMousePos(), image.Point{-1, -1}
	mouse_in_img := (pos_mouse.X >= imgScreenPosMin.X) && (pos_mouse.X <= imgScreenPosMax.X) &&
		(pos_mouse.Y >= imgScreenPosMin.Y) && (pos_mouse.Y <= imgScreenPosMax.Y)
	if mouse_in_img {
		pos_in_img = image.Pt(pos_mouse.X-imgScreenPosMin.X, pos_mouse.Y-imgScreenPosMin.Y)
		pos_in_img.X = int(float64(pos_in_img.X) * (float64(imgSize.Dx()) / float64(imgScreenPosRect.Dx())))
		pos_in_img.Y = int(float64(pos_in_img.Y) * (float64(imgSize.Dy()) / float64(imgScreenPosRect.Dy())))
	}
	idxCurPanel = -1
	for i, panelrect := range pageLayout.panels {
		if pos_in_img.X >= panelrect.Min.X && pos_in_img.X <= panelrect.Max.X &&
			pos_in_img.Y >= panelrect.Min.Y && pos_in_img.Y <= panelrect.Max.Y {
			idxCurPanel = i
		}
	}

	if brushRecording.is {
		if idxCurPanel != brushRecording.idxPanel {
			guiActionOnKeySpace()
		} else if len(brushRecording.moves) == 0 || !brushRecording.moves[len(brushRecording.moves)-1].Eq(pos_in_img) {
			brushRecording.moves = append(brushRecording.moves, pos_in_img)
		}
	}

	cur_mouse_pointer := If(idxCurPanel >= 0, g.MouseCursorNone, g.MouseCursorArrow)
	if cur_mouse_pointer != g.MouseCursorArrow {
		g.SetMouseCursor(cur_mouse_pointer)
	}

	top_widget := "| M:" + If(mode == ModeBrush, "B", If(mode == ModeFill, "F", "_")) +
		" | B:" + i2s(brushSize) +
		" | P" + If(idxCurPanel >= 0, i2s(idxCurPanel+1), "_") + ":" + i2s(pos_in_img.X) + "," + i2s(pos_in_img.Y) +
		" | "

	widgets := []g.Widget{
		g.Label(top_widget),
		g.Label("F-Zoom: " + i2s(idxImgSrc) + "   [,][.][-]"),
		g.Separator(),
		g.Label("Color: " + colorLabels[idxColSelCur]),
		g.Separator(),
	}
	widgets = append(widgets,
		g.Custom(func() {
			canvas := g.GetCanvas()
			img_rect_color := If(mode == ModeNone, color.RGBA{0, 0, 0, 255}, color.RGBA{128, 128, 128, 255})
			if brushRecording.is {
				img_rect_color = color.RGBA{234, 123, 0, 255}
			}
			canvas.AddRect(imgScreenPosMin, imgScreenPosMax, img_rect_color, 22, g.DrawFlagsRoundCornersAll, 44)
			if mode == ModeBrush {
				canvas.AddRect(imgScreenPosMin, imgScreenPosMax, color.Black, 22, g.DrawFlagsRoundCornersAll, 22)
			}
			canvas.AddImage(imgDstTexture, imgScreenPosMin, imgScreenPosMax)
			canvas.AddImage(imgSrcTexture[If(imgSrcShowFzoom, idxImgSrc, 0)], imgScreenPosMin, imgScreenPosMax)
			if mode != ModeNone && cur_mouse_pointer == g.MouseCursorNone {
				brush_size := brushSize
				canvas.AddCircleFilled(pos_mouse, float32(brush_size), allColors[idxColSelCur])
				if mode == ModeBrush {
					canvas.AddCircle(pos_mouse, float32(brush_size), color.Black, 22, 1)
				}
			}
			// colors swatch
			idx_color, sc1, sc2 := 0, color.RGBA{177, 77, 0, 255}, color.RGBA{255, 188, 0, 255}
			for i, btnw, btnh, btnph, btnpv := 0, 37, 28, 1, 12; i < 24; i++ {
				for j := 0; j < 9; j++ {
					ptmin, ptmax := image.Pt(4+j*(btnw+btnph), 123+i*(btnh+btnpv)), image.Pt(4+j*(btnw+btnph)+btnw, 123+i*(btnh+btnpv)+btnh)
					canvas.AddRectFilled(ptmin, ptmax, allColors[idx_color], 11, g.DrawFlagsRoundCornersAll)
					if idx_color == idxColSelCur {
						canvas.AddRect(ptmin, ptmax, sc1, 11, g.DrawFlagsRoundCornersAll, 8)
						canvas.AddRect(ptmin, ptmax, sc2, 11, g.DrawFlagsRoundCornersAll, 3)
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

func guiActionBrushIncr() {
	brushSize++
}

func guiActionBrushDecr() {
	brushSize = If(brushSize == brushSizeMin, int(brushSizeMin), brushSize-1)
}

func guiActionFzoomToggle() {
	imgSrcShowFzoom = !imgSrcShowFzoom
	println(imgSrcShowFzoom)
}

func guiActionColSel(letter int, digit int) func() {
	return func() {
		if digit == 10 { // -1
			idxColSelDigit = If(idxColSelDigit == 0, 8, idxColSelDigit-1)
		} else if digit == 11 { // +1
			idxColSelDigit = If(idxColSelDigit == 8, 0, idxColSelDigit+1)
		}
		if letter == 25 { // -1
			idxColSelLetter = If(idxColSelLetter == 0, 23, idxColSelLetter-1)
		} else if letter == 26 { // +1
			idxColSelLetter = If(idxColSelLetter == 23, 0, idxColSelLetter+1)
		}
		idxColSelLetter = If(letter < 0 || letter > 23, idxColSelLetter, letter)
		idxColSelDigit = If(digit < 0 || digit > 8, idxColSelDigit, digit)
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

func guiActionUndo() {}
func guiActionRedo() {}

func guiActionModeToggle() {
	brushRecording.is, brushRecording.moves, brushRecording.idxPanel = false, nil, -1
	switch mode {
	case ModeNone:
		mode = ModeFill
	case ModeFill:
		mode = ModeBrush
	case ModeBrush:
		mode = ModeNone
	default:
		panic(mode)
	}
}

func guiActionOnKeySpace() {
	switch mode {
	case ModeBrush:
		if (!brushRecording.is) || len(brushRecording.moves) == 0 || brushRecording.idxPanel != idxCurPanel {
			brushRecording.is, brushRecording.moves, brushRecording.idxPanel = true, nil, idxCurPanel
			break
		}

		brushRecording.is = false
		for _, move := range brushRecording.moves {
			println(move.String())
		}
		println(len(brushRecording.moves))
	case ModeFill:
	}
}

func guiActionOnKeyEnter() {
}

func guiActionOnKeyEscape() {
	if brushRecording.is {
		brushRecording.moves, brushRecording.idxPanel = nil, idxCurPanel
	}
}
