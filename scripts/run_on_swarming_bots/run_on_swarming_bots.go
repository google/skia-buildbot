package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
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

	var casInput *swarming_api.SwarmingRpcsCASReference
	if !*dryRun {
		if *script == "" {
			sklog.Fatal("--script is required if not running in dry run mode.")
		}

		// Copy the script to the workdir.
		casRoot, err := ioutil.TempDir(*workdir, "run_on_swarming_bots")
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
		casInput, err = swarming.MakeCASReference(digest, casInstance)
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
							CasInputRoot: casInput,
							CipdInput:    cipdInput,
							Command:      cmd,
							Dimensions:   dims,
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
							IoTimeoutSecs:        int64((120 * time.Minute).Seconds()),
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

func getPythonCIPDPackages(bot *swarming_api.SwarmingRpcsBotInfo) *swarming_api.SwarmingRpcsCipdInput {
	var os string
	var arch string
	for _, dim := range bot.Dimensions {
		if dim.Key == "os" {
			for _, value := range dim.Value {
				if strings.Contains(value, "iOS") || strings.Contains(value, "Android") || strings.Contains(value, "ChromeOS") || strings.Contains(value, "Raspbian") {
					// iOS, Android, and ChromeOS devices are hosted on RPis. We
					// don't have a 32-bit ARM binary for Python in CIPD, so
					// just use what's installed on the machine.
					return nil
				} else if strings.Contains(value, "Mac") {
					os = "Mac"
					break
				} else if strings.Contains(value, "Linux") {
					os = "Linux"
					break
				} else if strings.Contains(value, "Windows") {
					os = "Windows"
					break
				}
			}
		} else if dim.Key == "cpu" {
			for _, value := range dim.Value {
				if strings.Contains(value, "arm64") {
					arch = "arm64"
					break
				} else if strings.Contains(value, "x86-64") {
					arch = "x86-64"
					// Don't break, in case a bot has both "x86" and
					// "x86-64" dimensions.
				} else if strings.Contains(value, "x86-32") {
					arch = "386"
					// Don't break, in case a bot has both "x86" and
					// "x86-64" dimensions.
				}
			}
		}
	}
	var platform string
	if os == "Linux" {
		if arch == "arm64" {
			platform = cipd.PlatformLinuxArm64
		} else if arch == "x86-64" {
			platform = cipd.PlatformLinuxAmd64
		}
	} else if os == "Windows" {
		if arch == "386" {
			platform = cipd.PlatformWindows386
		} else {
			platform = cipd.PlatformWindowsAmd64
		}
	} else if os == "Mac" {
		platform = cipd.PlatformMacAmd64
	}
	var cipdInput *swarming_api.SwarmingRpcsCipdInput
	if platform != "" {
		cipdInput = swarming.ConvertCIPDInput(cipd.PkgsPython[platform])
	}
	return cipdInput
}
