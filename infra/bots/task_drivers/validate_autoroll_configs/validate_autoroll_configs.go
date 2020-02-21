package main

/*
	Read and validate an AutoRoll config file.
*/

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required properties for this task.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	workdir   = flag.String("workdir", ".", "Working directory")
	config    = flag.String("config", "", "Config file or dir of config files to validate.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

func validateConfig(ctx context.Context, f string) error {
	return td.Do(ctx, td.Props(fmt.Sprintf("Validate %s", f)), func(ctx context.Context) error {
		var cfg roller.AutoRollerConfig
		if err := util.WithReadFile(f, func(r io.Reader) error {
			// TODO(borenet): This will just ignore any extraneous keys!
			return json5.NewDecoder(r).Decode(&cfg)
		}); err != nil {
			return fmt.Errorf("Failed to read %s: %s", f, err)
		}
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("%s failed validation: %s", f, err)
		}
		return nil
	})
}

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if *config == "" {
		td.Fatalf(ctx, "--config is required.")
	}

	// Gather files to validate.
	configFiles := []string{}
	f, err := os_steps.Stat(ctx, *config)
	if err != nil {
		td.Fatal(ctx, err)
	}
	if f.IsDir() {
		files, err := os_steps.ReadDir(ctx, *config)
		if err != nil {
			td.Fatal(ctx, err)
		}
		for _, f := range files {
			// Ignore subdirectories and file names starting with '.'
			if !f.IsDir() && f.Name()[0] != '.' {
				configFiles = append(configFiles, filepath.Join(*config, f.Name()))
			}
		}
	} else {
		configFiles = append(configFiles, *config)
	}

	// Validate the file(s).
	for _, f := range configFiles {
		if err := validateConfig(ctx, f); err != nil {
			td.Fatal(ctx, err)
		}
	}
	sklog.Infof("Validated %d files.", len(configFiles))
}
