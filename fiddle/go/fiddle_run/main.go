// Compiles a fiddle and then runs the fiddle. The output of both processes is
// combined into a single JSON output.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
)

// flags
var (
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	fiddleRoot = flag.String("fiddle_root", "", "Directory location where all the work is done.")
	gitHash    = flag.String("git_hash", "", "The version of Skia code to run against.")
)

func serializeOutput(res types.Result) {
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(res); err != nil {
		fmt.Printf("Failed to encode: %s", err)
	}
}

func main() {
	common.Init()
	res := types.Result{
		Compile: types.Compile{},
		Execute: types.Execute{
			Errors: "",
			Output: types.Output{},
		},
	}
	if *fiddleRoot == "" {
		res.Errors = "fiddle_run: The --fiddle_root flag is required."
	}
	if *gitHash == "" {
		res.Errors = "fiddle_run: The --git_hash flag is required."
	}

	depotTools := filepath.Join(*fiddleRoot, "depot_tools")
	checkout := path.Join(*fiddleRoot, "versions", *gitHash)

	// Set limits on this process and all its children.

	// Limit total CPU seconds.
	rLimit := &syscall.Rlimit{
		Cur: 20,
		Max: 20,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CPU, rLimit); err != nil {
		fmt.Println("Error Setting Rlimit ", err)
	}
	// Do not emit core dumps.
	rLimit = &syscall.Rlimit{
		Cur: 0,
		Max: 0,
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_CORE, rLimit); err != nil {
		fmt.Println("Error Setting Rlimit ", err)
	}

	// Compile draw.cpp into 'fiddle'.
	if output, err := buildskia.GNNinjaBuild(checkout, depotTools, "Release", "fiddle", true); err != nil {
		res.Compile.Errors = err.Error()
		res.Compile.Output = output
		serializeOutput(res)
		return
	}

	// Now that we've built fiddle we want to run it as:
	//
	//    $ bin/fiddle_secwrap out/fiddle
	//
	// in the container, or as
	//
	//    $ out/Release/fiddle
	//
	// if running locally.
	name := path.Join(*fiddleRoot, "bin", "fiddle_secwrap")
	args := []string{path.Join(checkout, "skia", "out", "Release", "fiddle")}
	if *local {
		name = path.Join(checkout, "skia", "out", "Release", "fiddle")
		args = []string{}
	}

	stderr := bytes.Buffer{}
	stdout := bytes.Buffer{}
	runCmd := &exec.Command{
		Name:        name,
		Args:        args,
		Dir:         *fiddleRoot,
		InheritPath: true,
		Stdout:      &stdout,
		Stderr:      &stderr,
		Timeout:     20 * time.Second,
	}
	if err := exec.Run(runCmd); err != nil {
		sklog.Errorf("Failed to run: %s", err)
		res.Execute.Errors = err.Error()
	}
	if res.Execute.Errors != "" && stderr.String() != "" {
		sklog.Errorf("Found stderr output: %q", stderr.String())
		res.Execute.Errors += "\n"
	}
	res.Execute.Errors += stderr.String()
	if err := json.Unmarshal(stdout.Bytes(), &res.Execute.Output); err != nil {
		if res.Execute.Errors != "" {
			res.Execute.Errors += "\n"
		}
		res.Execute.Errors += err.Error()
	}

	serializeOutput(res)
}
