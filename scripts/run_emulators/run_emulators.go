package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/emulators/cockroachdb_instance"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/sklog"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [start|stop]\n", filepath.Base(os.Args[0]))
	os.Exit(1)
}

func main() {
	common.Init()

	if len(os.Args) != 2 {
		usage()
	}
	start := false
	if os.Args[1] == "start" {
		start = true
	} else if os.Args[1] != "stop" {
		usage()
	}
	if err := emulators.StopAllEmulators(); err != nil {
		sklog.Fatal(err)
	}
	if start {
		if err := emulators.StartAllEmulators(); err != nil {
			sklog.Fatal(err)
		}
		if _, err := cockroachdb_instance.StartCockroachDBIfNotRunning(); err != nil {
			sklog.Fatal(err)
		}
		time.Sleep(5 * time.Second)
		fmt.Println("Emulators started. Set environment variables as follows:")
		for _, e := range emulators.AllEmulators {
			// We need to set the *_EMULATOR_HOST environment variable before we can read its value via
			// emulators.GetEmulatorHostEnvVar().
			if err := emulators.SetEmulatorHostEnvVar(e); err != nil {
				sklog.Fatal(err)
			}
			fmt.Println(fmt.Sprintf("export %s=%s", emulators.GetEmulatorHostEnvVarName(e), emulators.GetEmulatorHostEnvVar(e)))
		}
	} else {
		fmt.Println("Emulators stopped. Unset environment variables as follows:")
		for _, e := range emulators.AllEmulators {
			fmt.Println(fmt.Sprintf("export %s=", emulators.GetEmulatorHostEnvVarName(e)))
		}
	}
}
