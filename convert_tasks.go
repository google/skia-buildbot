package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	BOT_GROUP_TMPL   = "bot-group:%s"
	BOT_NAME_TMPL    = "skia-d-gce-%s"
	BOT_SECTION_TMPL = `bot_group {
  bot_id_prefix: "%s"

  owners: "skiabot@google.com"

  auth {
    require_service_account: "chromium-swarm-bots@skia-swarming-bots.iam.gserviceaccount.com"
  }
  system_service_account: "pool-skia@chromium-swarm-dev.iam.gserviceaccount.com"

  dimensions: "pool:Skia"
%s
  bot_config_script: "skia.py"
}`
)

var (
	from = flag.String("from", "", "Root dir of source repo.")
	to   = flag.String("to", "", "Root dir of destination repo.")

	dimensions = []string{
		"pool:Skia",
	}
)

func main() {
	common.Init()

	now := time.Unix(1547578835, 0)

	dimSets := map[string]string{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Obtain average task durations for the last 24 hours.
		db, err := local_db.NewDB(local_db.DB_NAME, "/tmp/task-scheduler.bdb", nil)
		if err != nil {
			sklog.Fatal(err)
		}
		tasks, err := db.GetTasksFromDateRange(now.Add(-5*24*time.Hour), now, common.REPO_SKIA)
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

		cfgB := &specs.TasksCfg{
			Jobs:  make(map[string]*specs.JobSpec, len(cfgA.Jobs)),
			Tasks: make(map[string]*specs.TaskSpec, len(cfgA.Tasks)),
		}
		for name, jobSpec := range cfgA.Jobs {
			// Leave the JobSpecs the same.
			cfgB.Jobs[name] = jobSpec
		}

		dimSetNum := 0
		taskNames := make([]string, 0, len(cfgA.Tasks))
		for name, _ := range cfgA.Tasks {
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
			taskSpec.Command = []string{"infra/bots/dummy.sh", fmt.Sprintf("%d", avgDuration)}
			if len(taskSpec.Outputs) > 0 {
				taskSpec.Command = append(taskSpec.Command, taskSpec.Outputs...)
			}

			sort.Strings(taskSpec.Dimensions)
			dimSetKey := strings.Join(taskSpec.Dimensions, "\n")
			botGroup := fmt.Sprintf("%03d", dimSetNum)
			if _, ok := dimSets[dimSetKey]; !ok {
				dimSetNum++
				dimSets[dimSetKey] = botGroup
			}
			taskSpec.Dimensions = append(dimensions, fmt.Sprintf(BOT_GROUP_TMPL, botGroup))

			taskSpec.EnvPrefixes = nil
			taskSpec.ExtraTags = nil
			taskSpec.Isolate = "infrabots.isolate"
			taskSpec.ServiceAccount = ""

			cfgB.Tasks[name] = taskSpec
		}
		if err := specs.WriteTasksCfg(cfgB, *to); err != nil {
			sklog.Fatal(err)
		}
	}()

	// Set up Swarming client.
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmClient := httputils.DefaultClientConfig().WithTokenSource(ts).WithDialTimeout(3 * time.Minute).With2xxOnly().Client()
	swarm, err := swarming.NewApiClient(swarmClient, "chromium-swarm.appspot.com")
	if err != nil {
		sklog.Fatal(err)
	}
	bots, err := swarm.ListBotsForPool("Skia")
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Found %d bots", len(bots))
	wg.Wait()

	// Determine which dimension set numbers can be handled by each bot.
	canHandle := map[string][]string{}
	for _, bot := range bots {
		botDims := map[string]map[string]bool{}
		for _, dim := range bot.Dimensions {
			vals := map[string]bool{}
			for _, val := range dim.Value {
				vals[val] = true
			}
			botDims[dim.Key] = vals
		}
		sets := []string{}
		for dimString, setNum := range dimSets {
			match := true
			for _, dim := range strings.Split(dimString, "\n") {
				split := strings.SplitN(dim, ":", 2)
				if len(split) != 2 {
					sklog.Fatalf("Invalid dimension: %s", dim)
				}
				key := split[0]
				val := split[1]
				vals, ok := botDims[key]
				if !ok {
					match = false
					break
				}
				if !vals[val] {
					match = false
					break
				}
			}
			if match {
				sets = append(sets, setNum)
			}
		}
		if len(sets) > 0 {
			sort.Strings(sets)
			setKey := strings.Join(sets, ",")
			canHandle[setKey] = append(canHandle[setKey], bot.BotId)
		}
	}

	// Create sets of new bots with the dimension sets from above.
	setKeys := []string{}
	for key, _ := range canHandle {
		setKeys = append(setKeys, key)
	}
	sort.Strings(setKeys)
	numBots := 0
	for _, setKey := range setKeys {
		bots := canHandle[setKey]
		dimensions := ""
		for _, dimSet := range strings.Split(setKey, ",") {
			dimensions += fmt.Sprintf("  dimensions: \"%s\"\n", fmt.Sprintf(BOT_GROUP_TMPL, dimSet))
		}
		rangeStr := fmt.Sprintf("{%03d..%03d}", numBots, numBots+len(bots)-1)
		if len(bots) == 1 {
			rangeStr = fmt.Sprintf("%03d", numBots)
		}
		botSection := fmt.Sprintf(BOT_SECTION_TMPL, fmt.Sprintf(BOT_NAME_TMPL, rangeStr), dimensions)
		fmt.Println(botSection)
		numBots += len(bots)
	}

}
