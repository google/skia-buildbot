package analyzer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"go.opencensus.io/trace"

	stat "github.com/aclements/go-moremath/stats"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/cabe/go/backends"
	cpb "go.skia.org/infra/cabe/go/proto"
	cabe_stats "go.skia.org/infra/cabe/go/stats"
	"go.skia.org/infra/perf/go/perfresults"
)

const maxReadCASPoolWorkers = 100

// Options configure one or more fields of an Analyzer instance.
type Options func(*Analyzer)

// WithCASResultReader configures an Analyzer instance to use the given CASResultReader.
func WithCASResultReader(r backends.CASResultReader) Options {
	return func(e *Analyzer) {
		e.readCAS = r
	}
}

// WithTaskResultsReader configures an Analyzer instance to use the given TaskResultsReader.
func WithSwarmingTaskReader(r backends.SwarmingTaskReader) Options {
	return func(e *Analyzer) {
		e.readSwarmingTasks = r
	}
}

// WithExperimentSpec configures an Analyzer instance to use the given ExperimentSpec.
func WithExperimentSpec(s *cpb.ExperimentSpec) Options {
	return func(e *Analyzer) {
		e.experimentSpec = s
	}
}

// New returns a new instance of Analyzer. Set either pinpointJobID, or controlDigests and treatmentDigests.
func New(pinpointJobID string, opts ...Options) *Analyzer {
	ret := &Analyzer{
		pinpointJobID: pinpointJobID,
		diagnostics:   newDiagnostics(),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Analyzer encapsulates the state of an Analyzer process execution. Its lifecycle follows a request
// to process all of the output of an A/B benchmark experiment run.
// Users of Analyzer must instantiate and attach the necessary service dependencies.
type Analyzer struct {
	pinpointJobID     string
	readCAS           backends.CASResultReader
	readSwarmingTasks backends.SwarmingTaskReader

	experimentSpec *cpb.ExperimentSpec
	diagnostics    *Diagnostics

	results []Results
}

func (a *Analyzer) Diagnostics() *Diagnostics {
	return a.diagnostics
}

// Results encapsulates a response from the Go statistical package after it has processed
// swarming task data and verified the experimental setup is valid for analysis.
type Results struct {
	// Benchmark is the name of a perf benchmark suite, such as Speedometer2 or JetStream
	Benchmark string
	// Workload is the name of a benchmark-specific workload, such as TodoMVC-ReactJS
	WorkLoad string
	// BuildConfig is the name of a build configuration, e.g. "Mac arm Builder Perf PGO"
	BuildConfig string
	// RunConfig is the name of a run configuration, e.g. "Macmini9,1_arm64-64-Apple_M1_16384_1_4744421.0"
	RunConfig string
	// Statistics summarizes the difference between the treatment and control arms for the given
	// Benchmark and Workload on the hardware described by RunConfig, using the binary built using
	// the given BuildConfig.
	Statistics *cabe_stats.BerfWilcoxonSignedRankedTestResult
}

// AnalysisResults returns a slice of AnalysisResult protos populated with data from the
// experiment.
func (a *Analyzer) AnalysisResults() []*cpb.AnalysisResult {
	ret := []*cpb.AnalysisResult{}
	// Because ExperimentSpec will have so many identical
	// values across individual results, we'll build a template here
	// then clone and override the distinct per-result values for the
	// response proto below.
	//
	// Note that for most Pinpoint A/B tryjobs, the ExperimentSpec will
	// have a Common RunSpec set, and Treatment and Control will have different BuildSpec values.
	// That is, compare two different builds executing on the same hardware/OS.
	experimentSpecTemplate := a.experimentSpec
	if experimentSpecTemplate.Analysis == nil {
		experimentSpecTemplate.Analysis = &cpb.AnalysisSpec{}
	}
	experimentSpecTemplate.Analysis.Benchmark = nil

	sort.Sort(byBenchmarkAndWorkload(a.results))

	for _, res := range a.results {
		experimentSpec := proto.Clone(experimentSpecTemplate).(*cpb.ExperimentSpec)
		benchmark := []*cpb.Benchmark{
			{
				Name:     res.Benchmark,
				Workload: []string{res.WorkLoad},
			},
		}

		experimentSpec.Analysis.Benchmark = benchmark

		ret = append(ret, &cpb.AnalysisResult{
			ExperimentSpec: experimentSpec,
			Statistic: &cpb.Statistic{
				Upper:           res.Statistics.UpperCi,
				Lower:           res.Statistics.LowerCi,
				PValue:          res.Statistics.PValue,
				ControlMedian:   res.Statistics.YMedian,
				TreatmentMedian: res.Statistics.XMedian,
			},
		})
	}
	return ret
}

type byBenchmarkAndWorkload []Results

func (a byBenchmarkAndWorkload) Len() int      { return len(a) }
func (a byBenchmarkAndWorkload) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byBenchmarkAndWorkload) Less(i, j int) bool {
	if a[i].Benchmark != a[j].Benchmark {
		return a[i].Benchmark < a[j].Benchmark
	}
	return a[i].WorkLoad < a[j].WorkLoad
}

func (a *Analyzer) ExperimentSpec() *cpb.ExperimentSpec {
	return a.experimentSpec
}

// Run executes the whole Analyzer process for a single, complete experiment.
// TODO(seanmccullough): break this up into distinct, testable stages with one function per stage.
func (a *Analyzer) Run(ctx context.Context) ([]Results, error) {
	ctx, span := trace.StartSpan(ctx, "Analyzer_Run")
	defer span.End()

	var control, treatment []float64
	var benchmark, workload, buildspec, runspec []string
	var replicas []int

	res := []Results{}

	allTaskInfos, err := a.readSwarmingTasks(ctx, a.pinpointJobID)
	if err != nil {
		return res, err
	}

	// TODO(seanmccullough): choose different process<Foo>Tasks implementations based on AnalysisSpec values.
	processedArms, err := a.processPinpointTryjobTasks(allTaskInfos)
	if err != nil {
		return res, err
	}
	err = a.extractTaskOutputs(ctx, processedArms)
	if err != nil {
		return res, err
	}

	// TODO(seanmccullough): include pairing order information so we keep track of which arm executed first in
	// every pairing.
	pairs, err := processedArms.pairedTasks(a.diagnostics.ExcludedSwarmingTasks)
	if err != nil {
		return res, err
	}

	if a.experimentSpec == nil {
		ieSpec, err := a.inferExperimentSpec(pairs)
		if err != nil {
			sklog.Errorf("while trying to infer experiment spec: %v", err)
			return res, err
		}

		a.experimentSpec = ieSpec
	}

	transformType := cabe_stats.LogTransform

	for replicaNumber, pair := range pairs {
		// Check task result codes to identify and handle task failures (which are expected; lab hardware is inherently unreliable).
		if pair.hasTaskFailures() {
			a.diagnostics.excludeReplica(replicaNumber, pair, "one or both tasks failed")
			continue
		}
		pairIsOk := true
		for _, benchmarkSpec := range a.experimentSpec.Analysis.Benchmark {
			controlResults, crOk := pair.control.parsedResults[benchmarkSpec.Name]
			treatmentResults, trOk := pair.treatment.parsedResults[benchmarkSpec.Name]
			if !crOk || !trOk {
				a.diagnostics.excludeReplica(replicaNumber, pair, fmt.Sprintf("one or both tasks are missing output for entire benchmark %q", benchmarkSpec.Name))
				pairIsOk = false
				continue
			}
			expectedWorkloads := util.NewStringSet(benchmarkSpec.Workload)
			missingFromCtrl := expectedWorkloads.Complement(util.NewStringSet(controlResults.NonEmptyHistogramNames()))
			missingFromTrt := expectedWorkloads.Complement(util.NewStringSet(treatmentResults.NonEmptyHistogramNames()))
			if len(missingFromCtrl) != 0 {
				a.diagnostics.excludeReplica(replicaNumber, pair, fmt.Sprintf("control task %s on bot %s is missing output for benchmark %s: workload(s) %v",
					pair.control.taskID, pair.control.taskInfo.TaskResult.BotId, benchmarkSpec.Name, missingFromCtrl))
				pairIsOk = false
			}
			if len(missingFromTrt) != 0 {
				a.diagnostics.excludeReplica(replicaNumber, pair, fmt.Sprintf("treatment task %s on bot %s is missing output for benchmark %s workload(s) %v",
					pair.treatment.taskID, pair.treatment.taskInfo.TaskResult.BotId, benchmarkSpec.Name, missingFromTrt))
				pairIsOk = false
			}
		}
		if !pairIsOk {
			continue
		}
		controlTask, treatmentTask := pair.control, pair.treatment

		runSpecName := pair.control.runConfig
		buildSpecName := pair.control.buildConfig
		for _, benchmarkSpec := range a.experimentSpec.GetAnalysis().GetBenchmark() {
			cRes := controlTask.parsedResults[benchmarkSpec.GetName()]
			tRes := treatmentTask.parsedResults[benchmarkSpec.GetName()]

			for _, workloadName := range benchmarkSpec.GetWorkload() {

				cValues := cRes.GetSampleValues(workloadName)
				if len(cValues) == 0 {
					msg := fmt.Sprintf("control task %s is missing %q/%q or reported an empty list of samples", pair.control.taskID, benchmarkSpec.GetName(), workloadName)
					a.diagnostics.excludeReplica(replicaNumber, pair, msg)
					continue
				}

				tValues := tRes.GetSampleValues(workloadName)
				if len(tValues) == 0 {
					msg := fmt.Sprintf("treatment task %s is missing %q/%q or reported an empty list of samples", pair.control.taskID, benchmarkSpec.GetName(), workloadName)
					a.diagnostics.excludeReplica(replicaNumber, pair, msg)
					continue
				}

				a.diagnostics.includeReplica(replicaNumber, pair)

				cMean := stat.Mean(cValues)
				tMean := stat.Mean(tValues)
				if tMean <= 0.0 || cMean <= 0.0 {
					sklog.Infof("detected values less than or equal to zero in measurement data, using NomralizeResult instead of LogTransform")
					transformType = cabe_stats.NormalizeResult
				}
				benchmark = append(benchmark, benchmarkSpec.GetName())
				workload = append(workload, workloadName)
				buildspec = append(buildspec, buildSpecName)
				runspec = append(runspec, runSpecName)
				replicas = append(replicas, replicaNumber)
				control = append(control, cMean)
				treatment = append(treatment, tMean)
			}
		}
	}

	// Group control/treatment pairs by [benchmark, workload, buildspec, runspec], aggregated over [replicas]
	// such that we end up with a map of [benchmark, workload, buildspec, runspec] to lists of [control, treatment] ordered by replica number within the lists
	type groupKey struct{ benchmark, workload, buildspec, runspec string }
	type tcPair struct{ control, treatment float64 }
	aggregateOverReplicas := map[groupKey]map[int]*tcPair{}

	for i, replicaNumber := range replicas {
		gk := groupKey{benchmark[i], workload[i], buildspec[i], runspec[i]}
		tcps := aggregateOverReplicas[gk]
		if tcps == nil {
			tcps = map[int]*tcPair{}
			aggregateOverReplicas[gk] = tcps
		}
		if tcps[replicaNumber] != nil {
			return res, fmt.Errorf("should not have a treatment/control pair aggregate for replica %d yet", replicaNumber)
		}
		tcps[replicaNumber] = &tcPair{control: control[i], treatment: treatment[i]}
	}

	for gk, tcps := range aggregateOverReplicas {
		ctrls, trts := []float64{}, []float64{}

		for _, tcp := range tcps {
			ctrls = append(ctrls, tcp.control)
			trts = append(trts, tcp.treatment)
		}
		r, err := cabe_stats.BerfWilcoxonSignedRankedTest(trts, ctrls, cabe_stats.TwoSided, transformType)
		if err != nil {
			sklog.Errorf("cabe_stats.BerfWilcoxonSignedRankedTest returned an error (%q), "+
				"printing the table of parameters passed to it below:",
				err)
			return res, errors.Wrap(err, "problem reported by cabe_stats.BerfWilcoxonSignedRankedTest")
		}
		res = append(res, Results{
			Benchmark:   gk.benchmark,
			WorkLoad:    gk.workload,
			BuildConfig: gk.buildspec,
			RunConfig:   gk.runspec,
			Statistics:  r,
		})
	}

	a.results = res
	return res, nil
}

// RunChecker verifies some assumptions we need to make about the experiment data input for
// our analyses.
func (a *Analyzer) RunChecker(ctx context.Context, c Checker) error {
	allTaskInfos, err := a.readSwarmingTasks(ctx, a.pinpointJobID)
	if err != nil {
		return err
	}

	for _, taskInfo := range allTaskInfos {
		c.CheckSwarmingTask(taskInfo)
	}

	processedTasks, err := a.processPinpointTryjobTasks(allTaskInfos)
	if err != nil {
		sklog.Errorf("RunChecker: processPinpointTryjobTasks returned %v", err)
		return err
	}

	err = a.extractTaskOutputs(ctx, processedTasks)
	if err != nil {
		sklog.Errorf("RunChecker: extractTaskOutputs returned %v", err)
		return err
	}

	pairs, err := processedTasks.pairedTasks(a.diagnostics.ExcludedSwarmingTasks)
	if err != nil {
		sklog.Errorf("RunChecker: processedTasks.pairedTasks() returned %v", err)
		return err
	}

	if a.experimentSpec == nil {
		ieSpec, err := a.inferExperimentSpec(pairs)
		if err != nil {
			sklog.Errorf("while trying to infer experiment spec: %v", err)
			return err
		}

		a.experimentSpec = ieSpec
	}

	for _, taskInfo := range allTaskInfos {
		c.CheckRunTask(taskInfo)
	}

	c.CheckArmComparability(processedTasks.control, processedTasks.treatment)

	return nil
}

func (a *Analyzer) inferExperimentSpec(pairs []pairedTasks) (*cpb.ExperimentSpec, error) {
	controlTaskResults, treatmentTaskResults := []map[string]perfresults.PerfResults{}, []map[string]perfresults.PerfResults{}
	controlArmSpecs, treatmentArmSpecs := []*cpb.ArmSpec{}, []*cpb.ArmSpec{}
	treatmentFailures := 0
	for replicaNumber, pair := range pairs {
		if pair.hasTaskFailures() {
			sklog.Infof("excluding replica %d from spec inference because it contains failures", replicaNumber)
			if pair.treatment.taskInfo.TaskResult.ExitCode != 0 {
				treatmentFailures++
			}
			continue
		}

		controlTaskResults = append(controlTaskResults, pair.control.parsedResults)
		treatmentTaskResults = append(treatmentTaskResults, pair.treatment.parsedResults)
		cSpec, err := inferArmSpec(pair.control.taskInfo)
		if err != nil {
			return nil, err
		}
		controlArmSpecs = append(controlArmSpecs, cSpec)
		tSpec, err := inferArmSpec(pair.treatment.taskInfo)
		if err != nil {
			return nil, err
		}
		treatmentArmSpecs = append(treatmentArmSpecs, tSpec)
	}

	// If all treatment tests fail, we have no valid pair to compare.
	// We have specific error message since it is likely related to CL changes.
	if treatmentFailures > 0 && treatmentFailures == len(pairs) {
		// This error is used in Perf On CQ builds to identify whether CL changes cause the failure.
		// Please do not update unless necessary.
		return nil, fmt.Errorf("no valid result in all tests on treatment spec")
	}

	experimentSpec, err := inferExperimentSpec(controlArmSpecs, treatmentArmSpecs, controlTaskResults, treatmentTaskResults)
	if err != nil {
		return nil, err
	}

	return experimentSpec, nil
}

func (a *Analyzer) extractTaskOutputs(ctx context.Context, processedArms *processedExperimentTasks) error {
	// This is currently un-sliced. As in, it lumps all runconfigs together. This is fine if you only
	// have one runconfig (say, you only asked to analyze Mac results).
	// TODO(seanmccullough): add slicing, which will nest the code below inside an iterator over the slices.

	// Fetch outputs from control and treatment arms in parallel since there is no data dependency between them.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		ctx, span := trace.StartSpan(ctx, "Analyzer_extractTaskOutputs_control")
		defer span.End()

		controlDigests := map[int]*apipb.CASReference{}
		for n, t := range processedArms.control.tasks {
			if t.taskInfo.TaskResult.State != apipb.TaskState_COMPLETED {
				continue
			}
			controlDigests[n] = t.taskInfo.TaskResult.CasOutputRoot
		}
		controlReplicaOutputs, err := a.fetchOutputsFromReplicas(ctx, controlDigests)
		if err != nil {
			return err
		}
		for replica, results := range controlReplicaOutputs {
			processedArms.control.tasks[replica].parsedResults = results
		}
		return nil
	})

	g.Go(func() error {
		ctx, span := trace.StartSpan(ctx, "Analyzer_extractTaskOutputs_treatment")
		defer span.End()

		treatmentDigests := map[int]*apipb.CASReference{}
		for n, t := range processedArms.treatment.tasks {
			if t.taskInfo.TaskResult.State != apipb.TaskState_COMPLETED {
				continue
			}
			treatmentDigests[n] = t.taskInfo.TaskResult.CasOutputRoot
		}
		treatmentReplicaOutputs, err := a.fetchOutputsFromReplicas(ctx, treatmentDigests)
		if err != nil {
			return err
		}
		for replica, results := range treatmentReplicaOutputs {
			processedArms.treatment.tasks[replica].parsedResults = results
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// returns a slice of maps of perfresults.PerfResults files keyed by benchmark name.
func (a *Analyzer) fetchOutputsFromReplicas(ctx context.Context, outputs map[int]*apipb.CASReference) (map[int]map[string]perfresults.PerfResults, error) {
	ret := make(map[int]map[string]perfresults.PerfResults, len(outputs))
	g, ctx := errgroup.WithContext(ctx)
	if len(outputs) > maxReadCASPoolWorkers {
		g.SetLimit(maxReadCASPoolWorkers)
	} else {
		g.SetLimit(len(outputs))
	}

	retMu := &sync.Mutex{}

	for replica, casRef := range outputs {
		replica := replica
		casRef := casRef
		g.Go(func() error {
			ctx, span := trace.StartSpan(ctx, "Analyzer_fetchOutputsFromReplicas_readOutput")
			defer span.End()

			if casRef.Digest == nil {
				sklog.Error("missing CAS reference for replica %d", replica)
				return fmt.Errorf("missing CAS reference for replica %d", replica)
			}
			casDigest, err := digest.New(casRef.Digest.Hash, casRef.Digest.SizeBytes)
			if err != nil {
				sklog.Errorf("digest.New: %v", err)
				return err
			}
			span.AddAttributes(trace.StringAttribute("digest", casDigest.String()))
			res, err := a.readCAS(ctx, casRef.CasInstance, casDigest.String())
			if err != nil {
				sklog.Errorf("e.readCAS: %v", err)
				return err
			}
			retMu.Lock()
			defer retMu.Unlock()
			ret[replica] = res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		sklog.Errorf("fetchOutputsFromReplicas: %v", err)
		return nil, err
	}

	return ret, nil
}

// Split apipb.TaskRequestMetadataResponses into experiment arms and pair tasks according to how they should be
// compared in the analysis/hypothesis testing phase.
// The slice of tasks should be a complete list of all the tasks for an experiment, and they should
// all have completed executing successfully.
func (a *Analyzer) processPinpointTryjobTasks(tasks []*apipb.TaskRequestMetadataResponse) (*processedExperimentTasks, error) {
	ret := &processedExperimentTasks{
		treatment: &processedArmTasks{},
		control:   &processedArmTasks{},
	}

	buildTasks := map[string]*buildInfo{}
	sklog.Infof("splitting %d swarming tasks into control and treatment arms", len(tasks))
	for _, task := range tasks {
		// First check if we have a build task. It doesn't run any benchmarks, but it does build the
		// binaries for running benchmarks. So we need to keep track of it for BuildSpec details later.
		buildInfo, err := buildInfoForTask(task)
		if err != nil {
			msg := fmt.Sprintf("task.buildInfo(): %v", err)
			sklog.Error(msg)
			a.diagnostics.excludeSwarmingTask(task, msg)
		}
		if buildInfo != nil {
			buildTasks[task.TaskId] = buildInfo
			// Just move on to processing the next task now that we know this was a build task.
			continue
		}

		// err should not be nil if the task is not a run task.
		runInfo, err := runInfoForTask(task)
		if err != nil {
			msg := fmt.Sprintf("runInfoForTask: %v", err)
			sklog.Error(msg)
			a.diagnostics.excludeSwarmingTask(task, msg)
		}

		if task.TaskResult.State != apipb.TaskState_COMPLETED {
			msg := fmt.Sprintf("task result is in state %q, not %q", task.TaskResult.State, taskCompletedState)
			a.diagnostics.excludeSwarmingTask(task, msg)
		} else {
			a.diagnostics.includeSwarmingTask(task)
			// If it's not a build task, assume it's a test runner task.
			// Get the CAS digest for the task output so we can fetch it later.
			if task.TaskResult.CasOutputRoot == nil || task.TaskResult.CasOutputRoot.Digest == nil {
				return nil, fmt.Errorf("run task result missing CasOutputRoot: %+v", task)
			}
		}

		t := &armTask{
			taskID:       task.TaskId,
			resultOutput: task.TaskResult.CasOutputRoot,
			buildInfo:    buildInfo,
			runConfig:    runInfo.String(),
			taskInfo:     task,
		}

		// For pinpoint tryjobs, the following assumptions should hold true:
		// treatment has a tag like "change:exp: chromium@d879632 + 0dd4ae0 (Variant: 1)"
		// control has a tag like "change:base: chromium@d879632 (Variant: 0)"
		change := pinpointChangeTagForTask(task)
		if change == "" {
			return nil, fmt.Errorf("missing pinpoint change tag: %+v", task)
		}
		if strings.HasPrefix(change, "exp:") {
			if ret.treatment.pinpointChangeTag == "" {
				ret.treatment.pinpointChangeTag = change
			}
			if change != ret.treatment.pinpointChangeTag {
				return nil, fmt.Errorf("mismatched change tag for treatment arm. Got %q but expected %q", change, ret.treatment.pinpointChangeTag)
			}
			ret.treatment.tasks = append(ret.treatment.tasks, t)
		} else if strings.HasPrefix(change, "base:") {
			if ret.control.pinpointChangeTag == "" {
				ret.control.pinpointChangeTag = change
			}
			if change != ret.control.pinpointChangeTag {
				return nil, fmt.Errorf("mismatched change tag for control arm. Got %q but expected %q", change, ret.control.pinpointChangeTag)
			}
			ret.control.tasks = append(ret.control.tasks, t)
		} else if len(strings.Split(change, "@")) == 2 {
			// This might be from a bisect job, where control and treatment are builds from two different commit positions on a main branch.
			// TODO(seanmccullough): also check for "comparison_mode:performance" tags on these tasks.
			parts := strings.Split(change, "@")
			repo := parts[0]
			cp := parts[1]
			return nil, fmt.Errorf("unsupported yet: changes that only identify repo and commit position: %v @ %v", repo, cp)
		} else {
			return nil, fmt.Errorf("unrecognized change tag: %q", change)
		}
	}

	return ret, nil
}
