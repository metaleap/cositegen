package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	g "github.com/AllenDang/giu"
)

type GuiMode int

const (
	guiBrushSizeMin = 3

	GuiModeColPick GuiMode = iota
	GuiModeBrush
	GuiModeFill
)

var (
	imgSrcTex        [10]*g.Texture
	imgDstTex        *g.Texture
	imgDstPreviewTex *g.Texture
	idxImgSrc        = 0
	imgSrcShowFzoom  = false
	imgScreenPosMin  image.Point
	imgScreenPosMax  image.Point
	imgScreenPosRect image.Rectangle
	idxCurPanel      = -1
	guiShowImgDst    = true
	guiShowImgSrc    = true
	guiMode          = GuiModeColPick
	guiBrush         struct {
		size     int
		isRec    bool
		idxPanel int
		moves    []image.Point
	}
	guiFill struct {
		idxPanel int
		move     image.Point
	}
	guiMousePosInImg image.Point
	guiUndoStack     []*image.RGBA
	guiRedoStack     []*image.RGBA
	guiLastMsg       string
)

func guiMain() {
	guiBrush.size = 11
	imgScreenPosMin = image.Pt(375, 69)
	imgScreenPosMax = image.Pt(1881, 0 /* Y fixed up below! */)
	canvasratio, canvaswidth := float64(imgSize.Dx())/float64(imgSize.Dy()), imgScreenPosMax.X-imgScreenPosMin.X
	canvasheight := int(float64(canvaswidth) / canvasratio)
	imgScreenPosMax.Y = imgScreenPosMin.Y + canvasheight
	imgScreenPosRect = image.Rect(imgScreenPosMin.X, imgScreenPosMin.Y, imgScreenPosMax.X, imgScreenPosMax.Y)

	wnd := g.NewMasterWindow(imgDstFilePath, 1920, 1200, g.MasterWindowFlagsMaximized)
	style := g.DefaultTheme()
	style.SetFontSize(30)
	wnd.SetStyle(style)

	keybinds := []g.WindowShortcut{
		g.WindowShortcut{g.KeyQ, g.ModControl, wnd.Close},
		g.WindowShortcut{g.KeyS, g.ModControl, imgDstSave},
		g.WindowShortcut{g.KeyR, g.ModControl | g.ModShift, imgDstReload},
		g.WindowShortcut{g.KeyY, g.ModControl, guiActionUndo},
		g.WindowShortcut{g.KeyY, g.ModControl | g.ModShift, guiActionRedo},
		g.WindowShortcut{g.KeyZ, g.ModControl, guiActionRedo},
		g.WindowShortcut{g.KeyDelete, g.ModControl | g.ModShift, guiActionClear},
		g.WindowShortcut{g.KeyF8, g.ModNone, guiActionBlurModeToggle},
		g.WindowShortcut{g.KeyF9, g.ModNone, guiActionBlurSizeDecr},
		g.WindowShortcut{g.KeyF10, g.ModNone, guiActionBlurSizeIncr},
		g.WindowShortcut{g.KeyF11, g.ModNone, guiActionToggleShowSrc},
		g.WindowShortcut{g.KeyF12, g.ModNone, guiActionToggleShowDst},
		g.WindowShortcut{g.KeyPeriod, g.ModNone, guiActionFzoomIncr},
		g.WindowShortcut{g.KeyComma, g.ModNone, guiActionFzoomDecr},
		g.WindowShortcut{g.KeySlash, g.ModNone, guiActionFzoomToggle},
		g.WindowShortcut{g.KeyLeft, g.ModNone, guiActionColSel(-1, 10)},
		g.WindowShortcut{g.KeyRight, g.ModNone, guiActionColSel(-1, 11)},
		g.WindowShortcut{g.KeyUp, g.ModNone, guiActionColSel(25, -1)},
		g.WindowShortcut{g.KeyDown, g.ModNone, guiActionColSel(26, -1)},
		g.WindowShortcut{g.KeyPageDown, g.ModNone, guiActionBrushDecr},
		g.WindowShortcut{g.KeyPageUp, g.ModNone, guiActionBrushIncr},
		g.WindowShortcut{g.KeySemicolon, g.ModNone, guiActionFSizeDecr},
		g.WindowShortcut{g.KeyApostrophe, g.ModNone, guiActionFSizeIncr},
		g.WindowShortcut{g.KeyEnter, g.ModNone, guiActionModeToggle},
		g.WindowShortcut{g.KeyTab, g.ModNone, guiActionOnModeCommit},
		g.WindowShortcut{g.KeyEscape, g.ModNone, guiActionOnModeDiscard},
		g.WindowShortcut{g.KeySpace, g.ModNone, guiActionOnModeDo},
	}
	for letter := g.KeyA; letter <= g.KeyX; letter++ {
		keybinds = append(keybinds, g.WindowShortcut{letter, g.ModNone, guiActionColSel(int(letter-g.KeyA), -1)})
	}
	for digit := g.Key1; digit <= g.Key9; digit++ {
		keybinds = append(keybinds, g.WindowShortcut{digit, g.ModNone, guiActionColSel(-1, int(digit-g.Key1))})
	}
	wnd.RegisterKeyboardShortcuts(keybinds...)

	for i := 0; i < 10; i++ {
		guiUpdateTex(&imgSrcTex[i], imgSrc[i])
	}
	guiUpdateTex(&imgDstTex, imgDst)
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

	if guiBrush.isRec {
		if idxCurPanel != guiBrush.idxPanel {
			imgDstBrushHaltRec(true)
		} else if !guiMousePosInImg.Eq(pos_in_img) {
			guiBrush.moves = append(guiBrush.moves, pos_in_img)
		}
	}
	guiMousePosInImg = pos_in_img
	guiFill.idxPanel = idxCurPanel
	guiMousePosInImg = pos_in_img

	cur_mouse_pointer := If(idxCurPanel < 0 || guiMode == GuiModeColPick, g.MouseCursorArrow, g.MouseCursorNone)
	if cur_mouse_pointer != g.MouseCursorArrow {
		g.SetMouseCursor(cur_mouse_pointer)
	}

	top_widget := "| Mode: " + If(guiMode == GuiModeBrush, "Brush", If(guiMode == GuiModeFill, "Fill", "Color-picking")) +
		" | U:" + i2s(len(guiUndoStack)) + " R:" + i2s(len(guiRedoStack)) +
		// " | FMsl:" + i2s(fillModeStatMaxStackLen) +
		" | " + guiLastMsg

	widgets := []g.Widget{
		g.Label(top_widget),
		g.Separator(),
		g.Label("F-Zoom: " + i2s(idxImgSrc) + " F-PxSize: " + i2s(fillPixelSize)),
		g.Label("B-Size: " + i2s(guiBrush.size) + " [PgDn][PgUp]"),
		g.Label("Bl: " + f2s(blurSizeFactor) + If(blurModeGaussian, "G", "B") + " [F8][F9][F10]"),
		g.Label("Panel" + If(idxCurPanel >= 0, i2s(idxCurPanel+1), "_") + ": " + i2s(pos_in_img.X) + "," + i2s(pos_in_img.Y)),
		g.Label("Color: " + colorLabels[idxColSelCur] + " (" + c2s(allColors[idxColSelCur]) + ")"),
		g.Separator(),
	}
	widgets = append(widgets,
		g.Custom(func() {
			canvas := g.GetCanvas()
			img_rect_color := If(guiMode == GuiModeColPick, color.RGBA{123, 123, 123, 255}, color.RGBA{234, 234, 234, 255})
			if guiBrush.isRec { // orange
				img_rect_color = color.RGBA{234, 123, 0, 255}
			} else if imgDstPreviewTex != nil { // green
				img_rect_color = color.RGBA{0, 234, 123, 255}
			}
			canvas.AddRectFilled(imgScreenPosMin, imgScreenPosMax, img_rect_color, 22, g.DrawFlagsRoundCornersAll)
			canvas.AddRect(imgScreenPosMin, imgScreenPosMax, img_rect_color, 22, g.DrawFlagsRoundCornersAll, 33)
			if guiMode == GuiModeBrush {
				canvas.AddRect(imgScreenPosMin, imgScreenPosMax, color.Black, 22, g.DrawFlagsRoundCornersAll, 22)
			}
			if guiShowImgDst {
				canvas.AddImage(If(imgDstPreviewTex == nil, imgDstTex, imgDstPreviewTex), imgScreenPosMin, imgScreenPosMax)
			}
			if guiShowImgSrc {
				canvas.AddImage(imgSrcTex[If(imgSrcShowFzoom, idxImgSrc, 0)], imgScreenPosMin, imgScreenPosMax)
			}
			if guiMode != GuiModeColPick && cur_mouse_pointer == g.MouseCursorNone {
				brush_size := guiBrush.size
				canvas.AddCircleFilled(pos_mouse, float32(brush_size), allColors[idxColSelCur])
				if guiMode == GuiModeBrush {
					canvas.AddCircle(pos_mouse, float32(brush_size+1), color.White, 22, 1)
					canvas.AddCircle(pos_mouse, float32(brush_size), color.Black, 22, 1)
				}
			}
			// colors swatch
			idx_color, sc1, sc2 := 0, color.RGBA{177, 77, 0, 255}, color.RGBA{255, 188, 0, 255}
			for i, btnw, btnh, btnph, btnpv := 0, 37, 28, 1, 12; i < 24; i++ {
				for j := 0; j < 9; j++ {
					const ymin = 224
					ptmin, ptmax := image.Pt(4+j*(btnw+btnph), ymin+i*(btnh+btnpv)), image.Pt(4+j*(btnw+btnph)+btnw, ymin+i*(btnh+btnpv)+btnh)
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

func guiUpdateTex(dst **g.Texture, src *image.RGBA) {
	*dst = nil // TODO: dispose?
	if src != nil {
		g.EnqueueNewTextureFromRgba(src, func(tex *g.Texture) {
			*dst = tex
		})
	}
}

func guiMsg(str string, args ...any) {
	guiLastMsg = "(" + time.Now().Format("15:04:05") + ")" + " " + fmt.Sprintf(str, args...)
}

func guiActionFzoomIncr() {
	if idxImgSrc < 9 {
		idxImgSrc++
		if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	}
}

func guiActionFzoomDecr() {
	if idxImgSrc > 0 {
		idxImgSrc--
		if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	}
}

func guiActionBrushIncr() {
	if imgDstPreviewTex != nil {
		guiBrush.size += 2
		if guiMode == GuiModeBrush {
			imgDstBrushHaltRec(true)
		} else if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	} else if !guiBrush.isRec {
		guiBrush.size += 2
	}
}

func guiActionFSizeDecr() {
	if fillPixelSize > 1 {
		fillPixelSize -= 2
		if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	}
}

func guiActionFSizeIncr() {
	if fillPixelSize < 21 {
		fillPixelSize += 2
		if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	}
}

func guiActionBrushDecr() {
	if imgDstPreviewTex != nil && guiBrush.size > guiBrushSizeMin {
		guiBrush.size -= 2
		if guiMode == GuiModeBrush {
			imgDstBrushHaltRec(true)
		} else if guiMode == GuiModeFill {
			imgDstFillPreview()
		}
	} else if !guiBrush.isRec {
		guiBrush.size = If(guiBrush.size == guiBrushSizeMin, int(guiBrushSizeMin), guiBrush.size-2)
	}
}

func guiActionFzoomToggle() {
	imgSrcShowFzoom = !imgSrcShowFzoom
	guiMsg(If(imgSrcShowFzoom, "Showing", "Hiding") + " current flood-fill line-art resolution, [-] to " + If(imgSrcShowFzoom, "hide", "show") + " again, [,][.] to change it")
}

func guiActionColSel(letter int, digit int) func() {
	return func() {
		idx_prev := idxColSelCur
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
					goto end
				}
				idx++
			}
		}
	end:
		guiMsg("Current color: " + c2s(allColors[idxColSelCur]))
		if idxColSelCur != idx_prev && imgDstPreviewTex != nil {
			if guiMode == GuiModeBrush {
				imgDstBrushHaltRec(true)
			} else if guiMode == GuiModeFill {
				imgDstFillPreview()
			}
		}
	}
}

func guiActionUndo() {
	if len(guiUndoStack) > 0 {
		imgDstBrushHaltRec(false)
		guiRedoStack = append(guiRedoStack, imgDst)
		imgDst = guiUndoStack[len(guiUndoStack)-1]
		guiUndoStack = guiUndoStack[:len(guiUndoStack)-1]
		guiUpdateTex(&imgDstTex, imgDst)
		imgDstPreview = nil
		guiUpdateTex(&imgDstPreviewTex, nil)
	}
}

func guiActionRedo() {
	if len(guiRedoStack) > 0 {
		imgDstBrushHaltRec(false)
		guiUndoStack = append(guiUndoStack, imgDst)
		imgDst = guiRedoStack[len(guiRedoStack)-1]
		guiRedoStack = guiRedoStack[:len(guiRedoStack)-1]
		guiUpdateTex(&imgDstTex, imgDst)
		imgDstPreview = nil
		guiUpdateTex(&imgDstPreviewTex, nil)
	}
}

func guiActionClear() {
	if len(guiUndoStack) == 0 || imgDst != guiUndoStack[len(guiUndoStack)-1] {
		guiUndoStack = append(guiUndoStack, imgDst)
	}
	guiRedoStack = nil
	imgDst = imgDstNew(imgSize)
	guiUpdateTex(&imgDstTex, imgDst)
	guiUpdateTex(&imgDstPreviewTex, nil)
	guiMsg("All background colors cleared, [Ctrl+Z] to undo")
}

func guiActionToggleShowDst() {
	guiShowImgDst = !guiShowImgDst
	guiMsg(If(guiShowImgDst, "Showing", "Hiding") + " background colors, [F12] to " + If(guiShowImgDst, "hide", "show") + " them again")
}

func guiActionToggleShowSrc() {
	guiShowImgSrc = !guiShowImgSrc
	guiMsg(If(guiShowImgSrc, "Showing", "Hiding") + " line art, [F11] to " + If(guiShowImgSrc, "hide", "show") + " it again")
}

func guiActionBlurModeToggle() {
	blurModeGaussian = !blurModeGaussian
	guiMsg("Blur mode changed to " + If(blurModeGaussian, "Gaussian", "Box") + " blur")
	if guiMode == GuiModeBrush {
		imgDstBrushHaltRec(true)
	} else if guiMode == GuiModeFill {
		imgDstFillPreview()
	}
}

func guiActionBlurSizeIncr() {
	for _, bsf := range blurSizeFactors {
		if bsf > blurSizeFactor {
			blurSizeFactor = bsf
			if guiMode == GuiModeBrush {
				imgDstBrushHaltRec(true)
			} else if guiMode == GuiModeFill {
				imgDstFillPreview()
			}
			break
		}
	}
}

func guiActionBlurSizeDecr() {
	factors := slices.Clone(blurSizeFactors)
	slices.Reverse(factors)
	for _, bsf := range factors {
		if bsf < blurSizeFactor {
			blurSizeFactor = bsf
			if guiMode == GuiModeBrush {
				imgDstBrushHaltRec(true)
			} else if guiMode == GuiModeFill {
				imgDstFillPreview()
			}
			break
		}
	}
}

func guiActionModeToggle() {
	guiUpdateTex(&imgDstPreviewTex, nil)
	imgDstPreview = nil
	switch guiMode {
	case GuiModeColPick:
		guiMode = GuiModeFill
		guiMsg("Mode selected: Fill")
	case GuiModeFill:
		guiMode = GuiModeBrush
		guiMsg("Mode selected: Brush")
	case GuiModeBrush:
		imgDstBrushHaltRec(false)
		guiMode = GuiModeColPick
		guiMsg("Mode selected: Color-picking")
	default:
		panic(guiMode)
	}
}

func guiActionOnModeDo() {
	switch guiMode {
	case GuiModeFill:
		guiFill.move = guiMousePosInImg
		imgDstFillPreview()
		guiMsg("[Tab] to keep, [Escape] or [Space] do discard")
	case GuiModeBrush:
		if (!guiBrush.isRec) || len(guiBrush.moves) == 0 || guiBrush.idxPanel != idxCurPanel {
			guiUpdateTex(&imgDstPreviewTex, nil)
			imgDstPreview = nil
			guiBrush.isRec, guiBrush.moves, guiBrush.idxPanel = true, nil, idxCurPanel
			guiMsg("Recording mouse-move brush strokes until the next [Space]...")
		} else {
			imgDstBrushHaltRec(true)
			guiMsg("[Tab] to keep, [Escape] or [Space] do discard")
		}
	case GuiModeColPick:
		if idxCurPanel >= 0 && !ptZ.Eq(guiMousePosInImg) {
			col_lineart := imgSrc[0].At(guiMousePosInImg.X, guiMousePosInImg.Y)
			img := If(imgDstPreviewTex != nil, imgDstPreview, imgDst)
			col_bg := img.At(guiMousePosInImg.X, guiMousePosInImg.Y).(color.RGBA)
			_, _, _, a := col_lineart.RGBA()
			idx_closest, dist_closest, letter_closest, digit_closest := -1, math.MaxInt, -1, -1
			if idx := 0; a == 0 {
				for l := 0; l < 24; l++ {
					for d := 0; d < 9; d++ {
						dist := (max(int(allColors[idx].R), int(col_bg.R)) - min(int(allColors[idx].R), int(col_bg.R))) +
							(max(int(allColors[idx].G), int(col_bg.G)) - min(int(allColors[idx].G), int(col_bg.G))) +
							(max(int(allColors[idx].B), int(col_bg.B)) - min(int(allColors[idx].B), int(col_bg.B)))
						if dist < dist_closest {
							dist_closest, idx_closest, letter_closest, digit_closest = dist, idx, l, d
						}
						idx++
					}
				}
			}
			guiMsg("Picked color is: " + If(a > 0, "(line art)", c2s(col_bg)+If(idx_closest < 0, "", ", closest match on swatch: "+string('A'+letter_closest)+string('1'+digit_closest)+" ("+c2s(allColors[If(idx_closest < 0, 0, idx_closest)])+")")))
		} else {
			guiMsg("Color-picking only happens inside panels!")
		}
	}
}

func guiActionOnModeCommit() {
	if imgDstPreview != nil {
		guiFill.move = ptZ
		guiRedoStack, guiUndoStack = nil, append(guiUndoStack, imgDst)
		imgDst = imgDstPreview
		imgDstPreview = nil
		guiUpdateTex(&imgDstPreviewTex, nil)
		guiUpdateTex(&imgDstTex, imgDst)
		guiMsg("Change committed")
	}
}

func guiActionOnModeDiscard() {
	imgDstBrushHaltRec(false)
	guiUpdateTex(&imgDstPreviewTex, nil)
	imgDstPreview = nil
	guiMsg("Change discarded")
}
