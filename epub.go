package main

// func bla() {
// 	book.genPrep.files = map[string]map[string]string{}
// 	for _, lang := range App.Proj.Langs {
// 		book.genPrep.files[lang] = map[string]string{}

// 		book.genPrep.files[lang]["OEBPS/nav.xhtml"] = `<?xml version="1.0" encoding="UTF-8" ?>
// 			<html xmlns="http://www.w3.org/1999/xhtml"><head></head><body><nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc" id="toc">
// 				<ol>
// 				<li>
// 					<a href="chapter1.xhtml">LeChap1</a>
// 				</li>
// 				</ol>
// 			</nav></body></html>`

// 		book.genPrep.files[lang]["OEBPS/content.opf"] = `<?xml version="1.0" encoding="UTF-8" ?>
// 			<package version="3.0" dir="$DIR" xml:lang="` + lang + `" xmlns="http://www.idpf.org/2007/opf" unique-identifier="bid">
// 				<metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
// 					<dc:identifier id="bid">urn:uuid:$BOOKID</dc:identifier>
// 					<dc:title>` + locStr(book.Title, lang) + `</dc:title>
// 					<dc:language>` + lang + `</dc:language>
// 					<meta property="dcterms:modified">` + time.Now().Format("2006-01-02T15:04:05Z") + `</meta>
// 				</metadata>
// 				<manifest>
// 					<item href="nav.xhtml" id="nav" properties="nav" media-type="application/xhtml+xml"></item>
// 					<item href="chapter1.xhtml" id="chap1" media-type="application/xhtml+xml"></item>
// 				</manifest>
// 				<spine>
// 					<itemref idref="chap1"></itemref>
// 				</spine>
// 			</package>`

// 		book.genPrep.files[lang]["OEBPS/chapter1.xhtml"] = `<?xml version="1.0" encoding="UTF-8" ?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>foobar</title></head><body>Hello World</body></html>`
// 	}
// }

// func (me *Series) genBookBuildEpub(bookId string, outFilePath string, lang string, bgCol bool, dirRtl bool, onDone func()) {
// 	defer onDone()
// 	bookId = strToUuidLike(bookId) // not so uu really
// 	_ = os.Remove(outFilePath)
// 	outfile, err := os.Create(outFilePath)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer outfile.Close()
// 	zw := zip.NewWriter(outfile)

// 	files := []struct {
// 		Path string
// 		Data string
// 	}{
// 		{"mimetype", "application/epub+zip"},
// 		{"META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8" ?>
// 			<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
// 			<rootfiles>
// 				<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
// 			</rootfiles>
// 			</container>`},
// 	}
// 	repl := strings.NewReplacer("$DIR", strIf(dirRtl, "rtl", "ltr"), "$BOOKID", bookId)
// 	for prepFilePath, prepFileData := range me.Book.genPrep.files[lang] {
// 		files = append(files, struct {
// 			Path string
// 			Data string
// 		}{prepFilePath, repl.Replace(prepFileData)})
// 	}
// 	for i, file := range files {
// 		var f io.Writer
// 		var err error
// 		if i == 0 {
// 			f, err = zw.CreateRaw(&zip.FileHeader{Method: zip.Store, NonUTF8: true, Name: file.Path})
// 		} else {
// 			f, err = zw.Create(file.Path)
// 		}
// 		if err != nil {
// 			panic(err)
// 		} else if _, err = f.Write([]byte(file.Data)); err != nil {
// 			panic(err)
// 		}
// 	}
// 	if err := zw.Close(); err != nil {
// 		panic(err)
// 	}
// 	_ = outfile.Sync()
// }
