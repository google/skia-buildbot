// Generate unattend.xml files for multiple devices based on config files.
//
// Setup:
//  - Check out the buildbot repo.
//  - Create two config files, devices.json5 and vars.json5 (examples in this directory).
//  - Create the output directory, if necessary.
//
// Usage (assuming the CWD contains devices.json5 and vars.json5):
//   gen_unattend --resources-dir C:\path\to\buildbot\skolo\win \
//     --out-dir C:\RemoteInstall\WdsClientUnattend
package main

import (
	"flag"
	"os"
	"path/filepath"
	"text/template"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/sklog"
)

var (
	devicesFile  = flag.String("devices", "devices.json5", "JSON5 file with device configuration.")
	varsFile     = flag.String("vars", "vars.json5", "JSON5 file with values for global variables in the templates.")
	resourcesDir = flag.String("resources-dir", "", "The directory containing the templates directory. If blank the current directory will be used.")
	outDir       = flag.String("out-dir", "", "The directory in which to write the generated unattend files. If blank the current directory will be used.")
	assumeYes    = flag.Bool("assume-yes", false, "If true, create or modify files without confirmation.")
)

func main() {
	common.Init()

	devices := DevicesConfig{}
	config.MustParseConfigFile(*devicesFile, "devices", &devices)

	globalVars := GlobalVars{}
	config.MustParseConfigFile(*varsFile, "vars", &globalVars)

	templates := template.Must(template.New("").ParseGlob(filepath.Join(*resourcesDir, "templates/*.xml")))

	if fileInfo, err := os.Stat(*outDir); err != nil {
		sklog.Fatalf("Could not read out-dir %q: %s", *outDir, err)
	} else if !fileInfo.IsDir() {
		sklog.Fatalf("Specified out-dir %q is not a directory.", *outDir)
	}

	if err := genUnattend(devices, globalVars, templates, *outDir, *assumeYes); err != nil {
		sklog.Fatal(err)
	}
}
