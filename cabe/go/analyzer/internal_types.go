package analyzer

// This file contains struct types and helper functions for manipulating
// in-memory representations of experiment measurement task metadata.
import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/perf/go/perfresults"
)

// helper function for extracting buildInfo from swarming tasks.
func assignIfHasPrefix(prefix, source string, dest *string) {
	if strings.HasPrefix(source, prefix) {
		*dest = source[len(prefix):]
	}
}

// helper function for extracting buildInfo from swarming tasks.
func appendIfHasPrefix(prefix, source string, dest *[]string) {
	if strings.HasPrefix(source, prefix) {
		*dest = append(*dest, source[len(prefix):])
	}
}

// Pinpoint will set a "change:..." tag on the swarming tasks it runs for each arm.
// The contents are formatted here: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/change/change.py;l=52
// however we only care about the presence of the "change:" prefix in this function. The rest
// (if anything) is handled by the caller.
func pinpointChangeTagForTask(t *apipb.TaskRequestMetadataResponse) string {
	for _, tag := range t.TaskResult.Tags {
		if strings.HasPrefix(tag, "change:") {
			return tag[len("change:"):]
		}
	}
	return ""
}

// Request dimensions are key value pairs, and keys can appear more that once.  This function
// groups values by key.
func requestDimensionsForTask(s *apipb.TaskRequestMetadataResponse) map[string][]string {
	ret := make(map[string][]string)
	for _, dim := range s.Request.Properties.Dimensions {
		v := ret[dim.Key]
		if v == nil {
			v = []string{}
			ret[dim.Key] = v
		}
		ret[dim.Key] = append(v, dim.Value)
	}
	return ret
}

func resultBotDimensionsForTask(s *apipb.TaskRequestMetadataResponse) map[string][]string {
	ret := make(map[string][]string)
	for _, dim := range s.TaskResult.BotDimensions {
		v := ret[dim.Key]
		if v == nil {
			v = []string{}
			ret[dim.Key] = v
		}
		ret[dim.Key] = append(v, dim.Value...)
	}
	return ret
}

// A build task has a tag like "buildbucket_build_id:8810006378346751937"
// May return nil, nil if the task is not a build task at all.
func buildInfoForTask(s *apipb.TaskRequestMetadataResponse) (*buildInfo, error) {
	ret := &buildInfo{}
	for _, tag := range s.TaskResult.Tags {
		assignIfHasPrefix("buildbucket_build_id:", tag, &ret.buildbucketBuildID)
		assignIfHasPrefix("buildbucket_hostname:", tag, &ret.buildbucketHostname)
		assignIfHasPrefix("buildbucket_bucket:", tag, &ret.buildbucketBucket)
		assignIfHasPrefix("builder:", tag, &ret.builder)
		appendIfHasPrefix("buildset:", tag, &ret.buildSet)
	}
	if ret.buildbucketBuildID == "" {
		return nil, nil
	}
	return ret, nil
}

func runInfoForTask(s *apipb.TaskRequestMetadataResponse) (*runInfo, error) {
	ret := &runInfo{}

	// t.request.Properties.Dimensions reflect what the user (e.g. via Pinpoint) asked swarming to
	// run the task on.
	for _, dim := range s.Request.Properties.Dimensions {
		if dim.Key == "cpu" {
			ret.cpu = dim.Value
		}
		if dim.Key == "os" || dim.Key == "device_os" {
			ret.os = dim.Value
		}
		if dim.Key == "benchmark" {
			ret.benchmark = dim.Value
		}
		if dim.Key == "storyfilter" {
			ret.storyFilter = dim.Value
		}
	}

	// t.result.BotDimensions reflect what hardware swarming actually executed the task on.
	for _, d := range s.TaskResult.BotDimensions {
		// I am not sure who sets this field or how, but it appears to be there and kinda fits
		// the bill for a name for a specific "hardware/OS/other runtime stuff" configuration.
		if d.Key == "synthetic_product_name" {
			if len(d.Value) == 0 {
				js, _ := json.Marshal(s)
				sklog.Errorf("result: %s", string(js))
				return nil, fmt.Errorf("task %q had empty values for synthetic_product_name: %#v", s.TaskId, s.TaskResult.BotDimensions)

			}
			ret.syntheticProductName = d.Value[0]
		}
	}

	if ret.cpu == "" && ret.os == "" && ret.benchmark == "" && ret.storyFilter == "" && ret.syntheticProductName == "" {
		sklog.Errorf("unable to get valid runInfo for task:\n%#v\n", s)
		return nil, fmt.Errorf("unable to get valid runInfo for task (%+v): %+v", s, ret)
	}

	ret.botID = s.TaskResult.BotId
	ret.startTimestamp = s.TaskResult.StartedTs.AsTime().Format(swarming.TIMESTAMP_FORMAT)

	return ret, nil
}

// all of the data that is specific to one arm of an experiment.
type processedArmTasks struct {
	// change tag is specific to the pinpoint tryjob use case, also useful for gerrit use case.
	// This value comes from run tasks.
	pinpointChangeTag string

	// This value comes from build tasks.
	buildset []string

	tasks []*armTask
}

func (a *processedArmTasks) outputDigests() []*apipb.CASReference {
	ret := []*apipb.CASReference{}
	for _, t := range a.tasks {
		ret = append(ret, t.resultOutput)
	}
	return ret
}

type buildInfo struct {
	buildbucketBucket,
	buildbucketBuildID,
	buildbucketHostname,
	builder string
	buildSet []string
}

func (b *buildInfo) String() string {
	if b == nil {
		return ""
	}
	return b.builder
}

type runInfo struct {
	// Example: "Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0"
	syntheticProductName string
	cpu, os              string
	// These are more for valdating against what we expect to find in the results, according to the
	// requested AnalysisSpec.
	benchmark, storyFilter string
	// These fields are for determining pairing order.
	botID, startTimestamp string
}

func (r *runInfo) String() string {
	return fmt.Sprintf("%s,%s", r.os, r.cpu)
}

// all of the data that is specific to one measurement task for one arm of an experiment.
type armTask struct {
	taskID                 string
	resultOutput           *apipb.CASReference
	buildConfig, runConfig string
	buildInfo              *buildInfo
	parsedResults          map[string]perfresults.PerfResults
	taskInfo               *apipb.TaskRequestMetadataResponse
}

// all of the data collected for an experiment.
type processedExperimentTasks struct {
	control, treatment *processedArmTasks
}

// returns a list of task pairs, where pairs are identified by botID and start time for the task.
func (a *processedExperimentTasks) pairedTasks(excludedSwarmingTasks map[string]*SwarmingTaskDiagnostics) ([]pairedTasks, error) {
	ret := []pairedTasks{}
	// Sort tasks based on bot id + task start time.
	sort.Sort(byPairingOrder(a.control.tasks))
	sort.Sort(byPairingOrder(a.treatment.tasks))

	cPointer := 0
	tPointer := 0
	for cPointer < len(a.control.tasks) && tPointer < len(a.treatment.tasks) {
		c := a.control.tasks[cPointer]
		t := a.treatment.tasks[tPointer]

		if c.taskInfo.TaskResult.BotId == t.taskInfo.TaskResult.BotId && c.runConfig == t.runConfig && c.buildConfig == t.buildConfig {
			if _, ok := excludedSwarmingTasks[c.taskID]; ok {
				sklog.Warningf("Exclude swarming tasks: %s and %s, since the control task %s is in the exclude list: %s", c.taskID, t.taskID, c.taskID, excludedSwarmingTasks[c.taskID].Message)
			} else if _, ok := excludedSwarmingTasks[t.taskID]; ok {
				sklog.Warningf("Exclude swarming tasks: %s and %s, since the treatment task %s is in the exclude list: %s", c.taskID, t.taskID, t.taskID, excludedSwarmingTasks[t.taskID].Message)
			} else {
				ret = append(ret, pairedTasks{c, t})
			}
			cPointer++
			tPointer++
		} else if c.taskInfo.TaskResult.BotId != t.taskInfo.TaskResult.BotId {
			// if bot id don't match, skip the smaller bot id
			if c.taskInfo.TaskResult.BotId < t.taskInfo.TaskResult.BotId {
				sklog.Warningf("Exclude control swarming tasks: %s, since the bot id don't match. c.BotId = %s, t.BotId = %s", c.taskID, c.taskInfo.TaskResult.BotId, t.taskInfo.TaskResult.BotId)
				cPointer++
			} else {
				sklog.Warningf("Exclude treatment swarming tasks: %s, since the bot id don't match. c.BotId = %s, t.BotId = %s", t.taskID, c.taskInfo.TaskResult.BotId, t.taskInfo.TaskResult.BotId)
				tPointer++
			}
		} else {
			// if bot id match but the config doesn't match, skip both tasks.
			sklog.Warningf("Exclude swarming tasks: %s and %s, since the config don't match. c.runConfig = %s, t.runConfig = %s, c.buildConfig = %s, t.buildConfig = %s", c.taskID, t.taskID, c.runConfig, t.runConfig, c.buildConfig, t.buildConfig)
			cPointer++
			tPointer++
		}
	}

	return ret, nil
}

type pairedTasks struct {
	control, treatment *armTask
}

func (p *pairedTasks) isControlOrderFirst() bool {
	return p.control.taskInfo.TaskResult.StartedTs.AsTime().Before(p.treatment.taskInfo.TaskResult.StartedTs.AsTime())
}

func (p *pairedTasks) hasTaskFailures() bool {
	if p.control.taskInfo.TaskResult.ExitCode != 0 || p.treatment.taskInfo.TaskResult.ExitCode != 0 {
		return true
	}
	if p.control.parsedResults == nil || p.treatment.parsedResults == nil {
		return true
	}
	return false
}

type byPairingOrder []*armTask

func (v byPairingOrder) Len() int      { return len(v) }
func (v byPairingOrder) Swap(i, j int) { v[i], v[j] = v[j], v[i] }

// This looks fairly naive, but its assumptions should hold. Task pairs should execute on the same
// bot ID (device), and sorting tasks by start time within a given bot ID should give us the
// actual run order for tasks on that bot.  Even if bots are re-used by subsequent task pairs,
// the tasks should still be in pairing order within the given bot ID.
func (v byPairingOrder) Less(i, j int) bool {
	if v[i].taskInfo.TaskResult.BotId == v[j].taskInfo.TaskResult.BotId {
		return v[i].taskInfo.TaskResult.StartedTs.AsTime().Before(v[j].taskInfo.TaskResult.StartedTs.AsTime())
	}
	return v[i].taskInfo.TaskResult.BotId < v[j].taskInfo.TaskResult.BotId
}
