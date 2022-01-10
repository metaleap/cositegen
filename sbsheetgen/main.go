package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"time"
)

const dpi1200 float64 = 47.2424242424

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var sb Storyboard
	err = json.Unmarshal(data, &sb)
	if err != nil {
		panic(err)
	}

	pngenc := png.Encoder{CompressionLevel: png.BestCompression}
	for _, page := range sb {
		println(page.Name + "...")
		w, h := 297.0*dpi1200, 210.0*dpi1200
		img := image.NewGray(image.Rect(0, 0, int(w), int(h)))
		for x := 0; x < int(w); x++ {
			for y := 0; y < int(h); y++ {
				img.SetGray(x, y, color.Gray{222})
			}
		}
		for _, panel := range page.Panels {
			bw := 4 * dpi1200
			imgBwBorder(img.SubImage(image.Rect(
				int((10.0*panel.CmX)*dpi1200),
				int((10.0*panel.CmY)*dpi1200),
				int(((10.0*panel.CmX)+(10.0*panel.CmW))*dpi1200),
				int(((10.0*panel.CmY)+(10.0*panel.CmH))*dpi1200),
			)).(draw.Image), color.Gray{0}, int(bw), 0)
		}

		var buf bytes.Buffer
		if err := pngenc.Encode(&buf, img); err != nil {
			panic(err)
		}
		if err := os.WriteFile(page.Name+"."+time.Now().Format("20060102")+".png", buf.Bytes(), os.ModePerm); err != nil {
			panic(err)
		}
	}
}

func imgBwBorder(imgdst draw.Image, col color.Gray, size int, offset int) {
	if size > 0 {
		for px := imgdst.Bounds().Min.X + offset; px < (imgdst.Bounds().Max.X - offset); px++ {
			for i := 0; i < size; i++ {
				imgdst.Set(px, imgdst.Bounds().Min.Y+i+offset, col)
				imgdst.Set(px, imgdst.Bounds().Max.Y-(i+1+offset), col)
			}
		}
		for py := imgdst.Bounds().Min.Y + offset; py < imgdst.Bounds().Max.Y-offset; py++ {
			for i := 0; i < size; i++ {
				imgdst.Set(imgdst.Bounds().Min.X+i+offset, py, col)
				imgdst.Set(imgdst.Bounds().Max.X-(i+1+offset), py, col)
			}
		}
	}
}
