package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
)

const (
	SkCQCfgPath = "infra/skcq.json"
)

// SCHEMA of the old commit-queue.cfg is here:
// https://luci-config.appspot.com/schemas/projects:commit-queue.cfg
// https://chromium.googlesource.com/infra/luci/luci-go/+/refs/heads/main/cv/api/config/v2/cq.proto
// Example commit-queue.cfg is here:
// https://skia.googlesource.com/skia/+/infra/config/generated/commit-queue.cfg

// Skipping submit_options - implement it by default!
// Skipping retry config - implement it by default!

// SkCQBot requests a single bot that is will be run on a repo+branch.
type SkCQBot struct {
	// Name of the SkCQ bot.
	Name string `json:"name"`

	// The SkCQ bot should be triggered if the change contains any of the following location regexes.
	LocationRegexes []string `json:"location_regexes"`
}

// SkCQCfg is a struct which describes the SkCQ config for a repo+branch at a particular commit.
type SkCQCfg struct {

	// TODO(rmistry): What happens if this is missing???
	// Will use the internal skcq-fe instance to display results if true.
	Internal bool `json:"internal"`

	// Full path to tasks.json file to read CQ bots from.
	TasksJSONPath string `json:"tasks_json_path"`

	// Name of the go/cria group that includes the list of people allowed to
	// this repo+branch.
	CommitterList string `json:"committer_list"`

	// Name of the go/cria group that includes the list of people allowed to
	// run dry runs on this repo+branch.
	DryRunAccessList string `json:"dry_run_access_list"`

	// TODO(rmistry): WOULD BE BETTER TO DIRECTLY POINT TO THE GET URL HERE TO MAKE IT EXPLICIT
	// The URL of the tree status instance that will gate submissions to this
	// repo+branch when the tree is closed.
	TreeStatusURL string `json:"tree_status_url"`

	// Will be populated from the above TasksJSONPath.
	SkCQBots []SkCQBot `json:"-"`
}

// Validate returns an error if the SkCQCfg is not valid.
func (c *SkCQCfg) Validate() error {
	if c.CommitterList == "" {
		return skerr.Fmt("Must specify a CommitterList")
	}
	if c.DryRunAccessList == "" {
		return skerr.Fmt("Must specify a DryRunAccessList")
	}
	return nil
}

// ParseSkCQCfg parses the given SkCQ cfg file contents and returns the config.
func ParseSkCQCfg(contents string) (*SkCQCfg, error) {
	var rv SkCQCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read SkCQ cfg: could not parse file: %s\nContents:\n%s", err, string(contents))
	}

	return &rv, nil
}

// ReadSkCQCfg reads the SkCQ cfg file from the given gitiles dir and returns it.
// TODO(rmistry): Rename this to be process something or something else because of the CQ bots thingy.
func ReadSkCQCfg(ctx context.Context, gitilesRepo *gitiles.Repo, repo, ref string) (*SkCQCfg, error) {
	contents, err := gitilesRepo.ReadFileAtRef(ctx, SkCQCfgPath, ref)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found for %s/%s", SkCQCfgPath, repo, ref)
		}
		return nil, fmt.Errorf("Failed to read SkCQ cfg: could not read file: %s", err)
	}
	cfg, err := ParseSkCQCfg(string(contents))
	if err != nil {
		return nil, fmt.Errorf("Error when parsing SkCQ cfg: %s", err)
	}
	if cfg.TasksJSONPath != "" {
		// TODO(rmistry): Make sure the path exists and then parse commit queue bots from tasks.json
	}
	return cfg, nil
}
