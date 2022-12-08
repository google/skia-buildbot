// Package linux contains Linux-specific pieces of interrogation which are nonetheless testable on
// arbitrary platforms.
package linux

import (
	"bufio"
	"context"
	"io"
	"regexp"
	"strings"

	shell "github.com/kballard/go-shellquote"
	"go.skia.org/infra/go/gpus"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/util_generics"
	"go.skia.org/infra/machine/go/test_machine_monitor/standalone/crossplatform"
)

// VendorAndBrand returns the vendor and brand string of the first encountered CPU in the provided
// contents of /proc/cpuinfo. Extraction may fail and they may be empty--those are valid
// values--even if error is nil.
func VendorAndBrand(cpuInfo io.Reader) (vendor, brandString string, err error) {
	// Fields to be pulled out of /proc/cpuinfo:
	var vendorID, isa, cpuModel, hardware, modelName, processor string
	interestingFields := map[string]*string{
		"vendor_id":  &vendorID,
		"isa":        &isa,
		"cpu model":  &cpuModel,
		"Hardware":   &hardware,
		"model name": &modelName,
		"Processor":  &processor,
	}

	// cpuinfo typically consists of a similar multi-line stanza for each processor core. The cores
	// are usually all the same, to the level of detail we need, so we bail out after the first.
	// This saves some IO, since cpuinfo is about 200K on a 96-core cloudtop.
	scanner := bufio.NewScanner(cpuInfo)
	for scanner.Scan() {
		line := scanner.Text()
		k, v, _ := strings.Cut(line, ": ")
		if line == "" {
			// We've reached the end of the description of the first processor.
			break
		}
		if v == "" {
			// Weird line without a colon. Shouldn't happen, but don't zero out an already-found key
			// "foo" if we happen across a line with just "foo" on it.
			continue
		}
		k = strings.TrimSpace(k)
		field, found := interestingFields[k]
		if found {
			*field = v
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", skerr.Wrapf(err, "failed to iterate over lines of CPU info")
	}

	// Use logic pilfered from Swarming to come up with final answers.
	if vendorID != "" {
		vendor = vendorID
		brandString = modelName
	} else if strings.Contains(isa, "mips") {
		brandString = cpuModel
	} else {
		if hardware != "" {
			brandString = strings.TrimSuffix(hardware, " (Flattened Device Tree)") // on Samsungs
		}
		vendor = util.FirstNonEmpty(modelName, processor, "N/A")
	}
	return vendor, brandString, nil
}

func OSVersions(platform, version string) []string {
	ret := []string{"Linux"}
	if platform != "" {
		platform = strings.ToUpper(platform[0:1]) + platform[1:]
		ret = append(ret, crossplatform.VersionsOfAllPrecisions(platform, version)...)
	}
	return ret
}

var idRegex = regexp.MustCompile(`^(.+?) \[([0-9a-f]{4})\]$`)

// VendorsToVersionGetters is a map of vendor IDs to functions that return GPU driver versions for
// those vendors' products.
type VendorsToVersionGetters = map[string]func(context.Context) string

// GPUs returns a slice of Swarming-style descriptors of all the GPUs on the host, in all
// precisions: "vendorID", "vendorID-deviceID", and, if detectable,
// "vendorID-deviceID-driverVersion". nvidiaVersionGetter is a thunk that returns the version of the
// installed Nvidia driver. intelVersionGetter is similar but for the Intel driver.
func GPUs(ctx context.Context, lspciOutput string, versionGetters VendorsToVersionGetters) ([]string, error) {
	var ret []string
	for _, line := range util.SplitLines(lspciOutput) {
		fields, err := shell.Split(line)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to parse lspci output \"%s\"", line)
		}
		if len(fields) < 4 {
			continue
		}

		groups := idRegex.FindStringSubmatch(fields[1])
		if groups == nil {
			continue
		}
		deviceType := groups[2]
		// Look for display class as noted at http://wiki.osdev.org/PCI.
		if !strings.HasPrefix(deviceType, "03") {
			continue
		}

		groups = idRegex.FindStringSubmatch(fields[2])
		if groups == nil {
			continue
		}
		vendorName := groups[1]
		vendorID := groups[2]

		groups = idRegex.FindStringSubmatch(fields[3])
		if groups == nil {
			continue
		}
		deviceID := groups[2]

		versionGetter := util_generics.Get(versionGetters, vendorID, func(context.Context) string { return "" })
		version := versionGetter(ctx)

		// Prefer vendor name from table.
		vendorName, _ = gpus.IDsToNames(gpus.VendorID(vendorID), vendorName, "dummy", "dummy")

		ret = append(ret, vendorID, vendorID+":"+deviceID)
		if version != "" {
			ret = append(ret, vendorID+":"+deviceID+"-"+version)
		}
	}
	return ret, nil
}
