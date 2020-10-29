package cq

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/duration"
	"go.chromium.org/luci/cv/api/config/v2"
)

const (
	cqConfigHeader = `# See http://luci-config.appspot.com/schemas/projects:commit-queue.cfg for the
# documentation of this file format.

`
)

// WithUpdateCQConfig parses the given bytes as a Config, runs the given
// function to modify the Config, then returns the updated bytes.
func WithUpdateCQConfig(oldCfgBytes []byte, fn func(*config.Config) error) ([]byte, error) {
	// Parse the Config.
	var cfg config.Config
	if err := proto.UnmarshalText(string(oldCfgBytes), &cfg); err != nil {
		return nil, fmt.Errorf("Failed to parse config proto: %s", err)
	}

	// Run the passed-in func.
	if err := fn(&cfg); err != nil {
		return nil, fmt.Errorf("Config modification failed: %s", err)
	}

	// Write the new config bytes.
	var buf bytes.Buffer
	if err := proto.MarshalText(&buf, &cfg); err != nil {
		return nil, fmt.Errorf("Failed to encode config file: %s", err)
	}

	// We like curly braces instead of angle brackets.
	newCfgStr := buf.String()
	newCfgStr = strings.Replace(newCfgStr, ": <", " {", -1)
	newCfgStr = strings.Replace(newCfgStr, ">", "}", -1)

	return []byte(cqConfigHeader + newCfgStr), nil
}

// CloneBranch updates the given CQ config to create a config for a new
// branch based on a given existing branch. Optionally, include experimental
// tryjobs, include the tree-is-open check, and exclude trybots matching regular
// expressions.
func CloneBranch(cfg *config.Config, oldBranch, newBranch string, includeExperimental, includeTreeCheck bool, excludeTrybotRegexp []*regexp.Regexp) error {
	// Find the CQ config for the old branch.
	oldRef := fmt.Sprintf("refs/heads/%s", oldBranch)
	oldCg, oldGerrit, oldProject, err := MatchConfigGroup(cfg, oldRef)
	if err != nil {
		return fmt.Errorf("Failed to find config group for %q: %s", oldRef, err)
	}
	if oldCg == nil {
		return fmt.Errorf("No config group matches %q", oldRef)
	}

	// Create the CQ config for the new branch.
	newCg := &config.ConfigGroup{
		Name: newBranch,
		Gerrit: []*config.ConfigGroup_Gerrit{
			{
				Url: oldGerrit.Url,
				Projects: []*config.ConfigGroup_Gerrit_Project{
					{
						Name: oldProject.Name,
						RefRegexp: []string{
							fmt.Sprintf("refs/heads/%s", newBranch),
						},
					},
				},
			},
		},
	}
	if oldCg.CombineCls != nil {
		newCg.CombineCls = &config.CombineCLs{}
		if oldCg.CombineCls.StabilizationDelay != nil {
			newCg.CombineCls.StabilizationDelay = &duration.Duration{
				Seconds: oldCg.CombineCls.StabilizationDelay.Seconds,
				Nanos:   oldCg.CombineCls.StabilizationDelay.Nanos,
			}
		}
	}
	if oldCg.Verifiers != nil {
		newCg.Verifiers = &config.Verifiers{
			GerritCqAbility: oldCg.Verifiers.GerritCqAbility,
			Fake:            oldCg.Verifiers.Fake,
		}
		if includeTreeCheck {
			newCg.Verifiers.TreeStatus = oldCg.Verifiers.TreeStatus
		}
		if oldCg.Verifiers.Tryjob != nil {
			tryjobs := make([]*config.Verifiers_Tryjob_Builder, 0, len(oldCg.Verifiers.Tryjob.Builders))
			for _, tj := range oldCg.Verifiers.Tryjob.Builders {
				exclude := false
				for _, re := range excludeTrybotRegexp {
					if re.MatchString(tj.Name) {
						exclude = true
						break
					}
				}
				if tj.ExperimentPercentage != 0.0 && !includeExperimental {
					exclude = true
				}
				if !exclude {
					tryjobs = append(tryjobs, tj)
				}
			}
			newCg.Verifiers.Tryjob = &config.Verifiers_Tryjob{
				Builders:    tryjobs,
				RetryConfig: oldCg.Verifiers.Tryjob.RetryConfig,
			}
		}
	}
	cfg.ConfigGroups = append(cfg.ConfigGroups, newCg)
	return nil
}

// DeleteBranch updates the given CQ config to delete the config matching the
// given branch.
func DeleteBranch(cfg *config.Config, branch string) error {
	cg, _, _, err := MatchConfigGroup(cfg, fmt.Sprintf("refs/heads/%s", branch))
	if err != nil {
		return err
	}
	newGroups := make([]*config.ConfigGroup, 0, len(cfg.ConfigGroups))
	for _, g := range cfg.ConfigGroups {
		if g != cg {
			newGroups = append(newGroups, g)
		}
	}
	cfg.ConfigGroups = newGroups
	return nil
}
