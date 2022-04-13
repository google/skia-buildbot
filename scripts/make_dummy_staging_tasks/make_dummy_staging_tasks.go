package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
	"golang.org/x/oauth2/google"
)

const (
	BOT_GROUP_TMPL   = "bot-group:%s"
	BOT_NAME_TMPL    = "skia-d-gce-%s"
	BOT_SECTION_TMPL = `bot_group {
  bot_id: "%s"

  owners: "skiabot@google.com"

  auth {
    require_service_account: "chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com"
  }
  system_service_account: "pool-skia@chromium-swarm-dev.iam.gserviceaccount.com"

  dimensions: "pool:Skia"
%s
  bot_config_script: "skia.py"
}
`
)

var (
	from       = flag.String("from", "", "Root dir of source repo.")
	to         = flag.String("to", "", "Root dir of destination repo.")
	botsCfg    = flag.String("bots_cfg", "", "Name of file to write partial bot config data.")
	fsInstance = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"production\"")
	now        = flag.Int("now", int(time.Now().Unix()), "Current timestamp; use to make this script reproducible.")

	dimensions = []string{
		"pool:Skia",
	}
)

func main() {
	common.Init()

	nowTs := time.Unix(int64(*now), 0)

	includeDimensions := map[string]bool{}

	ctx := context.Background()
	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}

	var cfgB *specs.TasksCfg
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Obtain average task durations for the last 5 days.
		db, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks, err := db.GetTasksFromDateRange(ctx, nowTs.Add(-5*24*time.Hour), nowTs, common.REPO_SKIA)
		if err != nil {
			sklog.Fatal(err)
		}
		durations := map[string][]time.Duration{}
		for _, task := range tasks {
			if task.Status != types.TASK_STATUS_SUCCESS {
				continue
			}
			duration := task.Finished.Sub(task.Started)
			if duration < time.Duration(0) {
				continue
			}
			durations[task.Name] = append(durations[task.Name], duration)
		}
		avgDurations := make(map[string]time.Duration, len(durations))
		for name, durs := range durations {
			total := time.Duration(0)
			for _, d := range durs {
				total += d
			}
			avgDurations[name] = total / time.Duration(len(durs))
		}

		cfgA, err := specs.ReadTasksCfg(*from)
		if err != nil {
			sklog.Fatal(err)
		}

		cfgB = &specs.TasksCfg{
			Jobs:  make(map[string]*specs.JobSpec, len(cfgA.Jobs)),
			Tasks: make(map[string]*specs.TaskSpec, len(cfgA.Tasks)),
			CasSpecs: map[string]*specs.CasSpec{
				"infrabots": {
					Paths: []string{"infra/bots"},
					Root:  ".",
				},
			},
		}
		for name, jobSpec := range cfgA.Jobs {
			// Leave the JobSpecs the same.
			cfgB.Jobs[name] = jobSpec
		}

		taskNames := make([]string, 0, len(cfgA.Tasks))
		for name := range cfgA.Tasks {
			taskNames = append(taskNames, name)
		}
		sort.Strings(taskNames)
		for _, name := range taskNames {
			taskSpec := cfgA.Tasks[name]
			taskSpec.Caches = nil
			taskSpec.CipdPackages = nil
			avgDuration := int64(avgDurations[name].Seconds())
			if avgDuration == 0 {
				sklog.Errorf("No average duration for %s!", name)
				avgDuration = 10
			}
			taskSpec.Command = []string{"/bin/bash", "dummy.sh", fmt.Sprintf("%d", avgDuration)}
			if len(taskSpec.Outputs) > 0 {
				taskSpec.Command = append(taskSpec.Command, taskSpec.Outputs...)
			} else {
				taskSpec.Command = append(taskSpec.Command, "${ISOLATED_OUTDIR}")
			}

			for _, dim := range taskSpec.Dimensions {
				split := strings.SplitN(dim, ":", 2)
				if len(split) != 2 {
					sklog.Fatalf("Invalid dimension: %s", dim)
				}
				includeDimensions[split[0]] = true
			}

			taskSpec.EnvPrefixes = nil
			taskSpec.ExtraTags = nil
			taskSpec.CasSpec = "infrabots"
			taskSpec.ServiceAccount = ""

			cfgB.Tasks[name] = taskSpec
		}
	}()

	// Set up Swarming client.
	swarmClient := httputils.DefaultClientConfig().WithTokenSource(ts).WithDialTimeout(3 * time.Minute).With2xxOnly().Client()
	swarm, err := swarming.NewApiClient(swarmClient, "chromium-swarm.appspot.com")
	if err != nil {
		sklog.Fatal(err)
	}
	bots, err := swarm.ListBotsForPool(ctx, "Skia")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Found %d bots", len(bots))
	wg.Wait()

	// botGroups maps dimension sets to bot group IDs. The dimension set is
	// just a concatenation of bot dimensions.
	botGroups := map[string]string{}

	// botGroupIds simply lists the bot group IDs.
	botGroupIds := []string{}

	// numBots maps bot group IDs to the number of bots in each group.
	numBots := map[string]int{}

	// dimsByGroup maps a bot group ID (derived from dimensions) to a
	// 2-level map of the dimensions themselves: map[key]map[value]true.
	// This makes it easy to check whether a bot group can run a given task.
	dimsByGroup := map[string]map[string]map[string]bool{}

	sort.Slice(bots, func(i, j int) bool { return bots[i].BotId < bots[j].BotId })
	for _, bot := range bots {
		// botDims maps dimension keys to dimension values, using a
		// sub-map so that we can check for the existence of specific
		// values.
		botDims := map[string]map[string]bool{}

		subKeys := []string{}
		for _, dim := range bot.Dimensions {
			if includeDimensions[dim.Key] {
				vals := make([]string, 0, len(dim.Value))
				valsMap := make(map[string]bool, len(dim.Value))
				for _, val := range dim.Value {
					vals = append(vals, val)
					valsMap[val] = true
				}
				sort.Strings(vals)
				subKeys = append(subKeys, fmt.Sprintf("%s:%s", dim.Key, strings.Join(vals, ",")))
				botDims[dim.Key] = valsMap
			}
		}
		sort.Strings(subKeys)
		dimSetKey := strings.Join(subKeys, ";")
		groupId, ok := botGroups[dimSetKey]
		if !ok {
			groupId = fmt.Sprintf("%03d", len(botGroups))
			botGroups[dimSetKey] = groupId
			botGroupIds = append(botGroupIds, groupId)
			b, err := json.MarshalIndent(botDims, "", "  ")
			if err != nil {
				sklog.Fatal(err)
			}
			sklog.Infof("Group %s:\n%s", groupId, b)
		}
		numBots[groupId]++
		dimsByGroup[groupId] = botDims
	}
	sklog.Infof("Found %d sets of bots with shared dimensions.", len(botGroups))

	// Now, match the task specs up to bot groups.
	used := map[string]bool{}
	for name, t := range cfgB.Tasks {
		groups := []string{}
		for _, groupId := range botGroupIds {
			botDims := dimsByGroup[groupId]
			canHandle := true
			for _, dim := range t.Dimensions {
				split := strings.SplitN(dim, ":", 2)
				if len(split) != 2 {
					sklog.Fatalf("Invalid dimension: %s", dim)
				}
				if vals, ok := botDims[split[0]]; !ok || !vals[split[1]] {
					canHandle = false
					break
				}
			}
			if canHandle {
				groups = append(groups, groupId)
			}
		}
		// We don't know how to specify that a dimension could be one of
		// two different values, so we can't specify the "bot-group"
		// dimension to mean one of a set of groups. Just pick the first.
		if len(groups) == 0 {
			sklog.Errorf("No bots can run %s", name)
		} else {
			t.Dimensions = append(dimensions, fmt.Sprintf(BOT_GROUP_TMPL, groups[0]))
			used[groups[0]] = true
			if len(groups) > 1 {
				sklog.Infof("Have %d groups but chose %s; %v", len(groups), groups[0], groups)
			}
		}
	}
	if err := specs.WriteTasksCfg(cfgB, *to); err != nil {
		sklog.Fatal(err)
	}

	// Create sets of new bots with the dimension sets from above.
	botIdStart := 100 // To avoid issues with zero-padding.
	rangeStart := botIdStart
	botCfgData := ""
	for _, groupId := range botGroupIds {
		if !used[groupId] {
			sklog.Infof("Unused group: %s", groupId)
			continue
		}
		dimensions := fmt.Sprintf("  dimensions: \"%s\"\n", fmt.Sprintf(BOT_GROUP_TMPL, groupId))
		n := numBots[groupId]
		rangeStr := fmt.Sprintf("{%03d..%03d}", rangeStart, rangeStart+n-1)
		if len(bots) == 1 {
			rangeStr = fmt.Sprintf("%03d", rangeStart)
		}
		botSection := fmt.Sprintf(BOT_SECTION_TMPL, fmt.Sprintf(BOT_NAME_TMPL, rangeStr), dimensions)
		botCfgData += botSection
		rangeStart += n
	}
	if err := ioutil.WriteFile(*botsCfg, []byte(botCfgData), os.ModePerm); err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Create bots with:\n$ go run ./go/gce/swarming/swarming_vm.go --dev --create --type=linux-micro --instances=%d-%d", botIdStart, rangeStart-1)
}
