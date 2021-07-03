package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Any = interface{}
type A = map[string]string

var itoa = strconv.Itoa
var trim = strings.TrimSpace
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

func ftoa(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

func strSize(size int) string {
	return strSize64(int64(size))
}

func strSize64(size int64) string {
	if mb := int64(1024 * 1024); size >= mb {
		return ftoa(float64(size)*(1.0/float64(mb)), 1) + "MB"
	} else if kb := int64(1024); size >= kb {
		return itoa(1+int(float64(size)*(1.0/float64(kb)))) + "KB"
	}
	return fmt.Sprintf("%vB", size)
}

func mkDir(dirPath string) {
	if err := os.Mkdir(dirPath, os.ModePerm); err != nil && !os.IsExist(err) {
		panic(err)
	}
}

func rmDir(dirPath string) {
	if err := os.RemoveAll(dirPath); err != nil && !os.IsNotExist(err) {
		panic(err)
	}
}

func percent(hundred int, val int) int {
	if hundred == 0 || val == 0 {
		return 0
	}
	return 100 / (hundred / val)
}

func copyFile(srcPath string, dstPath string) {
	writeFile(dstPath, readFile(srcPath))
}

func readFile(fileName string) []byte {
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	return data
}

func writeFile(fileName string, data []byte) {
	tmpfilename := fileName + "." + strconv.FormatInt(time.Now().UnixNano(), 36)
	if err := os.WriteFile(tmpfilename, data, os.ModePerm); err != nil {
		_ = os.Remove(tmpfilename)
		panic(err)
	} else if err := os.Rename(tmpfilename, fileName); err != nil {
		_ = os.Remove(tmpfilename)
		panic(err)
	}
}

func contentHash(content []byte) []byte {
	assert(len(content) != 0)
	b1, b2 := sha1.Sum(content), md5.Sum(content)
	return append(b2[:], b1[:]...)
}

func contentHashStr(content []byte) (s string) {
	hash, sbuf := contentHash(content), strings.Builder{}
	for (len(hash) % 8) != 0 {
		hash = append(hash, 0)
	}
	for i := 0; i < len(hash); i += 8 {
		_, _ = sbuf.WriteString(strconv.FormatUint(binary.LittleEndian.Uint64(hash[i:i+8]), 36))
	}
	return sbuf.String()
}

func jsonLoad(eitherFileName string, orBytes []byte, intoPtr Any) {
	data := orBytes
	if eitherFileName != "" {
		data = readFile(eitherFileName)
	}
	if err := json.Unmarshal(data, intoPtr); err != nil {
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

func locStr(m map[string]string, langId string) (s string) {
	if s = m[langId]; s == "" {
		s = m[App.Proj.Langs[0]]
	}
	return s
}

func intAbs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func intLim(i int, min int, max int) int {
	if i > max {
		return max
	} else if i < min {
		return min
	}
	return i
}

func atoi(s string, min int, max int) int {
	v, _ := strconv.Atoi(s)
	return intLim(v, min, max)
}

func timedLogged(logMsg string, do func() string) {
	if logMsg != "" {
		printLn(logMsg)
	}
	tstart := time.Now()
	if summary := do(); logMsg == "" {
		printLn(summary, time.Now().Sub(tstart).String())
	} else {
		printLn("\t...took", time.Now().Sub(tstart).String(), summary)
	}
}
