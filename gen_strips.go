package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"path/filepath"
	"sync"
	"time"
)

func makeStrips(flags map[string]bool) {
	var work sync.WaitGroup
	const force_all = true
	const polygonBgCol = "#f7f2eb"
	var bookGen BookGen
	var did bool
	for _, strip := range App.Proj.Strips {
		series := App.Proj.seriesByName(strip)
		for _, chap := range series.Chapters {
			for _, sheet := range chap.sheets {
				for _, sv := range sheet.versions {
					dir := filepath.Join(sv.data.dirPath, "strip")
					if force_all || dirStat(dir) == nil {
						work.Add(1)
						go func(sv *SheetVer) {
							defer work.Done()
							dtName := sv.parentSheet.name
							dt, err := time.Parse("2006-01-02", dtName)
							_ = dt
							if err != nil {
								panic(err)
							}
							mkDir(dir)
							sheetsvgfilepath := filepath.Join(dir, dtName+".svg")
							sheetpngfilepath := filepath.Join(dir, dtName+".png")
							println(sheetpngfilepath)
							bookGen.genSheetSvg(sv, sheetsvgfilepath, false, "en", false, polygonBgCol)
							_ = imgAnyToPng(sheetsvgfilepath, sheetpngfilepath, 0, true, "")

							pngsrc := fileRead(sheetpngfilepath)
							img, _, err := image.Decode(bytes.NewReader(pngsrc))
							if err != nil {
								panic(err)
							}
							for x := 0; x < img.Bounds().Dx(); x++ {
								for y := 0; y < img.Bounds().Dy(); y++ {
									col := img.At(x, y).(color.NRGBA)
									if col.R == 0xf7 && col.G == 0xf2 && col.B == 0xeb {
										col.A = 0
										img.(draw.Image).Set(x, y, col)
									}
								}
							}
							pngsrc = pngEncode(img)
							fileWrite(sheetpngfilepath, pngsrc)
							pngsrc = imgDownsizedPng(bytes.NewReader(pngsrc), nil, 4096, true)
							fileWrite(sheetpngfilepath+".4096.png", pngsrc)
							work.Add(1)
							go func() {
								defer work.Done()
								pngOpt(sheetpngfilepath)
								pngOpt(sheetpngfilepath + ".4096.png")
							}()
						}(sv)
					}
				}
			}
		}
	}
	work.Wait()
	if did {
		App.Proj.save(false)
	}
}
