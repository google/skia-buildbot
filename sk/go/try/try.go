package try

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var tempCheckoutDir = filepath.Join(os.TempDir(), "sktry")

// Command returns a cobra.Command instance which represents the "try" command.
func Command() *cobra.Command {
	env := &tryEnv{}
	cmd := &cobra.Command{
		Use:   "try [job name or regex]",
		Short: "Run try jobs against the active CL",
		Long: `
Run try jobs against the active CL.
`,
		Args: cobra.MaximumNArgs(1),
		Run:  env.runTryCmd,
	}
	cmd.Flags().StringVar(&env.flagJob, "job", "", "Job name or regular expression.")
	cmd.Flags().BoolVar(&env.flagList, "list", false, "List matching jobs and exit.")
	cmd.Flags().BoolVar(&env.flagInternal, "internal", false, "Include internal jobs.")

	return cmd
}

// tryEnv provides the environment for the "try" command.
type tryEnv struct {
	flagJob      string
	flagList     bool
	flagInternal bool
}

func (e *tryEnv) runTryCmd(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	jobRequest := ""
	if len(args) > 0 {
		jobRequest = args[0]
	}
	if err := try(ctx, jobRequest, e.flagList, e.flagInternal); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func try(ctx context.Context, jobRequest string, list, internal bool) error {
	if err := fixupIssue(ctx); err != nil {
		return err
	}
	jobs, err := getTryJobs(ctx)
	if err != nil {
		return err
	}
	// TODO(borenet): Obtain internal if applicable.
	if internal {
		internalJobs, err := getInternalTryJobs(ctx)
		if err != nil {
			return err
		}
		for k, v := range internalJobs {
			jobs[k] = append(jobs[k], v...)
		}
	}

	// Filter by the given requested job name or regex.
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

	// Prompt the user before triggering jobs.
	count := 0
	for _, jobList := range filteredJobs {
		count += len(jobList)
		sort.Strings(jobList)
	}
	if count == 0 {
		return skerr.Fmt("Found no jobs matching %q", jobRequest)
	}
	fmt.Println(fmt.Sprintf("Found %d jobs:", count))
	for bucket, jobList := range filteredJobs {
		fmt.Println(fmt.Sprintf("  %s:", bucket))
		for _, job := range jobList {
			fmt.Println(fmt.Sprintf("    %s", job))
		}
	}
	if list {
		return nil
	}
	if count > 1 {
		fmt.Printf("Do you want to trigger these jobs? (y/n or i for interactive): ")
		read, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		read = strings.TrimSpace(read)
		if read != "y" && read != "i" {
			return nil
		}
		jobsToTrigger := filteredJobs
		if read == "i" {
			jobsToTrigger = map[string][]string{}
			for bucket, jobList := range filteredJobs {
				for _, job := range jobList {
					fmt.Printf("Trigger %s? (y/n): ", job)
					trigger, err := bufio.NewReader(os.Stdin).ReadString('\n')
					if err != nil {
						return err
					}
					if strings.TrimSpace(trigger) == "y" {
						jobsToTrigger[bucket] = append(jobsToTrigger[bucket], job)
					}
				}
			}
		}

		// Trigger the try jobs.
		for bucket, jobList := range jobsToTrigger {
			cmd := []string{"git", "cl", "try", "-B", bucket}
			for _, job := range jobList {
				cmd = append(cmd, "-b", job)
			}
			if _, err := exec.RunCwd(ctx, ".", cmd...); err != nil {
				return err
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
		"skia/skia.primary": jobs,
	}, nil
}

func getInternalTryJobs(ctx context.Context) (map[string][]string, error) {
	if _, err := os.Stat(tempCheckoutDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tempCheckoutDir, os.ModePerm); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	repo := common.REPO_SKIA_INTERNAL_TEST
	normRepo, err := git.NormalizeURL(repo)
	if err != nil {
		return nil, err
	}
	split := strings.Split(normRepo, "/")
	dirName := split[len(split)-1]
	repoDir := filepath.Join(tempCheckoutDir, dirName)
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		if _, err := exec.RunCwd(ctx, tempCheckoutDir, "git", "clone", "--mirror", repo, dirName); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if _, err := exec.RunCwd(ctx, repoDir, "git", "remote", "update"); err != nil {
		return nil, err
	}
	output, err := exec.RunCwd(ctx, repoDir, "git", "show", fmt.Sprintf("master:%s", specs.TASKS_CFG_FILE))
	if err != nil {
		return nil, err
	}
	tasksCfg, err := specs.ParseTasksCfg(string(output))
	if err != nil {
		return nil, err
	}
	jobs := make([]string, 0, len(tasksCfg.Jobs))
	for job := range tasksCfg.Jobs {
		jobs = append(jobs, job)
	}
	return map[string][]string{
		"skia-internal/skia.internal": jobs,
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
