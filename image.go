package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	pnm "github.com/go-forks/gopnm"
	"golang.org/x/image/draw"
)

var PngEncoder = png.Encoder{CompressionLevel: png.NoCompression}
var ImgScaler draw.Interpolator = draw.CatmullRom

func pngEncode(img image.Image) []byte {
	var buf bytes.Buffer
	if err := PngEncoder.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func imgPnmToPng(srcImgData io.ReadCloser, dstImgFile io.WriteCloser, ensureWide bool, snipTop int, snipRight int, snipBottom int, snipLeft int) {
	srcimg, err := pnm.Decode(srcImgData)
	if err != nil {
		panic(err)
	}
	_ = srcImgData.Close()

	if dstbounds := srcimg.(*image.Gray).Bounds(); ensureWide && dstbounds.Max.X < dstbounds.Max.Y {
		dstbounds.Max.X, dstbounds.Max.Y = dstbounds.Max.Y, dstbounds.Max.X
		dstimg, srcbounds := image.NewGray(dstbounds), srcimg.Bounds()
		for dstx := 0; dstx < dstbounds.Max.X; dstx++ {
			for dsty := 0; dsty < dstbounds.Max.Y; dsty++ {
				srcx, srcy := dsty, (srcbounds.Max.Y-1)-dstx
				dstimg.Set(dstx, dsty, srcimg.At(srcx, srcy))
			}
		}
		srcimg = dstimg
	}
	if snipTop > 0 || snipBottom > 0 || snipLeft > 0 || snipRight > 0 {
		srcimg = srcimg.(*image.Gray).SubImage(image.Rect(snipLeft, snipTop, srcimg.Bounds().Max.X-snipRight, srcimg.Bounds().Max.Y-snipBottom))
	}
	if err := PngEncoder.Encode(dstImgFile, srcimg); err != nil {
		panic(err)
	}
	_ = dstImgFile.Close()
}

func imgAnyToPng(srcFilePath string, outFilePath string, reSize int, noTmpFile bool, tmpFileNamePrefix string) (writtenFilePath string) {
	srcdata := fileRead(srcFilePath)
	chash := contentHashStr(srcdata)
	tmpfilepath := ".ccache/.pngtmp/" + tmpFileNamePrefix + chash + "." + itoa(reSize) + ".png"
	if noTmpFile {
		tmpfilepath = outFilePath
	}
	if noTmpFile || fileStat(tmpfilepath) == nil {
		// os.WriteFile("/home/_/tmp/pix/bla/"+strings.Replace(srcFilePath, "/", "_", -1)+"."+chash, srcdata, os.ModePerm)
		if src := string(srcdata); strings.HasSuffix(srcFilePath, ".svg") {
			sw, sh := src[7+strings.Index(src, "width=\""):], src[8+strings.Index(src, "height=\""):]
			sw, sh = strings.TrimSuffix(sw[:strings.IndexByte(sw, '"')], "px"), strings.TrimSuffix(sh[:strings.IndexByte(sh, '"')], "px")
			w, h := atoi(sw, 0, 22222), atoi(sh, 0, 11111)
			if w == 0 || h == 0 {
				panic(sw + "x" + sh)
			}
			if s := osExec(false, nil, browserCmd[0], append(browserCmd[2:],
				// "--headless",
				"--window-size="+itoa(w)+","+itoa(h)+"",
				"--screenshot="+tmpfilepath, "--default-background-color=00000000", srcFilePath)...); strings.Contains(s, "tile memory limits") {
				panic(s)
			} else if fstat := fileStat(tmpfilepath); fstat == nil || fstat.Size() == 0 {
				panic(s)
			}
			if reSize != 0 && reSize != w {
				_ = osExec(true, nil, "mogrify",
					"-quality", "00", "-resize", itoa(reSize), tmpfilepath)
			}
		} else {
			cmdargs := []string{srcFilePath,
				"-quality", "00",
				"-background", "white",
				"-alpha", "remove",
				"-alpha", "off"}
			if reSize != 0 {
				cmdargs = append(cmdargs, "-resize", itoa(reSize))
			}
			_ = osExec(true, nil, "convert", append(cmdargs, tmpfilepath)...)
		}
		writtenFilePath = tmpfilepath
	}
	if !noTmpFile {
		fileLinkOrCopy(tmpfilepath, outFilePath)
	}
	return
}

func imgDownsizedPng(srcImgData io.Reader, onDecoded func() error, maxWidth int, transparent bool) []byte {
	return pngEncode(imgDownsizedReader(srcImgData, onDecoded, maxWidth, transparent))
}

func imgDownsizedReader(srcImgData io.Reader, onDecoded func() error, maxWidth int, transparent bool) draw.Image {
	imgsrc, _, err := image.Decode(srcImgData)
	if onDecoded != nil {
		_ = onDecoded() // allow early file-closing for the caller
	}
	if err != nil {
		panic(err)
	}
	return imgDownsized(imgsrc, maxWidth, transparent)
}

func imgDownsized(imgSrc image.Image, maxWidth int, transparent bool) draw.Image {
	origwidth, origheight := imgSrc.Bounds().Max.X, imgSrc.Bounds().Max.Y
	if origwidth <= maxWidth && !transparent {
		return nil
	}

	switch imgsrcgray := imgSrc.(type) {
	case *image.Gray:
		if transparent {
			img := image.NewNRGBA(imgSrc.Bounds())
			for x := 0; x < imgSrc.Bounds().Max.X; x++ {
				for y := 0; y < imgSrc.Bounds().Max.Y; y++ {
					img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255 - imgsrcgray.GrayAt(x, y).Y})
				}
			}
			imgSrc = img
		}
	}

	newheight := int(float64(origheight) / (float64(origwidth) / float64(maxWidth)))
	var imgdown draw.Image
	if transparent {
		imgdown = image.NewNRGBA(image.Rect(0, 0, maxWidth, newheight))
	} else {
		imgdown = image.NewGray(image.Rect(0, 0, maxWidth, newheight))
	}
	ImgScaler.Scale(imgdown, imgdown.Bounds(), imgSrc, imgSrc.Bounds(), draw.Over, nil)
	return imgdown
}

func imgFill(img draw.Image, r image.Rectangle, c color.Color) {
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			img.Set(x, y, c)
		}
	}
}

func imgGrayDistrs(srcImgData io.Reader, onDecoded func() error, numClusters int) (grayDistrs []int, colDarkestLightest []uint8) {
	imgsrc, _, err := image.Decode(srcImgData)
	if onDecoded != nil {
		_ = onDecoded() // allow early file-closing for the caller
	}
	if err != nil {
		panic(err)
	}

	grayDistrs, colDarkestLightest = make([]int, numClusters), []uint8{255, 0}
	m := 256.0 / float64(numClusters)
	for px := 0; px < imgsrc.Bounds().Max.X; px++ {
		for py := 0; py < imgsrc.Bounds().Max.Y; py++ {
			var cm uint8 // ensure grayscale
			switch colsrc := imgsrc.At(px, py).(type) {
			case color.Gray:
				cm = colsrc.Y
			case color.RGBA:
				cm = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			case color.NRGBA:
				cm = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			default:
				panic(colsrc)
			}
			if cm < colDarkestLightest[0] {
				colDarkestLightest[0] = cm
			}
			if cm > colDarkestLightest[1] {
				colDarkestLightest[1] = cm
			}
			grayDistrs[int(float64(cm)/m)]++
		}
	}
	return
}

func imgToMonochromePng(srcImgData io.Reader, onDecoded func() error, blackIfLessThan uint8) []byte {
	return pngEncode(imgToMonochrome(srcImgData, onDecoded, blackIfLessThan))
}

// returns BW-thresholded by blackIfLessThan, or unthresholded grayscale if blackIfLessThan==0.
func imgToMonochrome(srcImgData io.Reader, onDecoded func() error, blackIfLessThan uint8) *image.Gray {
	srcimg, _, err := image.Decode(srcImgData)
	if onDecoded != nil {
		_ = onDecoded() // allow early file-closing for the caller
	}
	if err != nil {
		panic(err)
	}

	imggray := image.NewGray(image.Rect(0, 0, srcimg.Bounds().Max.X, srcimg.Bounds().Max.Y))
	for px := 0; px < srcimg.Bounds().Max.X; px++ {
		for py := 0; py < srcimg.Bounds().Max.Y; py++ {
			var colbw uint8
			// ensure grayscale
			switch colsrc := srcimg.At(px, py).(type) {
			case color.Gray:
				colbw = colsrc.Y
			case color.RGBA:
				colbw = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			case color.NRGBA:
				colbw = uint8((int(colsrc.R) + int(colsrc.G) + int(colsrc.B)) / 3)
			default:
				panic(colsrc)
			}
			// threshold
			if blackIfLessThan > 0 {
				if colbw < blackIfLessThan {
					colbw = 0
				} else {
					colbw = 255
				}
			}

			imggray.Set(px, py, color.Gray{Y: colbw})
		}
	}
	return imggray
}

func imgIsRectFullyOfColor(img *image.Gray, rect image.Rectangle, col color.Gray) bool {
	for x := rect.Min.X; x < rect.Max.X; x++ {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if img.GrayAt(x, y).Y != col.Y {
				return false
			}
		}
	}
	return true
}

func imgBwBorder(imgdst draw.Image, bwColor color.Gray, size int, offset int, transparent bool) {
	if size > 0 {
		var col color.Color = bwColor
		if transparent {
			col = color.NRGBA{R: 0, G: 0, B: 0, A: 255 - bwColor.Y}
		}
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

func imgSubRectSvg(srcImg *image.Gray, srcImgRect image.Rectangle, blackBorderSize int) (ret []byte) {
	if blackBorderSize != 0 {
		imgDrawRect(srcImg, srcImgRect, blackBorderSize, 0)
	}
	var buf bytes.Buffer
	name := strconv.FormatInt(time.Now().UnixNano(), 36)
	pnmpath, svgpath := "/dev/shm/"+name+".pbm", "/dev/shm/"+name+".svg"
	if err := pnm.Encode(&buf, srcImg.SubImage(srcImgRect), pnm.PBM); err != nil {
		panic(err)
	}
	fileWrite(pnmpath, buf.Bytes())
	osExec(true, nil, "potrace", "-s", pnmpath, "-o", svgpath)
	ret = fileRead(svgpath)
	_, _ = os.Remove(pnmpath), os.Remove(svgpath)
	return
}

func imgSubRectPngFile(srcImgFilePath string, rect image.Rectangle, blackBorderSize int, reWidth int, transparent bool) []byte {
	imgsrc, _, err := image.Decode(bytes.NewReader(fileRead(srcImgFilePath)))
	if err != nil {
		panic(err)
	}
	var gotsamesizeasorig bool
	w, h := rect.Dx(), rect.Dy()
	if reWidth != 0 && reWidth != imgsrc.Bounds().Dx() {
		factor := float64(imgsrc.Bounds().Dx()) / float64(reWidth)
		w, h = int(float64(w)/factor), int(float64(h)/factor)
	}
	return pngEncode(imgSubRect(imgsrc.(*image.Gray), rect, &w, &h, blackBorderSize, transparent, &gotsamesizeasorig))
}

func imgSubRectPng(srcImg *image.Gray, srcImgRect image.Rectangle, width *int, height *int, blackBorderSize int, transparent bool, gotSameSizeAsOrig *bool) []byte {
	return pngEncode(imgSubRect(srcImg, srcImgRect, width, height, blackBorderSize, transparent, gotSameSizeAsOrig))
}

func imgSubRect(srcImg *image.Gray, srcImgRect image.Rectangle, width *int, height *int, blackBorderSize int, transparent bool, gotSameSizeAsOrig *bool) image.Image {
	origwidth, origheight := srcImgRect.Dx(), srcImgRect.Dy()
	assert(((*width < origwidth) == (*height < origheight)) &&
		((*width > origwidth) == (*height > origheight)))

	var srcimg draw.Image = srcImg
	if transparent {
		srcimg = image.NewNRGBA(image.Rect(0, 0, srcImg.Bounds().Max.X, srcImg.Bounds().Max.Y))
		for px := srcImgRect.Min.X; px < srcImgRect.Max.X; px++ {
			for py := srcImgRect.Min.Y; py < srcImgRect.Max.Y; py++ {
				col := srcImg.GrayAt(px, py)
				srcimg.Set(px, py, color.NRGBA{R: 0, G: 0, B: 0, A: 255 - col.Y})
			}
		}
	}

	var imgdst draw.Image
	if *width >= origwidth {
		*gotSameSizeAsOrig, *width, *height = true, origwidth, origheight
		if !transparent {
			imgdst = srcImg.SubImage(srcImgRect).(draw.Image)
		} else {
			imgdst = srcimg.(*image.NRGBA).SubImage(srcImgRect).(draw.Image)
		}
	} else {
		imgdst = image.NewGray(image.Rect(0, 0, *width, *height))
		if transparent {
			imgdst = image.NewNRGBA(image.Rect(0, 0, *width, *height))
		}
		ImgScaler.Scale(imgdst, imgdst.Bounds(), srcimg, srcImgRect, draw.Over, nil)
	}
	imgBwBorder(imgdst, color.Gray{0}, blackBorderSize, 0, transparent)
	return imgdst
}

func imgDrawRect(imgDst *image.Gray, rect image.Rectangle, thickness int, gray uint8) {
	for x := rect.Min.X; x < rect.Max.X; x++ {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if thickness == 0 || x < (rect.Min.X+thickness) || x > (rect.Max.X-thickness) ||
				y < (rect.Min.Y+thickness) || y > (rect.Max.Y-thickness) {
				imgDst.SetGray(x, y, color.Gray{gray})
			}
		}
	}
}

func pngOptFireAndForget(pngFilePath string) {
	if pngFilePath != "" {
		_ = osExec(false, nil, "pngbattle", pngFilePath)
	}
}
