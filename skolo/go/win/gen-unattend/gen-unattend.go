package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"go.skia.org/infra/go/sklog"
)

const (
	FILE_MODE = 0660
)

// DevicesConfig contains listings of devices and how to generate their unattend files.
type DevicesConfig struct {
	// Devices maps device name to device config.
	Devices map[string]DeviceConfig
}

// DeviceConfig indicates which template to use for the unattend file.
type DeviceConfig struct {
	Unattend string
}

// GlobalVars contains device-indepenent template parameters, such as passwords.
type GlobalVars struct {
	// AdminPassword is the (plain text) password of Administrator.
	AdminPassword string
	// ChromeBotPassword is the (plain text) password of chrome-bot.
	ChromeBotPassword string
}

// TemplateVars is passed to the template as the top-level data.
type TemplateVars struct {
	GlobalVars
	DeviceName string
	DeviceConfig
}

// runTemplates generates the expected contents for all expected unattend files. Returns a map from
// base filename to contents.
func runTemplates(devices DevicesConfig, globalVars GlobalVars, templates *template.Template) (map[string][]byte, error) {
	rv := make(map[string][]byte, len(devices.Devices))
	for deviceName, deviceConfig := range devices.Devices {
		t := templates.Lookup(deviceConfig.Unattend)
		if t == nil {
			return nil, fmt.Errorf("For device %q: no such unattend template %q.", deviceName, deviceConfig.Unattend)
		}
		buf := bytes.Buffer{}
		err := t.Execute(&buf, TemplateVars{
			GlobalVars:   globalVars,
			DeviceName:   deviceName,
			DeviceConfig: deviceConfig,
		})
		if err != nil {
			return nil, fmt.Errorf("For device %q: error executing template: %s", deviceName, err)
		}
		filename := fmt.Sprintf("unattend-%s.xml", deviceName)
		rv[filename] = buf.Bytes()
	}
	return rv, nil
}

// diffs represents the differences between the expected outdir and the actual outdir.
type diffs struct {
	// Lists of base filenames.
	toCreate []string
	toModify []string
	toDelete []string
}

// computeDiffs compares expectedContents with the actual contents of outDir.
func computeDiffs(expectedContents map[string][]byte, outDir string) (diffs, error) {
	rv := diffs{}
	actualFiles, err := ioutil.ReadDir(outDir)
	if err != nil {
		return rv, err
	}
	missing := make(map[string]bool, len(expectedContents))
	for expected := range expectedContents {
		missing[expected] = true
	}
	for _, fileInfo := range actualFiles {
		if !fileInfo.Mode().IsRegular() {
			continue
		}
		filename := fileInfo.Name()
		expected, ok := expectedContents[filename]
		if !ok {
			rv.toDelete = append(rv.toDelete, filename)
			continue
		}
		delete(missing, filename)
		filePath := filepath.Join(outDir, filename)
		actual, err := ioutil.ReadFile(filePath)
		if err != nil {
			sklog.Warningf("Unable to read existing file %q; assuming modified.", filePath)
			rv.toModify = append(rv.toModify, filename)
			continue
		}
		if bytes.Compare(bytes.TrimSpace(expected), bytes.TrimSpace(actual)) != 0 {
			rv.toModify = append(rv.toModify, filename)
		}
	}
	if len(missing) > 0 {
		rv.toCreate = make([]string, 0, len(missing))
		for filename := range missing {
			rv.toCreate = append(rv.toCreate, filename)
		}
		sort.Strings(rv.toCreate)
	}
	return rv, nil
}

// confirmDiffsImpl writes the expected changes to stdout and optionally prompts user to continue.
// Returns an error if user aborted.
func confirmDiffsImpl(d diffs, outDir string, assumeYes bool, stdin io.Reader, stdout io.Writer) error {
	if len(d.toCreate) == 0 && len(d.toModify) == 0 && len(d.toDelete) == 0 {
		fmt.Fprintln(stdout, "No changes.")
		return nil
	}
	printIfAny := func(verb string, files []string) {
		if len(files) > 0 {
			fmt.Fprintf(stdout, "%s %d file(s):\n", verb, len(files))
			for _, f := range files {
				fmt.Fprintf(stdout, "\t%s\n", filepath.Join(outDir, f))
			}
			fmt.Fprintln(stdout)
		}
	}
	printIfAny("Create", d.toCreate)
	printIfAny("Modify", d.toModify)
	printIfAny("Delete", d.toDelete)

	if !assumeYes {
		fmt.Fprint(stdout, "Continue? (y/N) ")
		var response string
		_, err := fmt.Fscanf(stdin, "%s\n", &response)
		if err != nil || len(response) == 0 || strings.ToLower(response[0:1]) != "y" {
			return fmt.Errorf("Aborted.")
		}
	}
	return nil
}

// mustConfirmDiffs writes the expected changes to Stdout and optionally prompts user to continue.
// Exits the program if user aborted.
func mustConfirmDiffs(d diffs, outDir string, assumeYes bool) {
	if err := confirmDiffsImpl(d, outDir, assumeYes, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1) // Avoid backtrace from sklog.Fatalf.
	}
}

// writeFiles performs the actions given in d on outDir, taking file contents from expectedContents.
func writeFiles(expectedContents map[string][]byte, d diffs, outDir string) error {
	for _, filename := range d.toCreate {
		if err := ioutil.WriteFile(filepath.Join(outDir, filename), expectedContents[filename], FILE_MODE); err != nil {
			return err
		}
	}
	for _, filename := range d.toModify {
		if err := ioutil.WriteFile(filepath.Join(outDir, filename), expectedContents[filename], FILE_MODE); err != nil {
			return err
		}
	}
	for _, filename := range d.toDelete {
		if err := os.Remove(filepath.Join(outDir, filename)); err != nil {
			return err
		}
	}
	return nil
}

// genUnattend creates or modifies unattend files for devices in outDir.
func genUnattend(devices DevicesConfig, globalVars GlobalVars, templates *template.Template, outDir string, assumeYes bool) error {
	expectedContents, err := runTemplates(devices, globalVars, templates)
	if err != nil {
		return err
	}
	d, err := computeDiffs(expectedContents, outDir)
	if err != nil {
		return err
	}
	mustConfirmDiffs(d, outDir, assumeYes)
	return writeFiles(expectedContents, d, outDir)
}
