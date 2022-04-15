package config

import (
	"encoding/json"
	"html/template"

	"go.skia.org/infra/go/skerr"
)

// Config is a struct that contains all required configurations of projects
// supported by the npm-audit-mirror framework.
type Config struct {
	SupportedProjects map[string]*SupportedProject `json:"supported_projects"`
}

type SupportedProject struct {
	// A gitiles link to the repo. Eg: "https://skia.googlesource.com/buildbot.git".
	RepoURL string `json:"repo_url"`
	// The branch to use in the above repo. Eg: "main".
	GitBranch string `json:"git_branch"`
	// The directory in the above repo that contains the package.json file
	// we want to audit. Use empty string if it is at the top-level.
	// Eg: "" or "appengine/monorail".
	PackageJSONDir string `json:"package_json_dir"`
	// The packages we skip pre-download checks for.
	PackagesAllowList []PackagesAllowList `json:"packages_allow_list,omitempty"`
	// The scopes we skip pre-download checks for.
	TrustedScopes []string `json:"trusted_scopes,omitempty"`
	// The monorail config used for filing audit bugs.
	MonorailConfig *MonorailConfig `json:"monorail_config,omitempty"`
}

type MonorailConfig struct {
	InstanceName    string   `json:"instance_name"`
	Owner           string   `json:"owner"`
	Labels          []string `json:"labels"`
	ComponentDefIDs []string `json:"component_def_ids"`
}

type PackagesAllowList struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GetConfig is a utility function that returns the npm audit mirror config.
func GetConfig() (*Config, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(NpmAuditMirrorConfig), &cfg); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse the config file with contents:\n%s", NpmAuditMirrorConfig)
	}
	return &cfg, nil
}

// GetMirrorConfigTmpl is utility function that returns a template of the
// verdaccio config.
func GetMirrorConfigTmpl() (*template.Template, error) {
	tmpl, err := template.New("verdaccio-config-template").Parse(VerdaccioConfigTemplate)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse the verdaccio template file with contents:\n%s", VerdaccioConfigTemplate)
	}
	return tmpl, nil
}
