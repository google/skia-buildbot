package main

/*
	Read and validate an AutoRoll config file.
*/

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	config = flag.String("config", "", "Config file or dir of config files to validate.")
)

func validateConfig(f string) error {
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
}

func main() {
	common.Init()

	if *config == "" {
		sklog.Fatal("--config is required.")
	}

	// Gather files to validate.
	configFiles := []string{}
	f, err := os.Stat(*config)
	if err != nil {
		sklog.Fatal(err)
	}
	if f.IsDir() {
		files, err := ioutil.ReadDir(*config)
		if err != nil {
			sklog.Fatal(err)
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
		if err := validateConfig(f); err != nil {
			sklog.Fatal(err)
		}
	}
	sklog.Infof("Validated %d files.", len(configFiles))
}
