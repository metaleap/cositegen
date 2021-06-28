package main

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var (
	scanJob         *ScanJob
	scanJobNotice   string
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
			"depth":        "8",
			"source":       "Flatbed",
			"test-picture": "Grid",
		},
	}
	saneDevDontShow = map[string][]string{
		"": {
			// "mode",
			"lamp-off-time", "clear-calibration", "calibration-file", "expiration-time",
		},
		"test": {
			"source", "depth", "enable-test-options", // "test-picture",
			"button", "bool-*", "int-*", "int", "fixed-*", "fixed", "string-*", "string", "*gamma-*", "l", "t", "x", "y", "print-options", "non-blocking", "select-fd", "fuzzy-parameters", "ppl-loss", "hand-scanner", "three-pass", "three-pass-*", "invert-endianess", "read-*",
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

func scanDevicesDetection() {
	timedLogged("", func() string {
		scanDevices = nil
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

		prefcat, prefdesc, prefspec := "  ", "        ", "    -"
		for _, sd := range sds {
			if sd.Ident = trim(sd.Ident); sd.Ident == "" || html.EscapeString(sd.Ident) != sd.Ident {
				panic(fmt.Sprintf("TODO prep code for previously unexpected scandev ident format:\t%#v", sd.Ident))
			}
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
					opt.Description = append(opt.Description, trim(ln))
				} else if strings.HasPrefix(ln, prefspec) {
					next()
					ln = trim(ln[len(prefspec):])
					idx := strings.IndexFunc(ln, func(r rune) bool {
						return !(r == '-' || (r >= 'a' && r <= 'z'))
					})
					opt.Name = strings.TrimLeft(ln, "-")
					if idx > 0 {
						opt.Name = strings.TrimLeft(ln[:idx], "-")
						opt.FormatInfo = trim(ln[idx:])
						opt.Inactive = strings.HasSuffix(opt.FormatInfo, " [inactive]")
						opt.IsToggle = strings.HasPrefix(opt.FormatInfo, "[=(") && strings.Contains(opt.FormatInfo, "yes|no)]")
					} else {
						opt.IsToggle = true
					}
				} else if strings.HasPrefix(ln, prefcat) {
					next()
					cat = trim(ln)
				}
			}
			next()
		}
		scanDevices = sds
		return itoa(len(scanDevices)) + " scanner/s detected in"
	})
}

func scanJobDo() {
	sj := scanJob
	defer func() {
		scanJob = nil
		if err := recover(); err != nil {
			_ = os.Remove(sj.PngFileName)
			scanJobNotice = "[" + sj.SheetName + "_" + sj.SheetVerName + "] " + fmt.Sprintf("%v", err)
		}
	}()
	for _, fname := range []string{sj.PngFileName, sj.PnmFileName} {
		_ = os.Remove(fname)
	}

	timedLogged("SheetScan: from "+sj.Dev.Ident+"...", func() string {
		cmd := exec.Command("scanimage", append(saneDefaultArgs,
			"--device-name="+sj.Dev.Ident,
			"--output-file="+sj.PnmFileName,
		)...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		for name, val := range sj.Opts {
			cmd.Args = append(cmd.Args, "--"+name+"="+val)
		}
		printLn("\n\n\nSCANNING via command:\n" + strings.Join(cmd.Args, " ") + "\n\n")
		if err := cmd.Start(); err != nil {
			panic(fmt.Errorf("%v %v", err, cmd.Args))
		}
		if err := cmd.Wait(); err != nil {
			panic(fmt.Errorf("%v %v", err, cmd.Args))
		}
		return "for " + sj.PnmFileName
	})
	timedLogged("SheetScan: convert to PNG...", func() string {
		pnmfile, err := os.Open(sj.PnmFileName)
		if err != nil {
			panic(sj.PnmFileName + ": " + err.Error())
		}
		pngfile, err := os.Create(sj.PngFileName)
		if err != nil {
			panic(sj.PngFileName + ": " + err.Error())
		}
		imgPnmToPng(pnmfile, pngfile, true)
		_ = os.Remove(sj.PnmFileName)
		return "for " + sj.PngFileName
	})
	scanJobNotice = "successfully written to " + sj.PngFileName + ", available in editor upon restart"
	printLn(scanJobNotice)
	cmd := exec.Command(browserCmd[0], append(browserCmd[1:], "--app=file://"+os.Getenv("PWD")+"/"+sj.PngFileName)...)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
}
