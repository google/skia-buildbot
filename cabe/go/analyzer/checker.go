package analyzer

import (
	"fmt"
	"sort"
	"strings"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/util"

	specpb "go.skia.org/infra/cabe/go/proto"
)

const (
	taskCompletedState = "COMPLETED"
)

// Checker performs diagnostic checks on experiment artifacts prior to analysis. Its main use case
// is akin to a compiler's static type checker - it identifies conditions that violate assumptions
// about the input data before proceeding with the rest of the input processing steps.
type Checker interface {
	// Findings returns a list of strings describing potential issues that the checker identified.
	Findings() []string
	// CheckSwarmingTask validates a single swarming task in isolation.
	CheckSwarmingTask(taskInfo *apipb.TaskRequestMetadataResponse)
	// CheckRunTask validates a single swarming run task request/result pair in isolation.
	CheckRunTask(taskInfo *apipb.TaskRequestMetadataResponse)
	// CheckArmComparability validates assumptions about how treatment and control arm tasks may
	// differ from each other, and how tasks within an arm may differ from each other.
	CheckArmComparability(controls, treatments *processedArmTasks)
	// CheckControlTreatmentSpecMatch validates assumptions that the run spec of treatment and control
	// should match.
	CheckControlTreatmentSpecMatch(controlSpec, treatmentSpec *specpb.ExperimentSpec) error
}

// checker implements Checker.
type checker struct {
	// findings contains a list of descriptions of problems that the checker has found so far.
	findings                    []string
	ignoredDimKeys              util.StringSet
	ignoredTagKeys              util.StringSet
	expectedRunRequestTagKeys   util.StringSet
	expectedRunResultTagKeys    util.StringSet
	expectedRunRequestDimKeys   util.StringSet
	expectedRunResultBotDimKeys util.StringSet
}

var _ Checker = &checker{}

func (c *checker) Findings() []string {
	return c.findings
}

// CheckerOptions configure the behavior of Checker.
type CheckerOptions func(*checker)

// IgnoreDimKeys tells Checker to ignore the specified keys when running checks.
func IgnoreDimKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.ignoredDimKeys = util.NewStringSet(v).Union(c.ignoredDimKeys)
	}
}

// IgnoreTagKeys tells Checker to ignore the specified keys when running checks.
func IgnoreTagKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.ignoredTagKeys = util.NewStringSet(v).Union(c.ignoredTagKeys)
	}
}

// ExpectRunRequestDimKeys tells Checker to verify the existence of the specified keys.
func ExpectRunRequestDimKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.expectedRunRequestDimKeys = util.NewStringSet(v).Union(c.expectedRunRequestDimKeys)
	}
}

// ExpectRunResultBotDimKeys tells Checker to verify the existence of the specified keys.
func ExpectRunResultBotDimKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.expectedRunResultBotDimKeys = util.NewStringSet(v).Union(c.expectedRunResultBotDimKeys)
	}
}

// ExpectRunRequestTagKeys tells Checker to verify the existence of the specified keys.
func ExpectRunRequestTagKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.expectedRunRequestTagKeys = util.NewStringSet(v).Union(c.expectedRunRequestTagKeys)
	}
}

// ExpectRunResultTagKeys tells Checker to verify the existence of the specified keys.
func ExpectRunResultTagKeys(v ...string) CheckerOptions {
	return func(c *checker) {
		c.expectedRunResultTagKeys = util.NewStringSet(v).Union(c.expectedRunResultTagKeys)
	}
}

var (
	// DefaultCheckerOpts is a set of basic default checks to run on experiment data.
	DefaultCheckerOpts = []CheckerOptions{
		IgnoreDimKeys("id"),
		IgnoreTagKeys("id"),
		ExpectRunRequestDimKeys("os"),
		ExpectRunResultBotDimKeys("cpu", "synthetic_product_name"),
		ExpectRunRequestTagKeys("benchmark", "storyfilter"),
		ExpectRunResultTagKeys("os", "benchmark", "storyfilter"),
	}
)

// NewChecker returns an instance of Checker with the specified configuration options. If you
// are unsure which CheckerOptions make sense, try starting with DefaultCheckerOpts.
func NewChecker(opts ...CheckerOptions) Checker {
	ret := &checker{}
	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func (c *checker) addFinding(checkName string, msg string) {
	c.findings = append(c.findings, fmt.Sprintf("%s: %s", checkName, msg))
}

func (c *checker) CheckSwarmingTask(taskInfo *apipb.TaskRequestMetadataResponse) {
	addFinding := func(msg string) {
		c.addFinding(fmt.Sprintf("CheckSwarmingTask %q", taskInfo.TaskId), msg)
	}
	if taskInfo.TaskResult == nil {
		addFinding("SwarmingRpcsTaskRequestMetadata had no TaskResult")
		return
	}
	if taskInfo.TaskResult.State != apipb.TaskState_COMPLETED {
		addFinding(fmt.Sprintf("TaskResult is in state %q rather than %q", taskInfo.TaskResult.State, taskCompletedState))
	}
}

// CheckRunTask verifies assumptions about an individual request/result pair for a Swarming task
// that executed a benchmark.
func (c *checker) CheckRunTask(taskInfo *apipb.TaskRequestMetadataResponse) {
	if taskInfo == nil {
		c.addFinding("CheckRunTask", "taskInfo was nil")
		return
	}

	addFinding := func(msg string) {
		c.addFinding(fmt.Sprintf("CheckRunTask %q", taskInfo.TaskId), msg)
	}

	if taskInfo.Request == nil {
		addFinding("taskInfo.request was nil")
		return
	} else if taskInfo.Request.Properties == nil {
		addFinding("taskInfo.request.Properties was nil")
		return
	}

	if taskInfo.TaskResult == nil {
		addFinding("taskInfo.result was nil")
		return
	} else if taskInfo.TaskResult.BotDimensions == nil {
		addFinding("taskInfo.result.BotDimensions was nil")
		return
	}

	requestDims := requestDimensionsForTask(taskInfo)
	if _, ok := requestDims["builder"]; ok {
		// This is a builder task, not a runner task, so skip it.
		return
	}

	// Check for missing request dimensions.
	requestDimKeys := util.NewStringSet()
	for k := range requestDims {
		requestDimKeys[k] = true
	}

	for k := range c.expectedRunRequestDimKeys {
		if _, ok := requestDimKeys[k]; !ok {
			addFinding(fmt.Sprintf("missing request dimension for %q", k))
		}
	}

	// Check for missing request tags.
	requestTagKeys := util.NewStringSet()
	for k := range tagsToMap(taskInfo.Request.Tags) {
		requestTagKeys[k] = true
	}

	for k := range c.expectedRunRequestTagKeys {
		if _, ok := requestTagKeys[k]; !ok {
			addFinding(fmt.Sprintf("missing request tag %q", k))
		}
	}

	// Check for missing result dimensions.
	resultBotDimKeys := util.NewStringSet()
	for k := range resultBotDimensionsForTask(taskInfo) {
		resultBotDimKeys[k] = true
	}

	for k := range c.expectedRunResultBotDimKeys {
		if _, ok := resultBotDimKeys[k]; !ok {
			addFinding(fmt.Sprintf("missing result bot dimension %q", k))
		}
	}

	// Check for missing result tags.
	resultTagKeys := util.NewStringSet()
	for k := range tagsToMap(taskInfo.TaskResult.Tags) {
		resultTagKeys[k] = true
	}

	for k := range c.expectedRunResultTagKeys {
		if _, ok := resultTagKeys[k]; !ok {
			addFinding(fmt.Sprintf("missing result tag %q", k))
		}
	}
}

// CheckArmComparability verifies that two arms of an experiment are comparable according to the
// kind of experiment you are trying to analyze.  While some differences between tasks in each arm
// are expected (e.g. bot IDs, timestamps), others may not be (e.g. OS or GPU version).
func (c *checker) CheckArmComparability(controls, treatments *processedArmTasks) {
	addFinding := func(msg string) {
		c.addFinding("CheckArmComparability", msg)
	}

	controlArmTasks := armTasks(controls.tasks)
	treatmentArmTasks := armTasks(treatments.tasks)
	allArmTasks := armTasks(append(controls.tasks, treatments.tasks...))

	// Check that swarming task request dimensions are comparable between the two arms
	controlDisjointRequestDims := controlArmTasks.disjointRequestDimensions(c.ignoredDimKeys)
	treatmentDisjointRequestDims := treatmentArmTasks.disjointRequestDimensions(c.ignoredDimKeys)
	allDisjointRequestDims := allArmTasks.disjointRequestDimensions(c.ignoredDimKeys)
	sort.Strings(controlDisjointRequestDims)
	sort.Strings(treatmentDisjointRequestDims)
	sort.Strings(allDisjointRequestDims)

	if len(controlDisjointRequestDims) != 0 {
		addFinding(fmt.Sprintf("disjoint request dims within control: %v", controlDisjointRequestDims))
	}
	if len(treatmentDisjointRequestDims) != 0 {
		addFinding(fmt.Sprintf("disjoint request dims within treatment: %v", treatmentDisjointRequestDims))
	}
	if len(allDisjointRequestDims) != 0 {
		addFinding(fmt.Sprintf("disjoint request dims across arms: %v", allDisjointRequestDims))
	}

	// Check that swarming task request tags are comparable between the two arms.
	controlDisjointRequestTags := controlArmTasks.disjointRequestTags(c.ignoredTagKeys)
	treatmentDisjointRequestTags := treatmentArmTasks.disjointRequestTags(c.ignoredTagKeys)
	allDisjointRequestTags := allArmTasks.disjointRequestTags(c.ignoredTagKeys)
	sort.Strings(controlDisjointRequestTags)
	sort.Strings(treatmentDisjointRequestTags)
	sort.Strings(allDisjointRequestTags)
	if len(controlDisjointRequestTags) != 0 {
		addFinding(fmt.Sprintf("disjoint request tags within control: %v", controlDisjointRequestTags))
	}
	if len(treatmentDisjointRequestTags) != 0 {
		addFinding(fmt.Sprintf("disjoint request tags within treatment: %v", treatmentDisjointRequestTags))
	}
	if len(allDisjointRequestTags) != 0 {
		addFinding(fmt.Sprintf("disjoint request tags across arms: %v", allDisjointRequestTags))
	}

	// Check that swarming task result dimensions are comparable between the two arms.
	controlDisjointResultDims := controlArmTasks.disjointResultDimensions(c.ignoredDimKeys)
	treatmentDisjointResultDims := treatmentArmTasks.disjointResultDimensions(c.ignoredDimKeys)
	allDisjointResultDims := allArmTasks.disjointResultDimensions(c.ignoredDimKeys)
	sort.Strings(controlDisjointResultDims)
	sort.Strings(treatmentDisjointResultDims)
	sort.Strings(allDisjointResultDims)
	if len(controlDisjointResultDims) != 0 {
		addFinding(fmt.Sprintf("disjoint result dims within control tasks: %v", controlDisjointResultDims))
	}
	if len(treatmentDisjointResultDims) != 0 {
		addFinding(fmt.Sprintf("disjoint result dims within treatment tasks: %v", treatmentDisjointResultDims))
	}
	if len(allDisjointResultDims) != 0 {
		addFinding(fmt.Sprintf("disjoint result dims across arms' tasks: %v", allDisjointResultDims))
	}

	// Check that swarming task result tags are comparable between the two arms.
	controlDisjointResultTags := controlArmTasks.disjointResultTags(c.ignoredTagKeys)
	treatmentDisjointResultTags := treatmentArmTasks.disjointResultTags(c.ignoredTagKeys)
	allDisjointResultTags := allArmTasks.disjointResultTags(c.ignoredTagKeys)
	sort.Strings(controlDisjointResultTags)
	sort.Strings(treatmentDisjointResultTags)
	sort.Strings(allDisjointResultTags)
	if len(controlDisjointResultTags) != 0 {
		addFinding(fmt.Sprintf("disjoint result tags within control: %v", controlDisjointResultTags))
	}
	if len(treatmentDisjointResultTags) != 0 {
		addFinding(fmt.Sprintf("disjoint result tags within treatment: %v", treatmentDisjointResultTags))
	}
	if len(allDisjointResultTags) != 0 {
		addFinding(fmt.Sprintf("disjoint result tags across arms: %v", allDisjointResultTags))
	}
}

type armTasks []*armTask

// Returns all the dimension key/values that appear in at least one result, but not all of the results.
func (s armTasks) disjointResultDimensions(ignoredKeys util.StringSet) []string {
	dimensionSets := []map[string][]string{}
	for _, t := range s {
		dimensionSets = append(dimensionSets, resultBotDimensionsForTask(t.taskInfo))
	}
	return disjointDimensions(dimensionSets, ignoredKeys)

}

// Returns all the tags that appear in at least one result, but not all of the results.
func (s armTasks) disjointResultTags(ignoredKeys util.StringSet) []string {
	tagSets := [][]string{}
	for _, task := range s {
		tagSets = append(tagSets, task.taskInfo.TaskResult.Tags)
	}
	return disjointTags(tagSets, ignoredKeys)
}

// Returns all the dimension key/values that appear in at least one request, but not all of the requests.
func (s armTasks) disjointRequestDimensions(ignoredKeys util.StringSet) []string {
	dimensionSets := []map[string][]string{}
	for _, t := range s {
		dimensionSets = append(dimensionSets, requestDimensionsForTask(t.taskInfo))
	}
	return disjointDimensions(dimensionSets, ignoredKeys)
}

// Returns all the tags that appear in at least one request, but not all of the requests.
func (s armTasks) disjointRequestTags(ignoredKeys util.StringSet) []string {
	tagSets := [][]string{}
	for _, task := range s {
		tagSets = append(tagSets, task.taskInfo.Request.Tags)
	}
	return disjointTags(tagSets, ignoredKeys)
}

// Returns all the dimension key/value sets that appear in at least one set of dimensions, but not all of the sets of dimensions.
func disjointDimensions(dimensionSets []map[string][]string, ignoredKeys util.StringSet) []string {
	if len(dimensionSets) < 2 {
		return nil
	}
	ret := []string{}
	kvCounts := make(map[string]int)
	for _, dimSet := range dimensionSets {
		for k, v := range dimSet {
			if _, ok := ignoredKeys[k]; ok {
				continue
			}
			sort.Strings(v)
			dimKey := fmt.Sprintf("key: %q, values: %q", k, v)
			kvCounts[dimKey]++
		}
	}
	for k, v := range kvCounts {
		if v < len(dimensionSets) { // k appeared in some, but not all dimensionSets.
			ret = append(ret, fmt.Sprintf("%d tasks with {%s}", v, k))
		}
	}
	return ret
}

// Returns all the tag keys that appear in at least one set of tags, but not all of the sets of tags.
func disjointTags(tagSets [][]string, ignoredKeys util.StringSet) []string {
	if len(tagSets) < 2 {
		return nil
	}
	ret := []string{}
	kvCounts := make(map[string]int)
	for _, tagSet := range tagSets {
		for k := range tagsToMap(tagSet) {
			if _, ok := ignoredKeys[k]; ok {
				continue
			}
			kvCounts[k]++
		}
	}
	for k, v := range kvCounts {
		if v == 1 {
			ret = append(ret, k)
		}
	}
	return ret
}

func tagsToMap(s []string) map[string][]string {
	ret := map[string][]string{}
	for _, tag := range s {
		s := strings.Split(tag, ":")
		// Split on ":" char, but only use the first element as the key and re-pack the rest back into the value.
		// It's common for tag values to contain ":" chars themselves, which we need to preserve.
		k, v := s[0], strings.Join(s[1:], "")
		ret[k] = append(ret[k], v)
	}
	return ret
}

// CheckControlTreatmentSpecMatch validates assumptions that the run spec of treatment and control
// should match.
func (c *checker) CheckControlTreatmentSpecMatch(controlSpec, treatmentSpec *specpb.ExperimentSpec) error {
	if controlSpec.Control == nil || treatmentSpec.Treatment == nil {
		return fmt.Errorf("CheckControlTreatmentSpecMatch: the control spec (%v) does not have control or the treatment spec (%v) doesn't have treatment", controlSpec, treatmentSpec)
	}
	controlRunSpecs := controlSpec.GetControl().GetRunSpec()
	treatmentRunSpecs := treatmentSpec.GetTreatment().GetRunSpec()

	if controlRunSpecs == nil || len(controlRunSpecs) != 1 {
		return fmt.Errorf("CheckControlTreatmentSpecMatch: the control run specs (%v) expects to have length 1, but got %v", controlRunSpecs, len(controlRunSpecs))
	}

	if treatmentRunSpecs == nil || len(treatmentRunSpecs) != 1 {
		return fmt.Errorf("CheckControlTreatmentSpecMatch: the treatment run specs (%v) expects to have length 1, but got %v", treatmentRunSpecs, len(treatmentRunSpecs))
	}

	controlRunSpec := controlRunSpecs[0]
	treatmentRunSpec := treatmentRunSpecs[0]

	if controlRunSpec.GetOs() != treatmentRunSpec.GetOs() || controlRunSpec.GetSyntheticProductName() != treatmentRunSpec.GetSyntheticProductName() {
		return fmt.Errorf("CheckControlTreatmentSpecMatch: the control run spec (%v) and treatment run spec (%v) are not same", controlRunSpec, treatmentRunSpec)
	}

	return nil
}
