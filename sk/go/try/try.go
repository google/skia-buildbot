package try

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/task_scheduler/go/specs"
)

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "try",
		Short: "Run try jobs against the active CL",
		Long: `
Run try jobs against the active CL.
`,
		Run: tryCmd,
	}
	return cmd
}

func tryCmd(cmd *cobra.Command, args []string) {
}

func try(ctx context.Context, jobRequest string, internal bool) error {
	if err := fixupIssue(ctx); err != nil {
		return err
	}
	jobs, err := getTryJobs(ctx)
	if err != nil {
		return err
	}
	// TODO(borenet): Obtain internal if applicable.

	filteredJobs := jobs
	if jobRequest != "" {
		jobRegex, err := regexp.Compile(jobRequest)
		if err != nil {
			return err
		}
		filteredJobs = map[string][]string{}
		for bucket, jobList := range jobs {
			for _, job := range jobList {
				if jobRegex.MatchString(job) {
					filteredJobs[bucket] = append(filteredJobs[bucket], job)
				}
			}
		}
	}

	return nil
}

func fixupIssue(ctx context.Context) error {
	// If the change was uploaded using Depot Tools, the issue configuration
	// will already be present in the git config.
	output, err := exec.RunCwd(ctx, ".", "git", "branch", "--show-current")
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(output)

	if _, err := exec.RunCwd(ctx, ".", "git", "config", "--local", fmt.Sprintf("branch.%s.gerritissue", branch)); err != nil {
		// If the above failed, it's probably because the CL was not uploaded
		// using Depot Tools.  Find the Change-Id line in the most recent commit
		// and obtain the issue number using that.
		output, err = exec.RunCwd(ctx, ".", "git", "log", "n1", branch)
		if err != nil {
			return err
		}
		changeId, err := gerrit.ParseChangeId(output)
		if err != nil {
			return err
		}
		g, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, httputils.DefaultClientConfig().Client())
		if err != nil {
			return err
		}
		ci, err := g.GetChange(ctx, changeId)
		if err != nil {
			return err
		}
		issue := fmt.Sprintf("%d", ci.Issue)
		if _, err := exec.RunCwd(ctx, ".", "git", "cl", "issue", issue); err != nil {
			return err
		}
		return nil
	}
	return nil
}

// TODO(borenet): This assumes that the current repo is associated with the
// skia.primary bucket. This will work for most repos but it would be better to
// look up the correct bucket to use.
func getTryJobs(ctx context.Context) (map[string][]string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	tasksCfg, err := specs.ReadTasksCfg(repoRoot)
	if err != nil {
		return nil, err
	}
	jobs := make([]string, 0, len(tasksCfg.Jobs))
	for name, _ := range tasksCfg.Jobs {
		jobs = append(jobs, name)
	}
	return map[string][]string{
		"skia.primary": jobs,
	}, nil
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		gitDir := filepath.Join(cwd, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return cwd, nil
		}
		cwd, err = filepath.Abs(filepath.Join(cwd, ".."))
		if err != nil {
			return "", err
		}
	}
}

func getJobs(repoURL string) {

}
