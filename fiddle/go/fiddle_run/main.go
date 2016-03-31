// Compiles a fiddle and then runs the fiddle. The output of both processes is
// combined into a single JSON output.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/buildskia"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
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
		glog.Errorf("Failed to encode: %s", err)
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
	checkout := path.Join(*fiddleRoot, "versions", *gitHash)

	// Compile draw.cpp and link against fiddle_main.o and libskia to produce fiddle_main.
	files := []string{
		filepath.Join(*fiddleRoot, "src", "draw.cpp"),
	}
	linkArgs := []string{path.Join(checkout, "cmakeout", "fiddle_main.o"), "-lOSMesa"}
	compilePaths := []string{path.Join(checkout, "experimental", "fiddle")}
	compileOutput, err := buildskia.CMakeCompileAndLink(checkout, path.Join(*fiddleRoot, "out", "fiddle_main"), files, compilePaths, linkArgs)
	if err != nil {
		res.Compile.Errors = err.Error()
	}
	res.Compile.Output = compileOutput

	if err != nil {
		serializeOutput(res)
		return
	}

	// Now that we've built fiddle_main we want to run it as:
	//
	//    $ bin/fiddle_secwrap out/fiddle_main
	//
	// in the container, or as
	//
	//    $ out/fiddle_main
	//
	// if running locally.
	name := path.Join(*fiddleRoot, "bin", "fiddle_secwrap")
	args := []string{path.Join(*fiddleRoot, "out", "fiddle_main")}
	if *local {
		name = path.Join(*fiddleRoot, "out", "fiddle_main")
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
		Timeout:     10 * time.Second,
	}
	if err := exec.Run(runCmd); err != nil {
		res.Execute.Errors = err.Error()
	}
	if res.Execute.Errors != "" && stderr.String() != "" {
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
