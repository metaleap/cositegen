package main

import (
	"bytes"
	"fmt"
	"os/exec"
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
