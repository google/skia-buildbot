// Package adb is a simple wrapper around calling adb.
package adb

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"time"

	"go.skia.org/infra/go/exec"
)

var (
	// proplines is a regex that matches the output of adb shell getprop across
	// multiple lines. The output looks like:
	//
	// [ro.product.manufacturer]: [asus]
	// [ro.product.model]: [Nexus 7]
	// [ro.product.name]: [razor]
	proplines = regexp.MustCompile(`(?m)^\[([a-zA-Z\.]+)\]:\s*\[(.*)\]$`)
)

// Properties returns a map[string]string from running "adb shell getprop".
//
func Properties(ctx context.Context) (map[string]string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ret := map[string]string{}

	cmd := exec.Command{
		Name:    "adb",
		Args:    []string{"shell", "getprop"},
		Stdout:  stdout,
		Stderr:  stderr,
		Timeout: 5 * time.Second,
	}
	if err := exec.Run(ctx, &cmd); err != nil {
		return nil, fmt.Errorf("Failed to run adb shell getprop %q: %s", stderr.String(), err)
	}
	for _, line := range proplines.FindAllStringSubmatch(stdout.String(), -1) {
		ret[line[1]] = line[2]
	}

	return ret, nil
}
