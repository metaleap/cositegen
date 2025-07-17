// Package main presents usage of giu.Canvas.
package main

import (
	"image"
	"image/color"
	"os"

	g "github.com/AllenDang/giu"
)

var (
	imgSrcTexture *g.Texture
	imgDstTexture *g.Texture
	imgSize       image.Rectangle
	allColors     []color.RGBA
	keyedColors   [][]color.RGBA
	colorLabels   [216]string
	idxCurColor   int
)

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

var (
	imgSrcFilePath string
	imgDstFilePath string
)

func main() {
	imgSrcFilePath = os.Args[1]
	imgDstFilePath = os.Args[2]
	wnd := g.NewMasterWindow(imgDstFilePath, 1920, 1200, g.MasterWindowFlagsMaximized)
	style := g.DefaultTheme()
	style.SetFontSize(30)
	wnd.SetStyle(style)
	// wnd.RegisterKeyboardShortcuts()

	imgsrc, err := g.LoadImage(imgSrcFilePath)
	if err != nil {
		panic(err)
	}
	imgSize = imgsrc.Bounds()
	g.EnqueueNewTextureFromRgba(imgsrc, func(tex *g.Texture) {
		imgSrcTexture = tex
	})

	imgdst, err := g.LoadImage(imgDstFilePath)
	if err != nil {
		imgdst = image.NewRGBA(image.Rect(0, 0, imgSize.Dx(), imgSize.Dy()))
		for x := 0; x < imgSize.Dx(); x++ {
			for y := 0; y < imgSize.Dy(); y++ {
				imgdst.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	g.EnqueueNewTextureFromRgba(imgdst, func(tex *g.Texture) {
		imgDstTexture = tex
	})
	wnd.Run(loop)
}

func init() {
	var n = []uint8{0x33, 0x55, 0x88, 0xAA, 0xCC, 0xEE}
	for _, r := range n {
		for _, g := range n {
			for _, b := range n {
				allColors = append(allColors, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
	}
	allColors[0] = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	idx_color := 0
	for letter := 1; letter <= 24; letter++ {
		var digits []color.RGBA
		for digit := 1; digit <= 9; digit++ {
			colorLabels[idx_color] = string('A'+(letter-1)) + string('1'+(digit-1))
			digits = append(digits, allColors[idx_color])
			idx_color++
		}
		keyedColors = append(keyedColors, digits)
	}
}
