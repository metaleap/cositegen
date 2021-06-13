package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)

type Any = interface{}
type A = map[string]string

var itoa = strconv.Itoa

type Indexed interface {
	At(int) fmt.Stringer
	Len() int
}

func mkDir(dirPath string) {
	if err := os.Mkdir(dirPath, os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
}

func percent(hundred int, val int) int {
	if hundred == 0 || val == 0 {
		return 0
	}
	return 100 / (hundred / val)
}

func writeFile(fileName string, data []byte) {
	tmpfilename := fileName + "." + strconv.FormatInt(time.Now().UnixNano(), 36)
	if err := ioutil.WriteFile(tmpfilename, data, os.ModePerm); err != nil {
		panic(err)
	}
	if err := os.Rename(tmpfilename, fileName); err != nil {
		panic(err)
	}
}

var stdio sync.Mutex

func assert(b bool) {
	if !b {
		panic("ASSERT")
	}
}

func printLn(args ...Any) {
	stdio.Lock()
	fmt.Println(args...)
	stdio.Unlock()
}

func contentHash(content []byte) []byte {
	b1, b2 := sha1.Sum(content), md5.Sum(content)
	return append(b1[:], b2[:]...)
}

func jsonLoad(filename string, intoPtr Any) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, intoPtr); err != nil {
		panic(err)
	}
}

func jsonSave(fileName string, obj Any) {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		panic(err)
	}
	writeFile(fileName, data)
}
