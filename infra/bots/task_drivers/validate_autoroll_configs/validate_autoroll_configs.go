package main

/*
	Read and validate an AutoRoll config file.
*/

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/pmezard/go-difflib/difflib"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	// Required properties for this task.
	projectId  = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId     = flag.String("task_id", "", "ID of this task.")
	taskName   = flag.String("task_name", "", "Name of the task.")
	workdir    = flag.String("workdir", ".", "Working directory")
	configFlag = flag.String("config", "", "Config file or dir of config files to validate.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func validateConfig(ctx context.Context, f string) (string, error) {
	var rollerName string
	return rollerName, td.Do(ctx, td.Props(fmt.Sprintf("Validate %s", f)), func(ctx context.Context) error {
		// Decode the config.
		var cfg config.Config
		if err := util.WithReadFile(f, func(f io.Reader) error {
			b, err := ioutil.ReadAll(f)
			if err != nil {
				return skerr.Wrap(err)
			}
			if err := prototext.Unmarshal(b, &cfg); err != nil {
				return skerr.Wrap(err)
			}
			return nil
		}); err != nil {
			td.Fatalf(ctx, "%s failed validation: %s", f, err)
		}

		// Convert to the old-style config.
		oldCfg, err := roller.ProtoToConfig(&cfg)
		if err != nil {
			return skerr.Wrap(err)
		}

		// Validate the config.
		if err := oldCfg.Validate(); err != nil {
			return fmt.Errorf("%s failed validation: %s", f, err)
		}

		// Convert the config back to the proto version and back, ensuring that
		// we get the same config.
		newCfg, err := roller.AutoRollerConfigToProto(oldCfg)
		if err != nil {
			return skerr.Wrap(err)
		}
		opts := prototext.MarshalOptions{
			Indent: "  ",
		}
		origBytes, err := opts.Marshal(&cfg)
		if err != nil {
			return skerr.Wrap(err)
		}
		newBytes, err := opts.Marshal(newCfg)
		if err != nil {
			return skerr.Wrap(err)
		}
		if string(origBytes) != string(newBytes) {
			diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(origBytes)),
				B:        difflib.SplitLines(string(newBytes)),
				FromFile: "Original",
				ToFile:   "Converted",
				Context:  3,
				Eol:      "\n",
			})
			if err != nil {
				return skerr.Wrap(err)
			}
			return skerr.Fmt("Converted config file %s differs:\n%s", f, diff)
		}

		rollerName = cfg.RollerName
		return nil
	})
}

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *configFlag == "" {
		td.Fatalf(ctx, "--config is required.")
	}

	// Gather files to validate.
	configFiles := []string{}
	f, err := os_steps.Stat(ctx, *configFlag)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if f.IsDir() {
		files, err := os_steps.ReadDir(ctx, *configFlag)
		if err != nil {
			td.Fatal(ctx, err)
		}
		for _, f := range files {
			// Ignore subdirectories and file names starting with '.'
			if !f.IsDir() && f.Name()[0] != '.' {
				configFiles = append(configFiles, filepath.Join(*configFlag, f.Name()))
			}
		}
	} else {
		configFiles = append(configFiles, *configFlag)
	}

	// Validate the file(s).
	rollers := make(map[string]string, len(configFiles))
	for _, f := range configFiles {
		rollerName, err := validateConfig(ctx, f)
		if err != nil {
			td.Fatalf(ctx, "%s failed validation: %s", f, err)
		}
		if otherFile, ok := rollers[rollerName]; ok {
			td.Fatalf(ctx, "Roller %q is defined in both %s and %s", rollerName, f, otherFile)
		}
		rollers[rollerName] = f
	}
	sklog.Infof("Validated %d files.", len(configFiles))
}
