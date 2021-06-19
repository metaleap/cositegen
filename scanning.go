package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var (
	scanJob         *ScanJob
	scanJobFail     string
	scanDevices     []*ScanDevice
	saneDefaultArgs = []string{
		"--format=pnm",
		"--buffer-size=" + strconv.FormatInt(128* /*expects in KB*/ 1024, 10),
	}
	saneDevDefaults = map[string]map[string]string{
		"": {
			"resolution":              "1200dpi",
			"mode":                    "Gray",
			"disable-dynamic-lineart": "yes",
		},
		"test": {
			"depth":               "8",
			"source":              "Flatbed",
			"test-picture":        "Grid",
			"enable-test-options": "yes",
		},
	}
	saneDevDontShow = map[string][]string{
		"": {
			// "mode",
			"lamp-off-time", "clear-calibration", "calibration-file", "expiration-time",
		},
		"test": {
			"source", "depth", // "test-picture","enable-test-options",
			"button", "bool-*", "int-*", "int", "fixed-*", "fixed", "string-*", "string", "*gamma-*", "-l", "-t", "-x", "-y", "print-options", "non-blocking", "select-fd", "fuzzy-parameters", "ppl-loss", "hand-scanner", "three-pass", "three-pass-*", "invert-endianess", "read-*",
		},
	}
)

type ScanDevice struct {
	Nr      int
	Ident   string
	Vendor  string
	Model   string
	Type    string
	Options []ScanOption
}

func (me *ScanDevice) String() string {
	return fmt.Sprintf("[%d] %s (%s %s, type '%s')", me.Nr, me.Ident, me.Vendor, me.Model, me.Type)
}

type ScanOption struct {
	Category    string
	Name        string
	Description []string
	FormatInfo  string
	IsToggle    bool
	Inactive    bool
}

type ScanJob struct {
	Id           string
	Series       *Series
	Chapter      *Chapter
	SheetName    string
	SheetVerName string
	PnmFileName  string
	PngFileName  string
	Dev          *ScanDevice
	Opts         map[string]string
}

func detectScanners() {
	var sds []*ScanDevice
	cmd := exec.Command("scanimage", "--formatted-device-list",
		`{"Vendor": "%v", "Model": "%m", "Type": "%t", "Ident": "%d", "Nr": %i}`)
	data, err := cmd.CombinedOutput()
	if err != nil {
		panic(err.Error() + ": " + string(data))
	}
	dataprefix := []byte(`[{"Vendor": "sane-project.org", "Model": "sane-test", "Type": "sim", "Ident": "test", "Nr": -1},`)
	if data = bytes.TrimSpace(data); len(data) == 0 {
		dataprefix = dataprefix[:len(dataprefix)-1]
	}
	jsonLoad("", append(dataprefix, append(data, ']')...), &sds)

	prefcat, prefdesc, prefspec := "  ", "        ", "    --"
	for _, sd := range sds {
		cmdargs := append(saneDefaultArgs, "--device-name", sd.Ident, "--all-options")
		if sd.Ident == "test" {
			cmdargs = append(cmdargs, "--enable-test-options")
		}
		cmd := exec.Command("scanimage", cmdargs...)
		data, err = cmd.CombinedOutput()
		if err != nil {
			panic(err.Error() + ": " + string(data))
		}
		var cat string
		var opt ScanOption
		next := func() {
			if opt.Name != "" {
				sd.Options = append(sd.Options, opt)
			}
			opt = ScanOption{Category: cat}
		}
		for _, ln := range strings.Split(string(data), "\n") {
			// this exact ordering of the `if` tests matters here
			if strings.HasPrefix(ln, prefdesc) {
				opt.Description = append(opt.Description, strings.TrimSpace(ln))
			} else if strings.HasPrefix(ln, prefspec) {
				next()
				ln = strings.TrimSpace(ln[len(prefspec):])
				idx := strings.IndexFunc(ln, func(r rune) bool {
					return !(r == '-' || (r >= 'a' && r <= 'z'))
				})
				opt.Name = ln
				if idx > 0 {
					opt.Name = ln[:idx]
					opt.FormatInfo = strings.TrimSpace(ln[idx:])
					opt.Inactive = strings.HasSuffix(opt.FormatInfo, " [inactive]")
					opt.IsToggle = strings.HasPrefix(opt.FormatInfo, "[=(") && strings.Contains(opt.FormatInfo, "yes|no)]")
				} else {
					opt.IsToggle = true
				}
			} else if strings.HasPrefix(ln, prefcat) {
				next()
				cat = strings.TrimSpace(ln)
			}
		}
		next()
	}
	scanDevices = sds
	printLn(len(scanDevices), "scanner(s) detected")
}

func scanJobDo() {
	sj := scanJob
	defer func() {
		scanJob = nil
		if err := recover(); err != nil {
			scanJobFail = "[" + sj.SheetName + "_" + sj.SheetVerName + "] " + fmt.Sprintf("%v", err)
		}
	}()

	cmd := exec.Command("scanimage", append(saneDefaultArgs,
		"--device-name="+sj.Dev.Ident,
		"--output-file="+sj.PnmFileName,
	)...)
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	if err := cmd.Wait(); err != nil {
		panic(err)
	}

	_, err := os.Stat(sj.PnmFileName)
	if err != nil {
		panic(err)
	}
}
