package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
)

var (
	saneDefaultArgs = []string{
		"--format", "pnm",
		"--buffer-size", strconv.FormatInt(128* /*expects in KB*/ 1024, 10),
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
	Nr     int
	Dev    string
	Vendor string
	Model  string
	Type   string
}

func (me *ScannerDevice) String() string {
	return fmt.Sprintf("[%d] %s (%s %s, type '%s')", me.Nr, me.Dev, me.Vendor, me.Model, me.Type)
}

var DetectedScannerDevices = []ScannerDevice{}

func detectScanners() {
	cmd := exec.Command("scanimage", "--formatted-device-list",
		`{"Vendor": "%v", "Model": "%m", "Type": "%t", "Dev": "%d", "Nr": %i}`)
	data, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	dataprefix := []byte(`[{"Vendor": "sane-project.org", "Model": "sane-test", "Type": "sim", "Dev": "test", "Nr": -1},`)
	if data = bytes.TrimSpace(data); len(data) == 0 {
		dataprefix = dataprefix[:len(dataprefix)-1]
	}
	jsonLoad("", append(dataprefix, append(data, ']')...), &DetectedScannerDevices)
	printLn("Scanners: ", DetectedScannerDevices)
}
