package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (me *BookBuild) genBookBuildEpub(outFilePath string, srcFilePaths []string, lang string, dirRtl bool, res int, onDone func()) {
	defer onDone()

	xmlfiles, bookuid := map[string]string{}, strToUuidLike(me.id(lang, dirRtl, res)) // not so uu really

	xmlnav := `<?xml version="1.0" encoding="UTF-8" ?>
				<html xmlns="http://www.w3.org/1999/xhtml"><head></head><body>
					<nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc" id="toc"><ol>`
	for _, chap := range me.series.Chapters {
		pgnr := me.genPrep.pgNrs[lang][chap]
		xmlnav += `<li><a href="` + itoa0(pgnr, len(itoa(len(srcFilePaths)))) + `.xhtml">` +
			locStr(chap.Title, lang) + `</a></li>`
	}
	xmlnav += `</ol></nav></body></html>`
	xmlfiles["OEBPS/nav.xhtml"] = xmlnav

	xmlopf := `<?xml version="1.0" encoding="UTF-8" ?>
				<package version="3.0" xmlns="http://www.idpf.org/2007/opf"
					dir="` + strIf(dirRtl, "rtl", "ltr") + `" xml:lang="` + lang + `" unique-identifier="bid"
					prefix="rendition:http://www.idpf.org/vocab/rendition/#">
					<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
						<dc:identifier id="bid">urn:uuid:` + bookuid + `</dc:identifier>
						<dc:title>` + locStr(me.book.Title, lang) + `</dc:title>
						<dc:language>` + lang + `</dc:language>
						<meta property="dcterms:modified">` + time.Now().Format("2006-01-02T15:04:05Z") + `</meta>
						<meta property="rendition:orientation">landscape</meta>
						<meta property="rendition:spread">landscape</meta>
					</metadata>
					<manifest>
						<item href="nav.xhtml" id="nav" properties="nav" media-type="application/xhtml+xml"/>`
	for i, srcfilepath := range srcFilePaths {
		xmlopf += `<item id="p` + itoa(i) + `" media-type="image/png" href="` +
			itoa0(i+1, len(itoa(len(srcFilePaths)))) + filepath.Ext(srcfilepath) + `"/>`
		xmlopf += `<item id="h` + itoa(i) + `" media-type="application/xhtml+xml" href="` +
			itoa0(i+1, len(itoa(len(srcFilePaths)))) + ".xhtml" + `"/>`
	}
	xmlopf += `</manifest><spine>`
	for i := range srcFilePaths {
		xmlopf += `<itemref idref="h` + itoa(i) + `"/>`
	}
	xmlopf += `</spine></package>`
	xmlfiles["OEBPS/content.opf"] = xmlopf

	xmlinf := `<?xml version="1.0" encoding="UTF-8" ?>
				<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles>
					<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
				</rootfiles></container>`
	type zf struct {
		Path string
		Data string
	}
	files := []zf{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", xmlinf},
	}
	for prepFilePath, prepFileData := range xmlfiles {
		files = append(files, zf{prepFilePath, prepFileData})
	}

	outfile, err := os.Create(outFilePath)
	if err != nil {
		panic(err)
	}
	defer outfile.Close()
	zw := zip.NewWriter(outfile)

	for i, file := range files {
		var f io.Writer
		var err error
		if i == 0 {
			f, err = zw.CreateRaw(&zip.FileHeader{Method: zip.Store, NonUTF8: true, Name: file.Path})
		} else {
			f, err = zw.Create(file.Path)
		}
		if err != nil {
			panic(err)
		} else if _, err = f.Write([]byte(file.Data)); err != nil {
			panic(err)
		}
	}
	for i, srcfilepath := range srcFilePaths {
		name := itoa0(i+1, len(itoa(len(srcFilePaths))))
		xmlpage := `<?xml version="1.0" encoding="UTF-8" ?>
					<html xmlns="http://www.w3.org/1999/xhtml"><head>
						<title>` + name + `</title>
						<style type="text/css">
						body, img {
							margin: 0px; padding: 0px;
							margin-top: 0px;
							margin-bottom: 0px;
							margin-left: 0px;
							margin-right: 0px;
							padding-top: 0px;
							padding-bottom: 0px;
							padding-left: 0px;
							padding-right: 0px;
						}
						img { position: absolute; left: 0px; top: 0px; width: 100%; }
						@page { size: landscape; padding: 0; margin: 0; }
						</style>
					</head><body>
						<img width="100%" src="` + name + `.png"/>
					</body></html>`
		if fw, err := zw.Create("OEBPS/" + name + filepath.Ext(srcfilepath)); err != nil {
			panic(err)
		} else {
			io.Copy(fw, bytes.NewReader(fileRead(srcfilepath)))
		}
		if fw, err := zw.Create("OEBPS/" + name + ".xhtml"); err != nil {
			panic(err)
		} else {
			io.Copy(fw, strings.NewReader(xmlpage))
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	_ = outfile.Sync()
}
