package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

/*
	Run a specified command on all specified GCE instances.
*/

const (
	ISOLATE_DEFAULT_NAMESPACE = "default-gzip"
	TMP_ISOLATE_FILE_NAME     = "script.isolate"
	TMP_ISOLATE_FILE_CONTENTS = `{
  'variables': {
    'files': [
      '%s',
    ],
  },
}`
)

var (
	dev         = flag.Bool("dev", false, "Run against dev swarming and isolate instances.")
	dimensions  = common.NewMultiStringFlag("dimension", nil, "Colon-separated key/value pair, eg: \"os:Linux\" Dimensions of the bots on which to run. Can specify multiple times.")
	dryRun      = flag.Bool("dry_run", false, "List the bots, don't actually run any tasks")
	includeBots = common.NewMultiStringFlag("include_bot", nil, "Include these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed. Can be simple strings or regexes.")
	excludeBots = common.NewMultiStringFlag("exclude_bot", nil, "Exclude these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed and after --include_bot is applied. Can be simple strings or regexes.")
	internal    = flag.Bool("internal", false, "Run against internal swarming and isolate instances.")
	pool        = flag.String("pool", swarming.DIMENSION_POOL_VALUE_SKIA, "Which Swarming pool to use.")
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

	// validate isolate is on PATH
	if err := exec.Run(context.Background(), &exec.Command{
		Name: "isolate",
		Args: []string{"version"},
	}); err != nil {
		sklog.Fatalf(`isolated not found on PATH. Checkout the README for installation details.`)
	}
	sklog.Info("isolate detected on PATH")

	// validate isolated is on PATH
	if err := exec.Run(context.Background(), &exec.Command{
		Name: "isolated",
		Args: []string{"version"},
	}); err != nil {
		sklog.Fatalf(`isolated not found on PATH. Checkout the README for installation details.`)
	}
	sklog.Info("isolated detected on PATH")

	isolateServer := isolate.ISOLATE_SERVER_URL
	swarmingServer := swarming.SWARMING_SERVER
	if *internal {
		isolateServer = isolate.ISOLATE_SERVER_URL_PRIVATE
		swarmingServer = swarming.SWARMING_SERVER_PRIVATE
	} else if *dev {
		isolateServer = isolate.ISOLATE_SERVER_URL_DEV
		swarmingServer = swarming.SWARMING_SERVER_DEV
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmApi, err := swarming.NewApiClient(httpClient, swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of bots.
	bots, err := swarmApi.ListBots(dims)
	if err != nil {
		sklog.Fatal(err)
	}

	var hashes []string
	if !*dryRun {
		if *script == "" {
			sklog.Fatal("--script is required if not running in dry run mode.")
		}

		// Copy the script to the workdir.
		isolateDir, err := ioutil.TempDir(*workdir, "run_on_swarming_bots")
		if err != nil {
			sklog.Fatal(err)
		}
		defer util.RemoveAll(isolateDir)
		dstScript := path.Join(isolateDir, scriptName)
		if err := util.CopyFile(*script, dstScript); err != nil {
			sklog.Fatal(err)
		}

		// Create an isolate file.
		isolateFile := path.Join(isolateDir, TMP_ISOLATE_FILE_NAME)
		if err := util.WithWriteFile(isolateFile, func(w io.Writer) error {
			_, err := w.Write([]byte(fmt.Sprintf(TMP_ISOLATE_FILE_CONTENTS, scriptName)))
			return err
		}); err != nil {
			sklog.Fatal(err)
		}

		// Upload to isolate server.
		isolateClient, err := isolate.NewClient(*workdir, isolateServer)
		if err != nil {
			sklog.Fatal(err)
		}
		isolateTask := &isolate.Task{
			BaseDir:     isolateDir,
			IsolateFile: isolateFile,
		}
		hashes, _, err = isolateClient.IsolateTasks(ctx, []*isolate.Task{isolateTask})
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
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			dims := []*swarming_api.SwarmingRpcsStringPair{
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
			req := &swarming_api.SwarmingRpcsNewTaskRequest{
				Name:     *taskName,
				Priority: swarming.HIGHEST_PRIORITY,
				TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
					{
						ExpirationSecs: int64((120 * time.Minute).Seconds()),
						Properties: &swarming_api.SwarmingRpcsTaskProperties{
							Caches: []*swarming_api.SwarmingRpcsCacheEntry{
								{
									Name: "vpython",
									Path: "cache/vpython",
								},
							},
							CipdInput:  swarming.ConvertCIPDInput(cipd.PkgsPython),
							Command:    cmd,
							Dimensions: dims,
							EnvPrefixes: []*swarming_api.SwarmingRpcsStringListPair{
								{
									Key:   "PATH",
									Value: []string{"cipd_bin_packages", "cipd_bin_packages/bin"},
								},
								{
									Key:   "VPYTHON_VIRTUALENV_ROOT",
									Value: []string{"cache/vpython"},
								},
							},
							ExecutionTimeoutSecs: int64((120 * time.Minute).Seconds()),
							Idempotent:           false,
							InputsRef: &swarming_api.SwarmingRpcsFilesRef{
								Isolated:       hashes[0],
								Isolatedserver: isolateServer,
								Namespace:      isolate.DEFAULT_NAMESPACE,
							},
							IoTimeoutSecs: int64((120 * time.Minute).Seconds()),
						},
					},
				},
				Tags: tags,
			}
			if _, err := swarmApi.TriggerTask(req); err != nil {
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
