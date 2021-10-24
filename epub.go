package main

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
