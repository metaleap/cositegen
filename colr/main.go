// Package main presents usage of giu.Canvas.
package main

import (
	"encoding/json"
	"image"
	"io"
	"os"

	g "github.com/AllenDang/giu"
)

var (
	imgSrc, imgDst               *image.RGBA
	imgSrcTexture, imgDstTexture *g.Texture
	imgSrcFilePath               string
	imgDstFilePath               string
	imgSize                      image.Rectangle
	pageLayout                   *PageLayout
)

type PageLayout struct {
	Page   image.Rectangle
	Panels []image.Rectangle
}

func main() {
	jsondata, err := io.ReadAll(os.Stdin)
	if (err != nil) || (len(jsondata) == 0) {
		panic(err)
	}
	if err = json.Unmarshal(jsondata, &pageLayout); (err != nil) || (pageLayout == nil) {
		panic(err)
	}

	imgSrcFilePath, imgDstFilePath = os.Args[1], os.Args[2]
	imgSrc, err = g.LoadImage(imgSrcFilePath)
	if err != nil {
		panic(err)
	}
	imgSize = imgSrc.Bounds()
	imgSrcEnsurePanelBorders()
	imgDst, err = g.LoadImage(imgDstFilePath)
	if err != nil {
		imgDst = imgDstNew(imgSize)
	}
	guiMain()
}
