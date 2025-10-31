package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/go/util"
)

/*
	Run a specified command on all specified GCE instances.
*/

var (
	dev         = flag.Bool("dev", false, "Run against dev swarming instance.")
	dimensions  = common.NewMultiStringFlag("dimension", nil, "Colon-separated key/value pair, eg: \"os:Linux\" Dimensions of the bots on which to run. Can specify multiple times.")
	dryRun      = flag.Bool("dry_run", false, "List the bots, don't actually run any tasks")
	includeBots = common.NewMultiStringFlag("include_bot", nil, "Include these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed. Can be simple strings or regexes.")
	excludeBots = common.NewMultiStringFlag("exclude_bot", nil, "Exclude these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed and after --include_bot is applied. Can be simple strings or regexes.")
	internal    = flag.Bool("internal", false, "Run against internal swarming instance.")
	pool        = flag.String("pool", swarming.DIMENSION_POOL_VALUE_SKIA, "Which Swarming pool to use.")
	rerun       = flag.String("rerun", "", "Label from a previous invocation of this script, used to re-run all non-successful tasks.")
	script      = flag.String("script", "", "Path to a Python script to run.")
	taskName    = flag.String("task_name", "", "Name of the task to run.")
	workdir     = flag.String("workdir", os.TempDir(), "Working directory. Optional, but recommended not to use CWD.")
)

func main() {
	// Setup, parse args.
	common.Init()

	ctx := context.Background()
	if *internal && *dev {
		sklog.Fatal("Both --internal and --dev cannot be specified.")
	}
	scriptName := path.Base(*script)
	if *taskName == "" {
		*taskName = fmt.Sprintf("run_%s", scriptName)
	}

	dims, err := swarming.ParseDimensionsSingleValue(*dimensions)
	if err != nil {
		sklog.Fatalf("Problem parsing dimensions: %s", err)
	}
	dims["pool"] = *pool

	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	includeRegs, err := parseRegex(*includeBots)
	if err != nil {
		sklog.Fatal(err)
	}
	excludeRegs, err := parseRegex(*excludeBots)
	if err != nil {
		sklog.Fatal(err)
	}

	casInstance := rbe.InstanceChromiumSwarm
	swarmingServer := swarming.SWARMING_SERVER
	if *internal {
		casInstance = rbe.InstanceChromeSwarming
		swarmingServer = swarming.SWARMING_SERVER_PRIVATE
	} else if *dev {
		casInstance = rbe.InstanceChromiumSwarmDev
		swarmingServer = swarming.SWARMING_SERVER_DEV
	}

	// Authenticated HTTP client.
	ts, err := google.DefaultTokenSource(ctx, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmApi := swarmingv2.NewDefaultClient(httpClient, swarmingServer)

	// Are we re-running a previous set of tasks?
	if *rerun != "" {
		rerunPrevious(ctx, swarmingServer, swarmApi, *rerun)
		return
	}

	// Obtain the list of bots.
	bots, err := swarmingv2.ListBotsHelper(ctx, swarmApi, &apipb.BotsRequest{
		Dimensions: swarmingv2.StringMapToTaskDimensions(dims),
	})
	if err != nil {
		sklog.Fatal(err)
	}

	var casInput *apipb.CASReference
	if !*dryRun {
		if *script == "" {
			sklog.Fatal("--script is required if not running in dry run mode.")
		}

		// Copy the script to the workdir.
		casRoot, err := os.MkdirTemp(*workdir, "run_on_swarming_bots")
		if err != nil {
			sklog.Fatal(err)
		}
		defer util.RemoveAll(casRoot)
		dstScript := path.Join(casRoot, scriptName)
		if err := util.CopyFile(*script, dstScript); err != nil {
			sklog.Fatal(err)
		}

		// Upload to CAS.
		casClient, err := rbe.NewClient(ctx, casInstance, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		digest, err := casClient.Upload(ctx, casRoot, []string{scriptName}, nil)
		if err != nil {
			sklog.Fatal(err)
		}
		casInput, err = swarmingv2.MakeCASReference(digest, casInstance)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Trigger the task on each bot.
	cmd := []string{"python", "-u", scriptName}
	group := fmt.Sprintf("%s_%s", *taskName, uuid.New())
	tags := []string{
		fmt.Sprintf("group:%s", group),
	}
	if *dryRun {
		sklog.Info("Dry run mode.  Would run on following bots:")
	}
	var wg sync.WaitGroup
	for _, bot := range bots {
		if len(includeRegs) > 0 && !matchesAny(bot.BotId, includeRegs) {
			sklog.Debugf("Skipping %s because it does not match --include_bot", bot.BotId)
			continue
		}
		if matchesAny(bot.BotId, excludeRegs) {
			sklog.Debugf("Skipping %s because it matches --exclude_bot", bot.BotId)
			continue
		}
		if *dryRun {
			sklog.Info(bot.BotId)
			continue
		}
		cipdInput := getPythonCIPDPackages(bot)

		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			dims := []*apipb.StringPair{
				{
					Key:   "pool",
					Value: *pool,
				},
				{
					Key:   "id",
					Value: id,
				},
			}

			sklog.Infof("Triggering on %s", id)
			req := &apipb.NewTaskRequest{
				Name:     *taskName,
				Priority: swarming.HIGHEST_PRIORITY,
				TaskSlices: []*apipb.TaskSlice{
					{
						ExpirationSecs: int32((12 * time.Hour).Seconds()),
						Properties: &apipb.TaskProperties{
							Caches: []*apipb.CacheEntry{
								{
									Name: "vpython",
									Path: "cache/vpython",
								},
							},
							CasInputRoot: casInput,
							CipdInput:    cipdInput,
							Command:      cmd,
							Dimensions:   dims,
							EnvPrefixes: []*apipb.StringListPair{
								{
									Key:   "PATH",
									Value: []string{"cipd_bin_packages", "cipd_bin_packages/bin"},
								},
								{
									Key:   "VPYTHON_VIRTUALENV_ROOT",
									Value: []string{"cache/vpython"},
								},
							},
							ExecutionTimeoutSecs: int32((120 * time.Minute).Seconds()),
							Idempotent:           false,
							IoTimeoutSecs:        int32((120 * time.Minute).Seconds()),
						},
					},
				},
				Tags: tags,
			}
			if _, err := swarmApi.NewTask(ctx, req); err != nil {
				sklog.Fatal(err)
			}
		}(bot.BotId)
	}

	wg.Wait()
	if !*dryRun {
		tasksLink := fmt.Sprintf("https://%s/tasklist?f=group:%s", swarmingServer, group)
		sklog.Infof("Triggered Swarming tasks. Visit this link to track progress:\n%s", tasksLink)
	}
}

func parseRegex(flags []string) (retval []*regexp.Regexp, e error) {
	for _, s := range flags {
		r, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		retval = append(retval, r)
	}
	return retval, nil
}

func matchesAny(s string, xr []*regexp.Regexp) bool {
	for _, r := range xr {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}

func getPythonCIPDPackages(bot *apipb.BotInfo) *apipb.CipdInput {
	return swarmingv2.ConvertCIPDInput(cipd.PkgsPython)
}

func rerunPrevious(ctx context.Context, swarmingServer string, swarmApi swarmingv2.SwarmingV2Client, rerun string) {
	results, err := swarmingv2.ListTasksHelper(ctx, swarmApi, &apipb.TasksWithPerfRequest{
		State: apipb.StateQuery_QUERY_ALL,
		Tags:  []string{rerun},
	})
	if err != nil {
		sklog.Fatal(err)
	}
	newTag := rerun + "_rerun"
	var g errgroup.Group
	for _, result := range results {
		result := result // https://golang.org/doc/faq#closures_and_goroutines
		if result.State == apipb.TaskState_COMPLETED && !result.Failure && !result.InternalFailure {
			continue
		}
		g.Go(func() error {
			taskMeta, err := swarmApi.GetRequest(ctx, &apipb.TaskIdRequest{
				TaskId: result.TaskId,
			})
			if err != nil {
				return err
			}
			taskMeta.Tags = append(taskMeta.Tags, newTag)
			_, err = swarmingv2.RetryTask(ctx, swarmApi, taskMeta)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		sklog.Fatal(err)
	}
	tasksLink := fmt.Sprintf("https://%s/tasklist?f=%s", swarmingServer, newTag)
	sklog.Infof("Triggered Swarming tasks. Visit this link to track progress:\n%s", tasksLink)
}
