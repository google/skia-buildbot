package config

import (
	"encoding/json"

	"go.skia.org/infra/go/skerr"
)

type VisibilityType string

const PublicVisibility VisibilityType = "public"
const StagingVisibility VisibilityType = "staging"
const InternalVisibility VisibilityType = "internal"

// CherrypickWatcherCfg is a struct that contains supported branch deps
// configurations.
type CherrypickWatcherCfg struct {

	// Contains a map of name of the supported branch dep to a struct describing
	// the dep.
	SupportedBranchDeps map[string]*SupportedBranchDep `json:"supported_branch_deps"`
}

type SupportedBranchDep struct {
	// Name of the source repo. Eg: skia
	SourceRepo string `json:"source_repo"`
	// Name of the source branch. Eg: chrome/m100
	SourceBranch string `json:"source_branch"`

	// Name of the target repo. Eg: skia
	TargetRepo string `json:"target_repo"`
	// Name of the target branch. Eg: android/next-releease
	TargetBranch string `json:"target_branch"`

	// Text that will be included in the reminder comment posted by this
	// framework.
	CustomMessage string `json:"custom_message"`
}

// ParseCfg is a utility function that parses the given config file and returns
// a slice of the supported branch deps.
func ParseCfg(cfgContents []byte) ([]*SupportedBranchDep, error) {
	var cfg CherrypickWatcherCfg
	if err := json.Unmarshal([]byte(cfgContents), &cfg); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse the config file with contents:\n%s", string(cfgContents))
	}

	supportedBranchDeps := []*SupportedBranchDep{}
	for _, bd := range cfg.SupportedBranchDeps {
		supportedBranchDeps = append(supportedBranchDeps, bd)
	}
	return supportedBranchDeps, nil
}
