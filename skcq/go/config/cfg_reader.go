package config

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	SkCQCfgPath = "infra/skcq.json"
)

// ConfigNotFoundError is returned when a cfg is not found in the repo+branch.
type ConfigNotFoundError struct {
	configPath string
	repo       string
	branch     string
}

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("Config %s not found in %s/%s", e.configPath, e.repo, e.branch)
}

func IsNotFound(err error) bool {
	_, ok := err.(*ConfigNotFoundError)
	return ok
}

// CannotModifyCfgsOnTheFlyError is returned when the owner of the change does
// not have permission to modify the cfg.
type CannotModifyCfgsOnTheFlyError struct {
	issueID    int64
	issueOwner string
}

func (e *CannotModifyCfgsOnTheFlyError) Error() string {
	return fmt.Sprintf("Config was modified in %d but the owner %s does not have permission to run it", e.issueID, e.issueOwner)
}

func IsCannotModifyCfgsOnTheFly(err error) bool {
	_, ok := err.(*CannotModifyCfgsOnTheFlyError)
	return ok
}

// ConfigReader is an interface to read configs for SkCQ. Useful for testing.
type ConfigReader interface {
	// GetSkCQCfg reads the SkCQ cfg file from CL's ref if it was modified, else
	// it reads it from HEAD.
	GetSkCQCfg(ctx context.Context) (*SkCQCfg, error)

	// GetTasksCfg reads the Tasks.json file from CL's ref if it was modified, else
	// it reads it from HEAD.
	GetTasksCfg(ctx context.Context, tasksJSONPath string) (*specs.TasksCfg, error)
}

// GitilesConfigReader is an implementation of ConfigReader interface.
type GitilesConfigReader struct {
	gitilesRepo           gitiles.GitilesRepo
	ci                    *gerrit.ChangeInfo
	cr                    codereview.CodeReview
	changedFiles          []string
	canModifyCfgsOnTheFly allowed.Allow
}

// NewGitilesConfigReader returns an instance of GitilesConfigReader.
func NewGitilesConfigReader(ctx context.Context, httpClient *http.Client, ci *gerrit.ChangeInfo, cr codereview.CodeReview, canModifyCfgsOnTheFly allowed.Allow) (*GitilesConfigReader, error) {
	gitilesRepo := gitiles.NewRepo(cr.GetRepoUrl(ci), httpClient)
	changedFiles, err := cr.GetFileNames(ctx, ci)
	if err != nil {
		return nil, skerr.Fmt("Not able to get changed files for %d: %s", ci.Issue, err)
	}
	return &GitilesConfigReader{
		gitilesRepo:           gitilesRepo,
		cr:                    cr,
		ci:                    ci,
		changedFiles:          changedFiles,
		canModifyCfgsOnTheFly: canModifyCfgsOnTheFly,
	}, nil
}

// GetSkCQCfg implements the ConfigReader interface.
func (gc *GitilesConfigReader) GetSkCQCfg(ctx context.Context) (*SkCQCfg, error) {
	// If SkCQ cfg is in list of changed files then use that. Else use from HEAD.
	contents, modifiedInCL, err := gc.getFileContents(ctx, SkCQCfgPath)
	if err != nil {
		return nil, err
	}
	if modifiedInCL && !gc.canModifyCfgsOnTheFly.Member(gc.ci.Owner.Email) {
		return nil, &CannotModifyCfgsOnTheFlyError{
			issueID:    gc.ci.Issue,
			issueOwner: gc.ci.Owner.Email,
		}
	}
	cfg, err := ParseSkCQCfg(contents)
	if err != nil {
		return nil, skerr.Fmt("Error when parsing SkCQ cfg: %s", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, skerr.Wrapf(err, "Error validating SkCQ cfg")
	}
	return cfg, nil
}

// GetTasksCfg implements the ConfigReader interface.
func (gc *GitilesConfigReader) GetTasksCfg(ctx context.Context, tasksJSONPath string) (*specs.TasksCfg, error) {
	// If tasks.json is in list of changed files then use that. Else use from HEAD.
	contents, modifiedInCL, err := gc.getFileContents(ctx, tasksJSONPath)
	if err != nil {
		return nil, err
	}
	if modifiedInCL && !gc.canModifyCfgsOnTheFly.Member(gc.ci.Owner.Email) {
		return nil, &CannotModifyCfgsOnTheFlyError{
			issueID:    gc.ci.Issue,
			issueOwner: gc.ci.Owner.Email,
		}
	}
	cfg, err := specs.ParseTasksCfg(contents)
	if err != nil {
		return nil, skerr.Fmt("Error when parsing tasks.json cfg: %s", err)
	}
	return cfg, nil
}

// getFileContents checks to see if the CL has modified the file and returns those contents.
// If the file has not been modified then it returns the file contents from HEAD.
func (gc *GitilesConfigReader) getFileContents(ctx context.Context, cfgPath string) (string, bool, error) {
	ref := gc.ci.Branch
	modifiedInCL := false
	for _, f := range gc.changedFiles {
		if f == cfgPath {
			sklog.Infof("[%d] Has modified %s. Using the modified version.", gc.ci.Issue, cfgPath)
			ref = gc.cr.GetChangeRef(gc.ci)
			modifiedInCL = true
			break
		}
	}
	contents, err := gc.gitilesRepo.ReadFileAtRef(ctx, cfgPath, ref)
	if err != nil {
		if strings.Contains(err.Error(), "NOT_FOUND") {
			return "", modifiedInCL, &ConfigNotFoundError{
				configPath: cfgPath,
				repo:       gc.ci.Project,
				branch:     gc.ci.Branch,
			}
		}
		return "", modifiedInCL, skerr.Fmt("Failed to read %s: %s", cfgPath, err)
	}
	if !modifiedInCL {
		sklog.Infof("[%d] has not modified %s. Using the version from HEAD.", gc.ci.Issue, cfgPath)
	}
	return string(contents), modifiedInCL, nil
}
