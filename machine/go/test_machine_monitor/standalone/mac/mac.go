// Package mac contains Mac-specific pieces of interrogation which are nonetheless testable on
// arbitrary platforms.
package mac

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/gputable"
	"howett.net/plist"
)

type GPU struct {
	// Any string field missing from the plist ends up "".
	ID       string `plist:"spdisplays_device-id"`
	VendorID string `plist:"spdisplays_vendor-id"`
	Vendor   string `plist:"spdisplays_vendor"`
	Version  string `plist:"spdisplays_gmux-version"`
	Model    string `plist:"sppci_model"`
}

// GPUsFromSystemProfilerXML returns the GPUs found on a Mac by System Profiler, as expressed
// through its -xml output option. If the output doesn't look valid, returns an error and no GPUs.
// If there are no GPUs, will return 0 GPUs even if no error occurs.
func GPUsFromSystemProfilerXML(xml string) ([]GPU, error) {
	type OuterDict struct {
		GPUs []GPU `plist:"_items"`
	}
	var profilerOutput []OuterDict

	_, err := plist.Unmarshal([]byte(xml), &profilerOutput)

	if err != nil {
		return nil, skerr.Wrapf(err, "couldn't unmarshal XML from System Profiler")
	}
	if len(profilerOutput) < 1 {
		return nil, skerr.Fmt("System Profiler returned no info")
	}
	return profilerOutput[0].GPUs, nil
}

// DimensionsFromGPUs turns a slice of Mac GPUs into Swarming-style dimensions, e.g. ["8086",
// "8086:3e9b"]. If there are no GPUs, return ["none"].
func DimensionsFromGPUs(gpus []GPU) []string {
	var dimensions []string
	for _, gpu := range gpus {
		if gpu.ID == "" {
			continue
		}
		gpuID := strings.TrimPrefix(gpu.ID, "0x")
		var vendorID gputable.VendorID
		if gpu.VendorID != "" {
			// NVidia
			vendorID = gputable.VendorID(strings.TrimPrefix(gpu.VendorID, "0x"))
		} else if gpu.Vendor != "" {
			// Intel and ATI
			re := regexp.MustCompile(`\(0x([0-9a-f]{4})\)`)
			if matches := re.FindStringSubmatch(gpu.Vendor); matches != nil {
				vendorID = gputable.VendorID(matches[1])
			}
		}

		vendorName := ""
		if gpu.Model != "" {
			ok := false
			// The first word is pretty much always the company name:
			vendorName, _, ok = strings.Cut(gpu.Model, " ")
			if !ok {
				// This has apparently never happened in the history of Swarming's introspection
				// code, because it would have raised an uncaught exception there:
				sklog.Warningf("GPU device name was only a single word: %s", gpu.Model)
				continue
			}
		}

		// macOS 10.13 stopped including the vendor ID in the spdisplays_vendor string. Infer it
		// from the vendor name instead.
		if vendorID == "" {
			vendorID = gputable.VendorNameToID(vendorName)
		}
		if vendorID == "" && gpu.Vendor != "" {
			re := regexp.MustCompile(`sppci_vendor_([a-z]+)$`)
			if matches := re.FindStringSubmatch(gpu.Vendor); matches != nil {
				vendorName = matches[1]
				vendorID = gputable.VendorNameToID(vendorName)
			}
		}
		if vendorID == "" {
			vendorID = gputable.VendorID("UNKNOWN")
		} else if vendorID == gputable.VMWare {
			// We consider VMWare as not having any GPUs.
			return []string{"none"}
		}

		dimensions = append(dimensions, string(vendorID), fmt.Sprintf("%s:%s", vendorID, gpuID))

		// Add GPU version:
		if gpu.Version != "" {
			re := regexp.MustCompile(`([0-9.]+) \[([0-9.]+)\]`)
			if matches := re.FindStringSubmatch(gpu.Version); matches != nil {
				dimensions = append(dimensions, fmt.Sprintf("%s:%s-%s-%s", vendorID, gpuID, matches[1], matches[2]))
			}
		}
	}
	if len(dimensions) == 0 {
		dimensions = []string{"none"}
	}
	sort.Strings(dimensions)
	return dimensions
}
