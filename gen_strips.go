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
	polygon_bg_col := [3]uint8{0xf7, 0xf2, 0xeb}
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
									if col.R == polygon_bg_col[0] && col.G == polygon_bg_col[1] && col.B == polygon_bg_col[2] {
										col.A = 0
										img.(draw.Image).Set(x, y, col)
									}
								}
							}

							img = imgDownsized(img, 4096, true)
							if dt.Weekday() == time.Sunday {
								fileWrite(sheetpngfilepath, pngEncode(img))
								pngOpt(sheetpngfilepath)
							} else {
								fileWrite(sheetpngfilepath, pngEncode(img.(*image.NRGBA).SubImage(
									image.Rect(0, 0, img.Bounds().Max.X, img.Bounds().Max.Y/2))))

								sheetpngfilepath2 := filepath.Join(dir, dt.AddDate(0, 0, 1).Format("2006-01-02")+".png")
								fileWrite(sheetpngfilepath2, pngEncode(img.(*image.NRGBA).SubImage(
									image.Rect(0, img.Bounds().Max.Y/2, img.Bounds().Max.X, img.Bounds().Max.Y))))

								pngOpt(sheetpngfilepath)
								pngOpt(sheetpngfilepath2)
							}

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
