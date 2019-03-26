package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/cq"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	configFile          = flag.String("cfg-file", "", "commit-queue.cfg file to edit.")
	newBranch           = flag.String("new-branch", "", "Short name of the new branch.")
	oldBranch           = flag.String("old-branch", "master", "Short name of the existing branch whose config to copy.")
	excludeTrybots      = common.NewMultiStringFlag("exclude-trybots", nil, "Regular expressions for trybot names to exclude.")
	includeExperimental = flag.Bool("include-experimental", false, "If true, include experimental trybots.")
	includeTreeCheck    = flag.Bool("include-tree-check", false, "If true, include tree open check.")
)

func main() {
	common.Init()

	if *configFile == "" {
		sklog.Fatal("--cfg-file is required.")
	}
	if *newBranch == "" {
		sklog.Fatal("--new-branch is required.")
	}
	excludeTrybotRegexp := make([]*regexp.Regexp, 0, len(*excludeTrybots))
	for _, excludeTrybot := range *excludeTrybots {
		re, err := regexp.Compile(excludeTrybot)
		if err != nil {
			sklog.Fatalf("Failed to compile regular expression from %q; %s", excludeTrybot, err)
		}
		excludeTrybotRegexp = append(excludeTrybotRegexp, re)
	}

	// Read the config file.
	oldCfgBytes, err := ioutil.ReadFile(*configFile)
	if err != nil {
		sklog.Fatalf("Failed to read %s; %s", *configFile, err)
	}
	var cfg cq.Config
	if err := proto.UnmarshalText(string(oldCfgBytes), &cfg); err != nil {
		sklog.Fatalf("Failed to parse config proto: %s", err)
	}

	// Find the CQ config for the old branch.
	oldRef := fmt.Sprintf("refs/heads/%s", *oldBranch)
	oldCg, oldGerrit, oldProject, err := cq.MatchConfigGroup(&cfg, oldRef)
	if err != nil {
		sklog.Fatalf("Failed to find config group for %q: %s", oldRef, err)
	}
	if oldCg == nil {
		sklog.Fatalf("No config group matches %q", oldRef)
	}

	// Create the CQ config for the new branch.
	newCg := &cq.ConfigGroup{
		Gerrit: []*cq.ConfigGroup_Gerrit{
			&cq.ConfigGroup_Gerrit{
				Url: oldGerrit.Url,
				Projects: []*cq.ConfigGroup_Gerrit_Project{
					&cq.ConfigGroup_Gerrit_Project{
						Name: oldProject.Name,
						RefRegexp: []string{
							fmt.Sprintf("refs/heads/%s", *newBranch),
						},
					},
				},
			},
		},
	}
	if oldCg.Verifiers != nil {
		newCg.Verifiers = &cq.Verifiers{
			GerritCqAbility: oldCg.Verifiers.GerritCqAbility,
			Deprecator:      oldCg.Verifiers.Deprecator,
			Fake:            oldCg.Verifiers.Fake,
		}
		if *includeTreeCheck {
			newCg.Verifiers.TreeStatus = oldCg.Verifiers.TreeStatus
		}
		if oldCg.Verifiers.Tryjob != nil {
			tryjobs := make([]*cq.Verifiers_Tryjob_Builder, 0, len(oldCg.Verifiers.Tryjob.Builders))
			for _, tj := range oldCg.Verifiers.Tryjob.Builders {
				exclude := false
				for _, re := range excludeTrybotRegexp {
					if re.MatchString(tj.Name) {
						exclude = true
						break
					}
				}
				if tj.ExperimentPercentage != 0.0 && !*includeExperimental {
					exclude = true
				}
				if !exclude {
					tryjobs = append(tryjobs, tj)
				}
			}
			newCg.Verifiers.Tryjob = &cq.Verifiers_Tryjob{
				Builders:    tryjobs,
				RetryConfig: oldCg.Verifiers.Tryjob.RetryConfig,
			}
		}
	}

	// Write the new config file.
	cfg.ConfigGroups = append(cfg.ConfigGroups, newCg)
	var buf bytes.Buffer
	if err := proto.MarshalText(&buf, &cfg); err != nil {
		sklog.Fatalf("Failed to encode config file: %s", err)
	}

	// We like curly braces instead of angle brackets.
	newCfgStr := buf.String()
	newCfgStr = strings.Replace(newCfgStr, ": <", " {", -1)
	newCfgStr = strings.Replace(newCfgStr, ">", "}", -1)

	// Compare the old config file to the new. The files should be identical
	// up until the new ConfigGroup. This adds back any comments that were
	// lost when we parsed the file.
	oldCfgLines := strings.Split(string(oldCfgBytes), "\n")
	newCfgLines := strings.Split(newCfgStr, "\n")
	lastCfgGroupIdx := len(newCfgLines)
	for i := len(newCfgLines) - 1; i >= 0; i-- {
		if strings.HasPrefix(strings.TrimSpace(newCfgLines[i]), "config_groups") {
			lastCfgGroupIdx = i
			break
		}
	}
	newCfgLines = append(oldCfgLines, newCfgLines[lastCfgGroupIdx:]...)
	newCfgStr = strings.Join(newCfgLines, "\n")

	if err := util.WithWriteFile(*configFile, func(w io.Writer) error {
		_, err := w.Write([]byte(newCfgStr))
		return err
	}); err != nil {
		sklog.Fatalf("Failed to write config file: %s", err)
	}
}
