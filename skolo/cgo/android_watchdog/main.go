// Copyright 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
// +build android

// Watchdog daemon for android devices. It will attempt to reboot the device
// if it is disconnected from USB for too long.
package main

/*
#cgo LDFLAGS: -landroid -llog
#include <android/log.h>
#include <string.h>
*/
import "C"
import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"github.com/VividCortex/godaemon"
	"github.com/luci/luci-go/common/runtime/paniccatcher"
)

var (
	logHeader  = C.CString("Skia_Revive_Device")
	errTimeout = errors.New("timeout")
)

const (
	stdInFd  = 0
	stdOutFd = 1
	stdErrFd = 2
)

type logLevel int

const (
	logInfo = iota
	logWarning
	logError
)

func (l logLevel) getLogLevel() C.int {
	switch l {
	case logInfo:
		return C.ANDROID_LOG_INFO
	case logWarning:
		return C.ANDROID_LOG_WARN
	case logError:
		return C.ANDROID_LOG_ERROR
	default:
		panic("Unknown log level.")
	}
}

func logcatLog(level logLevel, format string, args ...interface{}) {
	cmsg := C.CString(fmt.Sprintf(format, args...))
	defer C.free(unsafe.Pointer(cmsg))
	C.__android_log_write(level.getLogLevel(), logHeader, cmsg)
}

// Reboot device by writing to sysrq-trigger. See:
// https://www.kernel.org/doc/Documentation/sysrq.txt
func rebootDevice() error {
	fd, err := os.OpenFile("/proc/sysrq-trigger", os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("Can't open /proc/sysrq-trigger: %s", err.Error())
	}
	defer fd.Close()
	_, err = fd.Write([]byte("b"))
	if err != nil {
		return fmt.Errorf("Can't reboot: %s", err.Error())
	}
	return fmt.Errorf("I just rebooted. How am I still alive?!?\n")
}

func realMain() int {
	godaemon.MakeDaemon(&godaemon.DaemonAttr{})
	maxDisconnects := flag.Int("max_disconnects", 5, "Maximum number of negative polls before a reboot is triggered.")
	sleepPeriod := flag.Duration("sleep_period", 10*time.Second, "How often to poll the USB connection.")
	flag.Parse()
	count := 0
	for count < *maxDisconnects {
		if out, err := exec.Command("/system/bin/dumpsys", "usb").Output(); err != nil {
			logcatLog(logError, "Problem running dumpsys %s", err)
			count++
		} else {
			if strings.Contains(string(out), "mConnected: true") {
				count = 0
			} else {
				count++
				logcatLog(logInfo, "We seem to be disconnected: %d", count)
			}
		}
		time.Sleep(*sleepPeriod)
	}
	logcatLog(logError, "Rebooting")
	logcatLog(logError, "%v", rebootDevice())
	return 0
}

func main() {
	paniccatcher.Do(func() {
		os.Exit(realMain())
	}, func(p *paniccatcher.Panic) {
		logcatLog(logError, "Panic: %s\n%s", p.Reason, p.Stack)
		os.Exit(1)
	})
}
