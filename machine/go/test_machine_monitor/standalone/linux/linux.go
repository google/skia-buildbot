// Package linux contains Linux-specific pieces of interrogation which are nonetheless testable on
// arbitrary platforms.
package linux

import (
	"bufio"
	"io"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
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
