package main

/*
	Read and validate an AutoRoll config file.
*/

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/conversion"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/lib/git_steps"
	"go.skia.org/infra/task_driver/go/td"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	// Required properties for this task.
	projectId         = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId            = flag.String("task_id", "", "ID of this task.")
	taskName          = flag.String("task_name", "", "Name of the task.")
	workdir           = flag.String("workdir", ".", "Working directory")
	configsFlag       = common.NewMultiStringFlag("config", nil, "Config file or dir of config files to validate. May be specified multiple times.")
	checkGCSArtifacts = flag.Bool("check-gcs-artifacts", false, "If true, filter out rollers whose GCS artifacts are missing.")

	// Optional flags.
	local  = flag.Bool("local", false, "True if running locally (as opposed to on the bots)")
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
)

var (
	// "constants"
	chromiumGerritHosts = []string{
		"https://chromium-review.googlesource.com",
		"https://chrome-internal-review.googlesource.com",
	}
)

func validateConfig(ctx context.Context, content []byte) (string, error) {
	// Decode the config.
	var cfg config.Config
	if err := prototext.Unmarshal(content, &cfg); err != nil {
		return "", skerr.Wrap(err)
	}

	// Validate the config.
	if err := cfg.Validate(); err != nil {
		return "", skerr.Wrap(err)
	}

	gerrit := cfg.GetGerrit()
	if gerrit != nil && util.In(gerrit.Url, chromiumGerritHosts) && (gerrit.Config != config.GerritConfig_CHROMIUM_BOT_COMMIT && gerrit.Config != config.GerritConfig_CHROMIUM_BOT_COMMIT_NO_CQ) {
		return "", skerr.Fmt("Chromium rollers must use Gerrit config CHROMIUM_BOT_COMMIT")
	}

	return cfg.RollerName, nil
}

func readAndValidateConfig(ctx context.Context, f string) (string, error) {
	var rollerName string
	return rollerName, td.Do(ctx, td.Props(fmt.Sprintf("Validate %s", f)), func(ctx context.Context) error {
		content, err := ioutil.ReadFile(f)
		if err != nil {
			return skerr.Wrap(err)
		}
		rollerName, err = validateConfig(ctx, content)
		return skerr.Wrap(err)
	})
}

func validateTemplate(ctx context.Context, client *http.Client, vars *conversion.TemplateVars, f string) ([]string, error) {
	var rollerNames []string
	err := td.Do(ctx, td.Props(fmt.Sprintf("Validate %s", f)), func(ctx context.Context) error {
		tmplContents, err := ioutil.ReadFile(f)
		if err != nil {
			return skerr.Wrap(err)
		}
		generatedConfigs, err := conversion.ProcessTemplate(ctx, client, f, string(tmplContents), vars, *checkGCSArtifacts)
		if err != nil {
			return skerr.Wrapf(err, "failed to process template file %s", f)
		}
		for _, cfgBytes := range generatedConfigs {
			rollerName, err := validateConfig(ctx, cfgBytes)
			if err != nil {
				return skerr.Wrap(err)
			}
			rollerNames = append(rollerNames, rollerName)
		}
		return nil
	})
	return rollerNames, err
}

func main() {
	// Setup.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	if len(*configsFlag) == 0 {
		td.Fatalf(ctx, "--config is required.")
	}
	ts, err := git_steps.Init(ctx, true)
	if err != nil {
		td.Fatal(ctx, err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	vars, err := conversion.CreateTemplateVars(ctx, client, "", "")
	if err != nil {
		td.Fatalf(ctx, "Failed to create template vars: %s", err)
	}

	// Gather files to validate.
	configFiles := []string{}
	templateFiles := []string{}
	for _, cfgPath := range *configsFlag {
		if err := filepath.Walk(cfgPath, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if strings.HasSuffix(info.Name(), ".cfg") {
					configFiles = append(configFiles, path)
				}
				if strings.HasSuffix(info.Name(), ".tmpl") {
					templateFiles = append(templateFiles, path)
				}
			}
			return nil
		}); err != nil {
			td.Fatal(ctx, err)
		}
	}

	// Validate the file(s).
	rollers := make(map[string]string, len(configFiles))
	for _, f := range configFiles {
		rollerName, err := readAndValidateConfig(ctx, f)
		if err != nil {
			td.Fatalf(ctx, "%s failed validation: %s", f, err)
		}
		if otherFile, ok := rollers[rollerName]; ok {
			td.Fatalf(ctx, "Roller %q is defined in both %s and %s", rollerName, f, otherFile)
		}
		rollers[rollerName] = f
	}
	for _, f := range templateFiles {
		rollerNames, err := validateTemplate(ctx, client, vars, f)
		if err != nil {
			td.Fatalf(ctx, "%s failed validation: %s", f, err)
		}
		for _, rollerName := range rollerNames {
			if otherFile, ok := rollers[rollerName]; ok {
				td.Fatalf(ctx, "Roller %q is defined in both %s and %s", rollerName, f, otherFile)
			}
			rollers[rollerName] = f
		}
	}
	sklog.Infof("Validated %d files.", len(configFiles))
}
