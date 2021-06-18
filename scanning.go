package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var (
	scannerDevices  []*ScannerDevice
	saneDefaultArgs = []string{
		"--format=pnm",
		"--buffer-size=" + strconv.FormatInt(128* /*expects in KB*/ 1024, 10),
	}
	saneDevDefaults = map[string][]string{
		"": {},
		"test": {
			"--depth", "8",
			"--mode", "Gray",
			"--resolution", "1200dpi",
			"--source", "Flatbed",
			"--test-picture", "Grid",
			"--enable-test-options", "yes",
		},
	}
	saneDevDontShow = map[string][]string{
		"": {},
		"test": {
			"--source", "--depth", "--mode", "--enable-test-options",
			"--button", "--bool-*", "--int-*", "--int", "--fixed-*", "--fixed", "--string-*", "--string", "*-gamma-*", "-l", "-t", "-x", "-y", "--print-options", "--non-blocking", "--select-fd", "--fuzzy-parameters", "--ppl-loss", "--handscanner", "--three-pass", "--three-pass-*", "--invert-endianess", "--read-*",
		},
	}
)

type ScannerDevice struct {
	Nr      int
	Dev     string
	Vendor  string
	Model   string
	Type    string
	Options []ScanOption
}

type ScanOption struct {
	Category    string
	Name        string
	Description []string
	FormatInfo  string
	IsToggle    bool
	Inactive    bool
}

func (me *ScannerDevice) String() string {
	return fmt.Sprintf("[%d] %s (%s %s, type '%s')", me.Nr, me.Dev, me.Vendor, me.Model, me.Type)
}

func detectScanners() {
	cmd := exec.Command("scanimage", "--formatted-device-list",
		`{"Vendor": "%v", "Model": "%m", "Type": "%t", "Dev": "%d", "Nr": %i}`)
	data, err := cmd.CombinedOutput()
	if err != nil {
		panic(err.Error() + ": " + string(data))
	}
	dataprefix := []byte(`[{"Vendor": "sane-project.org", "Model": "sane-test", "Type": "sim", "Dev": "test", "Nr": -1},`)
	if data = bytes.TrimSpace(data); len(data) == 0 {
		dataprefix = dataprefix[:len(dataprefix)-1]
	}
	jsonLoad("", append(dataprefix, append(data, ']')...), &scannerDevices)

	prefcat, prefdesc, prefspec := "  ", "        ", "    --"
	for _, sd := range scannerDevices {
		cmdargs := append(saneDefaultArgs, "--device-name", sd.Dev, "--all-options")
		if sd.Dev == "test" {
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
					opt.IsToggle = strings.HasPrefix(opt.FormatInfo, "[=(yes|no)]")
					opt.Inactive = strings.HasSuffix(opt.FormatInfo, " [inactive]")
				}
			} else if strings.HasPrefix(ln, prefcat) {
				next()
				cat = strings.TrimSpace(ln)
			}
		}
		next()
	}
	printLn(len(scannerDevices), "scanner(s) detected")
}
