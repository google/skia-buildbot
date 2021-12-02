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
	"go.skia.org/infra/go/sklog"
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
	// EnsureOnline returns nil if the Android device is online and ready to
	// run, otherwise it will try to bring it back online and if that fails will
	// return an error.
	EnsureOnline(ctx context.Context) error

	// RawProperties returns the unfiltered output of running "adb shell getprop".
	RawProperties(ctx context.Context) (string, error)

	// RawDumpSys returns the unfiltered output of running "adb shell dumpsys <service>".
	RawDumpSys(ctx context.Context, service string) (string, error)

	// Reboot the device.
	Reboot(ctx context.Context) error

	// Uptime returns how long the device has been awake since its last reboot.
	Uptime(ctx context.Context) (time.Duration, error)
}

// adbCommand sends a single adb command to the device then captures and returns
// both the stdout and stderr of the results.
func (a AdbImpl) adbCommand(ctx context.Context, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, "adb", args...)

	b, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
			err = skerr.Wrapf(err, "adb %s: failed with stderr: %q", strings.Join(args, " "), ee.Stderr)
		}
		return "", stderr, skerr.Wrap(err)
	}
	return string(b), "", nil
}

// getState returns an error if the device is in an error state or it can't be
// queried.
//
// The returned string will contain the text of error: message, excluding the
// 'error: ' part if the error was from an exit code, otherwise the empty string
// is returned.
func (a AdbImpl) getState(ctx context.Context) (string, error) {
	_, stderr, err := a.adbCommand(ctx, "get-state")
	if err != nil {
		return strings.TrimPrefix(stderr, "error:"), skerr.Wrap(err)
	}
	return "", nil
}

// EnsureOnline implements the Adb interface.
func (a AdbImpl) EnsureOnline(ctx context.Context) error {
	// Run `adb get-state` it will return a non-zero exit code and emit a string of the form:
	//
	//     error: device offline
	//     error: device unauthorized
	//     error: no devices/emulators found
	//
	// If the device is connected and ready to go it will return a 0 exit code and print:
	//
	//     device
	//
	// If offline we should run:
	//
	//     adb reconnect offline
	//
	// Which should reconnect the device.
	state, err := a.getState(ctx)
	if err == nil {
		return nil
	}
	if !strings.Contains(state, "offline") {
		return skerr.Wrapf(err, "adb returned an error state we can't do anything about: %q", state)
	}
	// TODO(jcgregorio) If this works we should generalize this behavior for other commands.
	_, _, err = a.adbCommand(ctx, "reconnect", "offline")
	if err != nil {
		return skerr.Wrap(err)
	}

	_, err = a.getState(ctx)

	return skerr.Wrap(err)
}

// RawProperties implements the Adb interface.
func (a AdbImpl) RawProperties(ctx context.Context) (string, error) {
	stdout, _, err := a.adbCommand(ctx, "shell", "getprop")
	if err != nil {
		return "", err
	}
	return stdout, nil
}

// RawDumpSys implements the Adb interface.
func (a AdbImpl) RawDumpSys(ctx context.Context, service string) (string, error) {
	stdout, _, err := a.adbCommand(ctx, "shell", "dumpsys", service)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

// Reboot implements the Adb interface.
func (a AdbImpl) Reboot(ctx context.Context) error {
	_, _, err := a.adbCommand(ctx, "reboot")
	if err == nil {
		return nil
	}
	// Try reconnecting.
	_, _, err = a.adbCommand(ctx, "reconnect", "offline")
	if err != nil {
		sklog.Errorf("Reboot: Failed to reconnect: %s", err)
	}
	_, _, err = a.adbCommand(ctx, "reboot")

	return err
}

// Uptime implements the Adb interface.
func (a AdbImpl) Uptime(ctx context.Context) (time.Duration, error) {
	stdout, _, err := a.adbCommand(ctx, "shell", "cat", "/proc/uptime")
	if err != nil {
		return 0, err
	}

	// The contents of /proc/uptime are the uptime in seconds, followed by the
	// idle time of all the cores.
	// https://en.wikipedia.org/wiki/Uptime#Using_/proc/uptime
	uptimeAsString := stdout
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
