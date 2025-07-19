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
	pageLayout *PageLayout
)

type PageLayout struct {
	Page   image.Rectangle
	Panels []image.Rectangle
	panels []image.Rectangle
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
	imgSrc[0], err = g.LoadImage(imgSrcFilePath)
	if err != nil {
		panic(err)
	}
	imgSize = imgSrc[0].Bounds()
	imgSrcEnsurePanelBorders()
	for i, idx := 0.9, 1; i >= 0.1; i, idx = i-0.1, idx+1 {
		imgSrc[idx] = imgDownsized(imgSrc[0], int(float64(imgSize.Dx())*i))
	}

	imgdstfile, _ := os.Stat(imgDstFilePath)
	if imgdstfile == nil || imgdstfile.Size() <= 0 || imgdstfile.IsDir() {
		imgDst = imgDstNew(imgSize)
		imgDstSave()
		guiMsg("Created: %s", imgDstFilePath)
	} else {
		imgDst, err = g.LoadImage(imgDstFilePath)
		if err != nil {
			panic(err)
		}
		guiMsg("Loaded: %s", imgDstFilePath)
		imgDstOrig = imgDst
	}
	guiMain()
}
