package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Any = interface{}
type A = map[string]string

type FilePathsSortingByFileSize []string

func (me FilePathsSortingByFileSize) Len() int          { return len(me) }
func (me FilePathsSortingByFileSize) Swap(i int, j int) { me[i], me[j] = me[j], me[i] }
func (me FilePathsSortingByFileSize) Less(i int, j int) bool {
	fi1, fi2 := fileStat(me[i]), fileStat(me[j])
	return fi1 == nil || (fi2 != nil && fi1.Size() < fi2.Size())
}

type FilePathsSortingByModTime []string

func (me FilePathsSortingByModTime) Len() int          { return len(me) }
func (me FilePathsSortingByModTime) Swap(i int, j int) { me[i], me[j] = me[j], me[i] }
func (me FilePathsSortingByModTime) Less(i int, j int) bool {
	fi1, fi2 := fileStat(me[i]), fileStat(me[j])
	return fi1 == nil || (fi2 != nil && fi1.ModTime().UnixNano() < fi2.ModTime().UnixNano())
}

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

func strIf(b bool, sThen string, sElse string) string {
	if b {
		return sThen
	}
	return sElse
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

func fileLinkOrCopy(srcPath string, dstPath string) {
	if os.Getenv("NOLINKS") != "" {
		fileWrite(dstPath, fileRead(srcPath))
	} else if srcpathabs, err := filepath.Abs(srcPath); err != nil {
		panic(err)
	} else if err := os.Symlink(srcpathabs, dstPath); err != nil {
		panic(err)
	}
}

func fileRead(fileName string) []byte {
	data, err := os.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	return data
}

func fileIsSymlink(fileInfo os.FileInfo) bool {
	if fileInfo != nil {
		return (fileInfo.Mode() & os.ModeSymlink) != 0
	}
	return false
}

func fileWrite(fileName string, data []byte) {
	tmpfilename := fileName + "." + strconv.FormatInt(time.Now().UnixNano(), 36)
	if err := os.WriteFile(tmpfilename, data, os.ModePerm); err != nil {
		_ = os.Remove(tmpfilename)
		panic(err)
	} else if err := os.Rename(tmpfilename, fileName); err != nil {
		_ = os.Remove(tmpfilename)
		panic(err)
	}
}

func stat(fileName string, isDir bool) os.FileInfo {
	fileinfo, err := os.Stat(fileName)
	if err != nil && !os.IsNotExist(err) {
		panic(err)
	}
	if fileinfo != nil && isDir == fileinfo.IsDir() {
		return fileinfo
	}
	return nil
}

func dirStat(fileName string) os.FileInfo {
	return stat(fileName, true)
}

func fileStat(fileName string) os.FileInfo {
	return stat(fileName, false)
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
		data = fileRead(eitherFileName)
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
	fileWrite(fileName, data)
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

func intMin(is []int) (r int) {
	r = is[0]
	for _, i := range is[1:] {
		if i < r {
			r = i
		}
	}
	return
}

func intMax(is []int) (r int) {
	r = is[0]
	for _, i := range is[1:] {
		if i > r {
			r = i
		}
	}
	return
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

func xmlOuters(s string, start string, end string) (ret []string) {
	for more := true; more; {
		idx0, idx1, idx2 := strings.LastIndex(s, start[:len(start)-1]+" "), strings.LastIndex(s, start), strings.LastIndex(s, end)
		if idx1 < 0 || idx0 > idx1 {
			idx1 = idx0
		}
		if more = (idx2 > idx1 && idx1 > 0); more {
			ret = append([]string{s[:idx2+len(end)][idx1:]}, ret...)
			s = s[:idx1]
		}
	}
	return
}

func xmlAttrs(s string, names ...string) (ret []string) {
	for _, name := range names {
		ret = append(ret, xmlAttr(s, name))
	}
	return
}

func xmlAttr(s string, name string) string {
	needle := ` ` + name + `="`
	if idx := strings.Index(s, needle); idx > 0 {
		s = s[idx+len(needle):]
		if idx = strings.Index(s, `"`); idx > 0 {
			return s[:idx]
		}
	}
	return ""
}

var xmlUnescRepl = strings.NewReplacer(
	"&#34;", "\"",
	"&#39;", "'",
	"&lt;", "<",
	"&gt;", ">",
	"&#x9;", "\t",
	"&#xA;", "\n",
	"&#xD;", "\r",
)

func xmlUnesc(s string) string {
	for k, v := range hEscs {
		s = strings.Replace(s, v, string(k), -1)
	}
	return strings.Replace(xmlUnescRepl.Replace(s), "&amp;", "&", -1)
}
