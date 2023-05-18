package analyzer

// This file contains struct types and helper functions for manipulating
// in-memory representations of experiment measurement task metadata.
import (
	"encoding/json"
	"fmt"
	"strings"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/cabe/go/perfresults"
	"go.skia.org/infra/go/sklog"
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

// To get all the information we need about a given swarming task, we need both the results and
// the original request. This struct helps us pair them.
type swarmingTaskInfo struct {
	taskID  string
	request *swarming.SwarmingRpcsTaskRequest
	result  *swarming.SwarmingRpcsTaskResult
}

// Pinpoint will set a "change:..." tag on the swarming tasks it runs for each arm.
// The contents are formatted here: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/change/change.py;l=52
// however we only care about the presence of the "change:" prefix in this function. The rest
// (if anything) is handled by the caller.
func pinpointChangeTagForTask(t swarmingTaskInfo) string {
	for _, tag := range t.result.Tags {
		if strings.HasPrefix(tag, "change:") {
			return tag[len("change:"):]
		}
	}
	return ""
}

// Request dimensions are key value pairs, and keys can appear more that once.  This function
// groups values by key.
func (s *swarmingTaskInfo) requestDimensions() map[string][]string {
	ret := make(map[string][]string)
	for _, dim := range s.request.Properties.Dimensions {
		v := ret[dim.Key]
		if v == nil {
			v = []string{}
			ret[dim.Key] = v
		}
		ret[dim.Key] = append(v, dim.Value)
	}
	return ret
}

func (s *swarmingTaskInfo) resultBotDimensions() map[string][]string {
	ret := make(map[string][]string)
	for _, dim := range s.result.BotDimensions {
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
func (s *swarmingTaskInfo) buildInfo() (*buildInfo, error) {
	ret := &buildInfo{}
	for _, tag := range s.result.Tags {
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

func (s *swarmingTaskInfo) runInfo() (*runInfo, error) {
	ret := &runInfo{}

	// t.request.Properties.Dimensions reflect what the user (e.g. via Pinpoint) asked swarming to
	// run the task on.
	for _, dim := range s.request.Properties.Dimensions {
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
	for _, d := range s.result.BotDimensions {
		// I am not sure who sets this field or how, but it appears to be there and kinda fits
		// the bill for a name for a specific "hardware/OS/other runtime stuff" configuration.
		if d.Key == "synthetic_product_name" {
			if len(d.Value) == 0 {
				js, _ := json.Marshal(s.result)
				sklog.Errorf("result: %s", string(js))
				return nil, fmt.Errorf("task %q had empty values for synthetic_product_name: %#v", s.taskID, s.result.BotDimensions)

			}
			ret.syntheticProductName = d.Value[0]
		}
	}

	if ret.cpu == "" && ret.os == "" && ret.benchmark == "" && ret.storyFilter == "" && ret.syntheticProductName == "" {
		sklog.Errorf("unable to get valid runInfo for task:\n%#v\n", s)
		return nil, fmt.Errorf("unable to get valid runInfo for task (%+v): %+v", s, ret)
	}

	ret.botID = s.result.BotId
	if s.result.StartedTs == "" {
		return nil, fmt.Errorf("task %q had no StartedTs value", s.taskID)
	}
	ret.startTimestamp = s.result.StartedTs

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

func (a *processedArmTasks) outputDigests() []*swarming.SwarmingRpcsCASReference {
	ret := []*swarming.SwarmingRpcsCASReference{}
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
	resultOutput           *swarming.SwarmingRpcsCASReference
	buildConfig, runConfig string
	botID                  string
	buildInfo              *buildInfo
	runInfo                *runInfo
	parsedResults          map[string]perfresults.PerfResults
	taskInfo               swarmingTaskInfo
}
