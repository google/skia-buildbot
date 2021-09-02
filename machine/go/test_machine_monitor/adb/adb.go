// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
)

const (
	commandTimeout = 5 * time.Second
)

// AdbImpl handles talking to the adb process.
type AdbImpl struct{}

// New returns a new Adb.
func New() AdbImpl {
	return AdbImpl{}
}

// Adb is the interface that AdbImpl provides.
type Adb interface {
	// RawProperties returns the unfiltered output of running "adb shell getprop".
	RawProperties(ctx context.Context) (string, error)

	// RawDumpSys returns the unfiltered output of running "adb shell dumpsys <service>".
	RawDumpSys(ctx context.Context, service string) (string, error)

	// Reboot the device.
	Reboot(ctx context.Context) error

	// Uptime returns how long the device has been awake since its last reboot.
	Uptime(ctx context.Context) (time.Duration, error)
}

// RawProperties implements the Adb interface.
func (a AdbImpl) RawProperties(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, "adb", "shell", "getprop")

	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "adb failed with stderr: %q", ee.Stderr)
		}
		return "", err
	}
	return string(b), nil
}

// RawDumpSys implements the Adb interface.
func (a AdbImpl) RawDumpSys(ctx context.Context, service string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, "adb", "shell", "dumpsys", service)

	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "adb failed with stderr: %q", ee.Stderr)
		}
		return "", err
	}
	return string(b), nil
}

// Reboot implements the Adb interface.
func (a AdbImpl) Reboot(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, "adb", "reboot")

	_, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "adb reboot with stderr: %q", ee.Stderr)
		}
		return err
	}
	return nil
}

// Uptime implements the Adb interface.
func (a AdbImpl) Uptime(ctx context.Context) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, "adb", "shell", "cat", "/proc/uptime")

	b, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "adb failed with stderr: %q", ee.Stderr)
		}
		return 0, err
	}
	// The contents of /proc/uptime are the uptime in seconds, followed by the
	// idle time of all the cores.
	// https://en.wikipedia.org/wiki/Uptime#Using_/proc/uptime
	uptimeAsString := string(b)
	parts := strings.Split(uptimeAsString, " ")
	if len(parts) != 2 {
		return 0, skerr.Fmt("Found invalid format for /proc/uptime: %q", uptimeAsString)
	}
	uptime, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return time.Duration(int64(uptime) * int64(time.Second)), nil
}

// Assert that AdbImpl implements the Adb interface.
var _ Adb = AdbImpl{}
