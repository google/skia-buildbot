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
	"slices"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"golang.org/x/oauth2/google"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
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

	gerritURL = "https://skia-review.googlesource.com"

	tsJobURLPublic  = "https://task-scheduler.skia.org/job/%s"
	tsJobURLPrivate = "https://skia-task-scheduler.corp.goog/job/%s"
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

	// bbBucketToTaskSchedulerURL maps buildbucket buckets to a task scheduler
	// job URL.
	bbBucketToTaskSchedulerURL = map[string]string{
		bbBucketPublic:  tsJobURLPublic,
		bbBucketPrivate: tsJobURLPrivate,
	}
)

// Command returns a cli.Command instance which represents the "try" command.
func Command() []*cli.Command {
	yFlag := "y"
	bucketFlag := "bucket"
	issueFlag := "issue"
	patchSetFlag := "patchset"
	return []*cli.Command{
		{
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
					return skerr.Wrap(err)
				}
				return try(ctx.Context, ctx.Args().Slice(), ctx.Bool(yFlag), ctx.String(bucketFlag))
			},
		},
		{
			Name:        "try-results",
			Usage:       "try-results",
			Description: "Retrieve try jobs for the active CL",
			Flags: []cli.Flag{
				&cli.Int64Flag{
					Name:  issueFlag,
					Value: 0,
					Usage: "Retrieve try jobs for this issue instead of the one for the current branch.",
				},
				&cli.Int64Flag{
					Name:  patchSetFlag,
					Value: 0,
					Usage: "Retrieve try jobs for this patch set instead of the most recent.",
				},
			},
			Action: func(ctx *cli.Context) error {
				return tryResults(ctx.Context, ctx.Int64(issueFlag), ctx.Int64(patchSetFlag))
			},
		},
	}
}

// try loads the available try jobs, filters by the given request strings, and
// triggers the try jobs selected by the user.
func try(ctx context.Context, jobRequests []string, triggerWithoutPrompt bool, overrideBucket string) error {
	// Setup.
	jobs, err := tryjobs.getTryJobs(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Filter by the given requested job names/regexes.
	filteredJobs := jobs
	if len(jobRequests) > 0 {
		jobRegexes := make([]*regexp.Regexp, 0, len(jobRequests))
		for _, jobRequest := range jobRequests {
			jobRegex, err := regexp.Compile(jobRequest)
			if err != nil {
				return skerr.Wrap(err)
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
	fmt.Printf("Found %d jobs:\n", count)
	for bucket, jobList := range filteredJobs {
		fmt.Printf("  %s:\n", bucket)
		for _, job := range jobList {
			fmt.Printf("    %s\n", job)
		}
	}
	if len(jobRequests) == 0 || count == 0 {
		return nil
	}

	jobsToTrigger := filteredJobs
	if !triggerWithoutPrompt {
		fmt.Println("Do you want to trigger these jobs? (y/n or i for interactive): ")
		reader := bufio.NewReader(stdin)
		read, err := reader.ReadString('\n')
		if err != nil {
			return skerr.Wrap(err)
		}
		read = strings.TrimSpace(read)
		if read != "y" && read != "i" {
			return nil
		}
		if read == "i" {
			jobsToTrigger = map[string][]string{}
			for bucket, jobList := range filteredJobs {
				for _, job := range jobList {
					fmt.Printf("Trigger %s? (y/n):\n", job)
					trigger, err := reader.ReadString('\n')
					if err != nil {
						return skerr.Wrap(err)
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
			return skerr.Wrap(err)
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
		return skerr.Wrap(err)
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
		return skerr.Wrap(err)
	}
	changeId, err := gerrit.ParseChangeId(output)
	if err != nil {
		return skerr.Wrap(err)
	}
	ts, err := google.DefaultTokenSource(ctx, gerrit.AuthScope)
	if err != nil {
		return skerr.Wrap(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	g, err := gerrit.NewGerrit(gerrit.GerritSkiaURL, client)
	if err != nil {
		return skerr.Wrap(err)
	}
	ci, err := g.GetChange(ctx, changeId)
	if err != nil {
		return skerr.Wrap(err)
	}
	issue := fmt.Sprintf("%d", ci.Issue)
	if _, err := exec.RunCwd(ctx, ".", "git", "cl", "issue", issue); err != nil {
		return skerr.Wrap(err)
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
	props, err := getLocalIssueProperties(ctx)
	if err != nil {
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

type issueProperties struct {
	GerritHost    string `json:"gerrit_host"`
	GerritProject string `json:"gerrit_project"`
	IssueURL      string `json:"issue_url"`
	Issue         int64  `json:"issue"`
}

func getLocalIssueProperties(ctx context.Context) (*issueProperties, error) {
	out, err := exec.RunCwd(ctx, ".", "git", "cl", "issue", "--json=-")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// Strip the first line of non-JSON output.
	out = strings.Join(strings.Split(out, "\n")[1:], "\n")
	// Parse the JSON.
	var props issueProperties
	if err := json.NewDecoder(bytes.NewReader([]byte(out))).Decode(&props); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &props, nil
}

func tryResults(ctx context.Context, issue, patchset int64) error {
	// Set up HTTP client.
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()

	// Find the issue ID if not provided.
	if issue == 0 {
		if err := fixupIssue(ctx); err != nil {
			return skerr.Wrap(err)
		}
		props, err := getLocalIssueProperties(ctx)
		if err != nil {
			return skerr.Wrap(err)
		}
		issue = props.Issue
	}

	// Find the patchset ID if not provided.
	if patchset == 0 {
		g, err := gerrit.NewGerrit(gerritURL, httpClient)
		if err != nil {
			return skerr.Wrap(err)
		}
		ci, err := g.GetIssueProperties(ctx, issue)
		if err != nil {
			return skerr.Wrap(err)
		}
		patchsets := ci.GetPatchsetIDs()
		patchset = patchsets[len(patchsets)-1]
	}

	// Retrieve Buildbucket builds for the issue+patchset.
	bbClient := buildbucket.NewClient(httpClient)
	builds, err := bbClient.GetTrybotsForCL(ctx, issue, patchset, gerritURL, nil)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Group by status.
	buildsByStatus := map[buildbucketpb.Status][]*buildbucketpb.Build{}
	for _, build := range builds {
		buildsByStatus[build.Status] = append(buildsByStatus[build.Status], build)
	}
	statuses := make([]buildbucketpb.Status, 0, len(buildsByStatus))
	for status := range buildsByStatus {
		statuses = append(statuses, status)
	}
	slices.Sort(statuses)

	// Print results.
	fmt.Printf("Patchset %d:\n", patchset)
	for _, s := range statuses {
		colorLogf(s, "  %s:\n", s.String())
		for _, build := range builds {
			colorLogf(s, "    %s: %s\n", build.Builder.Builder, jobURLForBuild(build))
		}
	}
	return nil
}

func colorLogf(status buildbucketpb.Status, format string, a ...interface{}) {
	_, _ = getColor(status).Printf(format, a...)
}

var statusToColor = map[buildbucketpb.Status]*color.Color{
	buildbucketpb.Status_CANCELED:           color.New(color.FgCyan),
	buildbucketpb.Status_FAILURE:            color.New(color.FgRed),
	buildbucketpb.Status_STARTED:            color.New(color.FgYellow),
	buildbucketpb.Status_SUCCESS:            color.New(color.FgGreen),
	buildbucketpb.Status_STATUS_UNSPECIFIED: color.New(color.Reset),
}

func getColor(status buildbucketpb.Status) *color.Color {
	color, ok := statusToColor[status]
	if !ok {
		color = statusToColor[buildbucketpb.Status_STATUS_UNSPECIFIED]
	}
	return color
}

func jobURLForBuild(build *buildbucketpb.Build) string {
	var jobID string
	if build.Infra != nil && build.Infra.Backend != nil && build.Infra.Backend.Task != nil && build.Infra.Backend.Task.Id != nil {
		jobID = build.Infra.Backend.Task.Id.Id
	}
	if jobID == "" {
		return "(not started yet)"
	}
	bucket := fmt.Sprintf("%s/%s", build.Builder.Project, build.Builder.Bucket)
	tsURL := bbBucketToTaskSchedulerURL[bucket]
	return fmt.Sprintf(tsURL, jobID)
}
