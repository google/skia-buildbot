package main

import (
	"flag"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/flynn/json5"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	fileHeader = `# See https://skia.googlesource.com/buildbot.git/+show/master/autoroll/go/config/config.proto
# for the structure of this file.

`
)

var (
	configFlag = flag.String("config", "", "Config file or dir of config files to convert.")
)

func convertConfig(f string) error {
	// Read the old-style config file.
	var cfg roller.AutoRollerConfig
	if err := util.WithReadFile(f, func(f io.Reader) error {
		return json5.NewDecoder(f).Decode(&cfg)
	}); err != nil {
		return skerr.Wrap(err)
	}

	// Convert to the new style.
	newCfg, err := roller.AutoRollerConfigToProto(&cfg)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Write the new config file.
	b, err := prototext.MarshalOptions{
		Indent: "  ",
	}.Marshal(newCfg)
	if err != nil {
		return skerr.Wrap(err)
	}
	newFileName := strings.Replace(f, ".json", ".cfg", 1)
	return util.WithWriteFile(newFileName, func(w io.Writer) error {
		_, err := w.Write([]byte(fileHeader))
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	})
}

func main() {
	common.Init()

	if *configFlag == "" {
		sklog.Fatalf("--config is required.")
	}

	// Gather files to validate.
	configFiles := []string{}
	f, err := os.Stat(*configFlag)
	if err != nil {
		sklog.Fatal(err)
	}
	if f.IsDir() {
		files, err := ioutil.ReadDir(*configFlag)
		if err != nil {
			sklog.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".json") {
				configFiles = append(configFiles, filepath.Join(*configFlag, f.Name()))
			}
		}
	} else {
		configFiles = append(configFiles, *configFlag)
	}

	// Convert the file(s).
	for _, f := range configFiles {
		if err := convertConfig(f); err != nil {
			sklog.Fatalf("%s failed conversion: %s", f, err)
		}
	}
	sklog.Infof("Converted %d files.", len(configFiles))
}
