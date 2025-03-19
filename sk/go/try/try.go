package try

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	bbBucketPublic  = "skia/skia.primary"
	bbBucketPrivate = "skia-internal/skia.internal"
)

var (
	// stdin is an abstraction of os.Stdin which is convenient for testing.
	stdin io.Reader = os.Stdin

	// tryjobs is an instance of tryJobReader which may be replaced for testing.
	tryjobs tryJobReader = &tryJobReaderImpl{}

	// gerritProjectToBucket indicates which Gerrit projects are associated with
	// which Buildbucket bucket.
	gerritProjectToBucket = map[string]string{
		// Public projects.
		"buildbot":     bbBucketPublic,
		"common":       bbBucketPublic,
		"libgifcodec":  bbBucketPublic,
		"lottie-ci":    bbBucketPublic,
		"skcms":        bbBucketPublic,
		"skia":         bbBucketPublic,
		"skiabot-test": bbBucketPublic,
		// Private projects.
		"skia-autoroll-internal-config": bbBucketPrivate,
		"skia_internal":                 bbBucketPrivate,
		"eskia":                         bbBucketPrivate,
		"infra-internal":                bbBucketPrivate,
		"internal_test":                 bbBucketPrivate,
		"k8s-config":                    bbBucketPrivate,
	}
)

// Command returns a cli.Command instance which represents the "try" command.
func Command() *cli.Command {
	yFlag := "y"
	bucketFlag := "bucket"
	return &cli.Command{
		Name:        "try",
		Usage:       "try [-y] [job name or regex]...",
		Description: "Run try jobs against the active CL",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  yFlag,
				Value: false,
				Usage: "Trigger all matching try jobs without asking for confirmation.",
			},
			&cli.StringFlag{
				Name:  bucketFlag,
				Value: "",
				Usage: "Override the Buildbucket bucket used to trigger try jobs.",
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := fixupIssue(ctx.Context); err != nil {
				return err
			}
			return try(ctx.Context, ctx.Args().Slice(), ctx.Bool(yFlag), ctx.String(bucketFlag))
		},
	}
}

// try loads the available try jobs, filters by the given request strings, and
// triggers the try jobs selected by the user.
func try(ctx context.Context, jobRequests []string, triggerWithoutPrompt bool, overrideBucket string) error {
	// Setup.
	jobs, err := tryjobs.getTryJobs(ctx)
	if err != nil {
		return err
	}

	// Filter by the given requested job names/regexes.
	filteredJobs := jobs
	if len(jobRequests) > 0 {
		jobRegexes := make([]*regexp.Regexp, 0, len(jobRequests))
		for _, jobRequest := range jobRequests {
			jobRegex, err := regexp.Compile(jobRequest)
			if err != nil {
				return err
			}
			jobRegexes = append(jobRegexes, jobRegex)
		}
		filteredJobs = map[string][]string{}
		for bucket, jobList := range jobs {
			for _, job := range jobList {
				for _, jobRegex := range jobRegexes {
					if jobRegex.MatchString(job) {
						filteredJobs[bucket] = append(filteredJobs[bucket], job)
						break
					}
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
		return skerr.Fmt("Found no jobs matching %v", jobRequests)
	}
	fmt.Println(fmt.Sprintf("Found %d jobs:", count))
	for bucket, jobList := range filteredJobs {
		fmt.Println(fmt.Sprintf("  %s:", bucket))
		for _, job := range jobList {
			fmt.Println(fmt.Sprintf("    %s", job))
		}
	}
	if len(jobRequests) == 0 || count == 0 {
		return nil
	}

	jobsToTrigger := filteredJobs
	if !triggerWithoutPrompt {
		fmt.Printf("Do you want to trigger these jobs? (y/n or i for interactive): ")
		reader := bufio.NewReader(stdin)
		read, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		read = strings.TrimSpace(read)
		if read != "y" && read != "i" {
			return nil
		}
		if read == "i" {
			jobsToTrigger = map[string][]string{}
			for bucket, jobList := range filteredJobs {
				for _, job := range jobList {
					fmt.Printf("Trigger %s? (y/n): ", job)
					trigger, err := reader.ReadString('\n')
					if err != nil {
						return err
					}
					if strings.TrimSpace(trigger) == "y" {
						jobsToTrigger[bucket] = append(jobsToTrigger[bucket], job)
					}
				}
			}
		}
	}

	// Trigger the try jobs.
	for bucket, jobList := range jobsToTrigger {
		if overrideBucket != "" {
			bucket = overrideBucket
		}
		cmd := []string{"git", "cl", "try", "-B", bucket}
		for _, job := range jobList {
			cmd = append(cmd, "-b", job)
		}
		if _, err := exec.RunCwd(ctx, ".", cmd...); err != nil {
			return err
		}
	}
	return nil
}

// fixupIssue ensures that the Gerrit issue is set via "git cl issue",
// regardless of whether "git cl upload" was used to create the change.
func fixupIssue(ctx context.Context) error {
	// If the change was uploaded using Depot Tools, the issue configuration
	// will already be present in the git config.
	output, err := exec.RunCwd(ctx, ".", "git", "branch", "--show-current")
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(output)

	if _, err := exec.RunCwd(ctx, ".", "git", "config", "--local", fmt.Sprintf("branch.%s.gerritissue", branch)); err == nil {
		return nil
	}
	// If the above failed, it's probably because the CL was not uploaded
	// using Depot Tools.  Find the Change-Id line in the most recent commit
	// and obtain the issue number using that.
	output, err = exec.RunCwd(ctx, ".", "git", "log", "-n1", branch)
	if err != nil {
		return err
	}
	changeId, err := gerrit.ParseChangeId(output)
	if err != nil {
		return err
	}
	ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
	if err != nil {
		return err
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	g, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
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

// tryJobReader provides an abstraction for reading the available set of try
// jobs to facilitate testing.
type tryJobReader interface {
	// getTryJobs reads tasks.json from the current repo and returns a
	// map[string][]string of Buildbucket bucket names to try job names.
	getTryJobs(context.Context) (map[string][]string, error)
}

// tryJobReaderImpl is the default tryJobReader implementation which reads from
// the tasks.json file in the current repo.
type tryJobReaderImpl struct{}

// GetTryJobs implements tryJobReader.
func (r *tryJobReaderImpl) getTryJobs(ctx context.Context) (map[string][]string, error) {
	repoRoot, err := repo_root.GetLocal()
	if err != nil {
		return nil, err
	}
	tasksCfg, err := specs.ReadTasksCfg(repoRoot)
	if err != nil {
		return nil, err
	}
	jobs := make([]string, 0, len(tasksCfg.Jobs))
	for name := range tasksCfg.Jobs {
		jobs = append(jobs, name)
	}

	// Attempt to determine which Buildbucket bucket to use by obtaining
	// information about the active CL.
	out, err := exec.RunCwd(ctx, ".", "git", "cl", "issue", "--json=-")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Strip the first line of non-JSON output.
	out = strings.Join(strings.Split(out, "\n")[1:], "\n")
	// Parse the JSON.
	type issueProperties struct {
		GerritProject string `json:"gerrit_project"`
	}
	var props issueProperties
	if err := json.NewDecoder(bytes.NewReader([]byte(out))).Decode(&props); err != nil {
		return nil, skerr.Wrap(err)
	}
	bbBucket, ok := gerritProjectToBucket[props.GerritProject]
	if !ok {
		return nil, skerr.Fmt("Unknown Gerrit project %q", props.GerritProject)
	}
	return map[string][]string{
		bbBucket: jobs,
	}, nil
}
