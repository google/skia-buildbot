package analyzer

import (
	"fmt"
	"sort"
	"strings"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"

	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/perfresults"
)

// Returns an ArmSpec proto containing field values that are common between a and b.
func intersectArmSpecs(a, b *cpb.ArmSpec) *cpb.ArmSpec {
	ret := &cpb.ArmSpec{}

	ret.BuildSpec = intersectBuildSpecs(a.GetBuildSpec(), b.GetBuildSpec())
	ret.RunSpec = intersectRunSpecs(a.GetRunSpec(), b.GetRunSpec())
	return ret
}

// Returns an ArmSpec proto containing field values that are present in a but not in b.
func diffArmSpecs(a, b *cpb.ArmSpec) *cpb.ArmSpec {
	ret := &cpb.ArmSpec{}

	ret.BuildSpec = diffBuildSpecs(a.GetBuildSpec(), b.GetBuildSpec())
	ret.RunSpec = diffRunSpecs(a.GetRunSpec(), b.GetRunSpec())
	return ret
}

// Returns a BuildSpec proto containing field values that are common between a and b.
func intersectBuildSpecs(a, b []*cpb.BuildSpec) []*cpb.BuildSpec {
	ret := []*cpb.BuildSpec{}
	for i, aBuildSpec := range a {
		if i >= len(b) {
			break
		}
		bBuildSpec := b[i]
		cBuildSpec := &cpb.BuildSpec{}

		// Get intersection of gitiles commit fields.
		aGitilesCommit := aBuildSpec.GetGitilesCommit()
		bGitilesCommit := bBuildSpec.GetGitilesCommit()
		if aGitilesCommit != nil && bGitilesCommit != nil {
			cgc := &cpb.GitilesCommit{}
			if aGitilesCommit.GetProject() == bGitilesCommit.GetProject() && aGitilesCommit.GetId() == bGitilesCommit.GetId() {
				cgc.Project = aGitilesCommit.GetProject()
				cgc.Id = aGitilesCommit.GetId()
				cBuildSpec.GitilesCommit = cgc
			}
		}

		aGerritChanges := aBuildSpec.GetGerritChanges()
		bGerritChanges := bBuildSpec.GetGerritChanges()
		cGerritChanges := []*cpb.GerritChange{}
		if aGerritChanges != nil && bGerritChanges != nil {
			for j, aGerritChange := range aGerritChanges {
				if j >= len(bGerritChanges) {
					break
				}
				bGerritChange := bGerritChanges[j]
				if aGerritChange.GetProject() == bGerritChange.GetProject() && aGerritChange.GetPatchsetHash() == bGerritChange.GetPatchsetHash() {
					cGerritChanges = append(cGerritChanges, &cpb.GerritChange{
						Project:      aGerritChange.GetProject(),
						PatchsetHash: aGerritChange.GetPatchsetHash(),
					})
				}
			}
		}

		if len(cGerritChanges) > 0 {
			cBuildSpec.GerritChanges = cGerritChanges
		}

		if cBuildSpec.GitilesCommit != nil || len(cBuildSpec.GerritChanges) > 0 {
			ret = append(ret, cBuildSpec)
		}
	}
	return ret
}

// Returns a BuildSpec proto containing field values that are set in a but not b.
func diffBuildSpecs(a, b []*cpb.BuildSpec) []*cpb.BuildSpec {
	ret := []*cpb.BuildSpec{}
	for i, aBuildSpec := range a {
		if i >= len(b) {
			ret = append(ret, aBuildSpec)
			continue
		}
		bBuildSpec := b[i]
		dBuildSpec := &cpb.BuildSpec{}

		// Get intersection of gitiles commit fields.
		aGitilesCommit := aBuildSpec.GetGitilesCommit()
		bGitilesCommit := bBuildSpec.GetGitilesCommit()
		if aGitilesCommit != nil || bGitilesCommit != nil {
			dgc := &cpb.GitilesCommit{}
			if aGitilesCommit.GetProject() != bGitilesCommit.GetProject() {
				dgc.Project = aGitilesCommit.GetProject()
				dBuildSpec.GitilesCommit = dgc
			}
			if aGitilesCommit.GetId() != bGitilesCommit.GetId() {
				dgc.Id = aGitilesCommit.GetId()
				dBuildSpec.GitilesCommit = dgc
			}
		}

		aGerritChanges := aBuildSpec.GetGerritChanges()
		bGerritChanges := bBuildSpec.GetGerritChanges()
		dGerritChanges := []*cpb.GerritChange{}
		if aGerritChanges != nil || bGerritChanges != nil {
			for j, aGerritChange := range aGerritChanges {
				if j >= len(bGerritChanges) {
					dGerritChanges = append(dGerritChanges, aGerritChange)
					continue
				}
				bGerritChange := bGerritChanges[j]
				dGerritChange := &cpb.GerritChange{}
				if aGerritChange.GetProject() != bGerritChange.GetProject() {
					dGerritChange.Project = aGerritChange.GetProject()
				}
				if aGerritChange.GetPatchsetHash() != bGerritChange.GetPatchsetHash() {
					dGerritChange.PatchsetHash = aGerritChange.GetPatchsetHash()
					// Even if the projects are the same, if the hash is different, still include the Project.
					// This makes the diff'd BuildSpec more useful, since otherwise it would just give you
					// a patch without identifying which project (therefore which git repo) it came from.
					dGerritChange.Project = aGerritChange.GetProject()
				}
				dGerritChanges = append(dGerritChanges, dGerritChange)
			}
		}

		if len(dGerritChanges) > 0 {
			dBuildSpec.GerritChanges = dGerritChanges
		}

		if dBuildSpec.GitilesCommit != nil || len(dBuildSpec.GerritChanges) > 0 {
			ret = append(ret, dBuildSpec)
		}
	}
	return ret
}

// Returns a RunSpec proto containing field values that are common between a and b.
func intersectRunSpecs(a, b []*cpb.RunSpec) []*cpb.RunSpec {
	ret := []*cpb.RunSpec{}
	for i, aRunSpec := range a {
		if i >= len(b) {
			break
		}
		bRunSpec := b[i]
		cRunSpec := &cpb.RunSpec{}
		if aRunSpec.GetOs() == bRunSpec.GetOs() {
			cRunSpec.Os = aRunSpec.GetOs()
		}
		if aRunSpec.GetSyntheticProductName() == bRunSpec.GetSyntheticProductName() {
			cRunSpec.SyntheticProductName = aRunSpec.GetSyntheticProductName()
		}
		if aRunSpec.FinchConfig != nil && bRunSpec.FinchConfig != nil {
			aFinchConfig := aRunSpec.GetFinchConfig()
			bFinchConfig := bRunSpec.GetFinchConfig()
			cFinchConfig := &cpb.FinchConfig{}
			if aFinchConfig.GetSeedHash() != "" && aFinchConfig.GetSeedHash() == bFinchConfig.GetSeedHash() {
				cFinchConfig.SeedHash = aFinchConfig.GetSeedHash()
				cRunSpec.FinchConfig = cFinchConfig
			}
			if aFinchConfig.GetSeedChangelist() != 0 && aFinchConfig.GetSeedChangelist() == bFinchConfig.GetSeedChangelist() {
				cFinchConfig.SeedChangelist = aFinchConfig.GetSeedChangelist()
				cRunSpec.FinchConfig = cFinchConfig
			}
		}

		if cRunSpec.FinchConfig != nil || cRunSpec.SyntheticProductName != "" || cRunSpec.Os != "" {
			ret = append(ret, cRunSpec)
		}
	}
	return ret
}

// Returns a RunSpec proto containing field values that are set in a but not in b.
func diffRunSpecs(a, b []*cpb.RunSpec) []*cpb.RunSpec {
	ret := []*cpb.RunSpec{}
	for i, aRunSpec := range a {
		if i >= len(b) {
			ret = append(ret, aRunSpec)
			continue
		}
		bRunSpec := b[i]
		dRunSpec := &cpb.RunSpec{}
		if aRunSpec.GetOs() != bRunSpec.GetOs() {
			dRunSpec.Os = aRunSpec.GetOs()
		}
		if aRunSpec.GetSyntheticProductName() != bRunSpec.GetSyntheticProductName() {
			dRunSpec.SyntheticProductName = aRunSpec.GetSyntheticProductName()
		}
		if aRunSpec.FinchConfig != nil || bRunSpec.FinchConfig != nil {
			aFinchConfig := aRunSpec.GetFinchConfig()
			bFinchConfig := bRunSpec.GetFinchConfig()
			cFinchConfig := &cpb.FinchConfig{}
			if aFinchConfig.GetSeedHash() != "" && aFinchConfig.GetSeedHash() != bFinchConfig.GetSeedHash() {
				cFinchConfig.SeedHash = aFinchConfig.GetSeedHash()
				dRunSpec.FinchConfig = cFinchConfig
			}
			if aFinchConfig.GetSeedChangelist() != 0 && aFinchConfig.GetSeedChangelist() != bFinchConfig.GetSeedChangelist() {
				cFinchConfig.SeedChangelist = aFinchConfig.GetSeedChangelist()
				dRunSpec.FinchConfig = cFinchConfig
			}
		}

		if dRunSpec.FinchConfig != nil || dRunSpec.SyntheticProductName != "" || dRunSpec.Os != "" {
			ret = append(ret, dRunSpec)
		}
	}
	return ret
}

func fromKeys(in map[string]perfresults.PerfResults) util.StringSet {
	ret := util.StringSet{}
	for key := range in {
		ret[key] = true
	}
	return ret
}

// returns a map of benchmark names to sets of histogram names.  A histogram name is only included
// if *every* task in controlTaskResults and treatmentTaskResults reported a non-empty set of sample values under that histogram name.
func commonBenchmarkWorkloads(controlTaskResults, treatmentTaskResults []map[string]perfresults.PerfResults) (map[string]util.StringSet, error) {
	// Only try to analyze benchmarks and histograms that appear in data from all tasks.
	commonBenchmarks := util.StringSet{}
	commonHistograms := map[string]util.StringSet{}
	for i, controlResults := range controlTaskResults {
		if i >= len(treatmentTaskResults) {
			return nil, fmt.Errorf("missing treatment task result: %d", i)
		}
		treatmentResults := treatmentTaskResults[i]
		pairCommonBenchmarks := fromKeys(controlResults).Intersect(fromKeys(treatmentResults))
		if i == 0 {
			commonBenchmarks = pairCommonBenchmarks
		}
		commonBenchmarks = commonBenchmarks.Intersect(pairCommonBenchmarks)

		for benchmarkName, results := range controlResults {
			if commonHistograms[benchmarkName] == nil {
				commonHistograms[benchmarkName] = util.NewStringSet(results.NonEmptyHistogramNames())
			}
			commonHistograms[benchmarkName] = commonHistograms[benchmarkName].Intersect(util.NewStringSet(results.NonEmptyHistogramNames()))
		}
		for benchmarkName, results := range treatmentResults {
			if commonHistograms[benchmarkName] == nil {
				commonHistograms[benchmarkName] = util.NewStringSet(results.NonEmptyHistogramNames())
			}
			commonHistograms[benchmarkName] = commonHistograms[benchmarkName].Intersect(util.NewStringSet(results.NonEmptyHistogramNames()))
		}
	}

	for benchmarkName, histogramNames := range commonHistograms {
		if len(histogramNames) == 0 {
			delete(commonHistograms, benchmarkName)
		}
	}
	return commonHistograms, nil
}

// This parses the "change:..." tag strings generated and added to the swarming task requests in
// this part of the pinpoint source (which really should be conveyed in a more structured way so
// we don't have to resort to hand-written parsing code like this on the receiving end):
// https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/change/change.py;l=52
func buildSpecForChangeString(s string) (*cpb.BuildSpec, error) {
	changeParts := strings.Split(s, ":")
	if len(changeParts) < 2 || (changeParts[0] != "exp" && changeParts[0] != "base") {
		return nil, fmt.Errorf("failed to parse buildspec from change tag: %q", s)
	}

	// changeParts = "exp", "project@commit_hash + patch_id (args) (Variant: 0)"
	buildParts := strings.Split(strings.Join(changeParts[1:], ":"), "+")

	// buildParts = "project@commit_hash", "patch_id (args) (Variant: 0)"
	commitParts := strings.Split(buildParts[0], "@")

	// commitParts = "project", "commit_hash"
	if len(commitParts) != 2 {
		return nil, fmt.Errorf("failed to parse commit parts from change tag: %q", s)
	}
	repoProject := strings.TrimSpace(commitParts[0])

	gitHashPlusExtraParts := strings.Split(commitParts[1], " ")
	gitHash := strings.TrimSpace(gitHashPlusExtraParts[0])

	ret := &cpb.BuildSpec{
		GitilesCommit: &cpb.GitilesCommit{
			Project: repoProject,
			Id:      gitHash,
		},
	}

	if len(buildParts) == 2 {
		gerritPactchsetHash := strings.TrimSpace(strings.Split(strings.TrimSpace(buildParts[1]), " ")[0])
		// This value is the git hash of the patchset, without reference to the actual
		// gerrit change ID or which patchset on that change we're talking about.
		// Need to rethink this, either update pinpoint's code to put all of the data we need
		// into the swarming tags, or resign to using an opaque "applied git patch" string and
		// forget about gerrit's details.
		ret.GerritChanges = []*cpb.GerritChange{
			{
				PatchsetHash: gerritPactchsetHash,
			},
		}
	}

	return ret, nil
}

// Returns an ArmSpec proto populated with fields matching the details of s.
func inferArmSpec(s *apipb.TaskRequestMetadataResponse) (*cpb.ArmSpec, error) {
	ret := &cpb.ArmSpec{}

	ppc := pinpointChangeTagForTask(s)
	if ppc != "" {
	} else {
		sklog.Errorf("couldn't get pinpoint change info for a pinpoint task. Swarming ID %s", s.TaskId)
	}
	bs, err := buildSpecForChangeString(ppc)
	if err != nil {
		return nil, err
	}

	ret.BuildSpec = []*cpb.BuildSpec{bs}

	runInfo, err := runInfoForTask(s)
	if err != nil {
		return nil, err
	}

	ret.RunSpec = []*cpb.RunSpec{
		{
			Os:                   runInfo.os,
			SyntheticProductName: runInfo.syntheticProductName,
		},
	}

	return ret, nil
}

// Because we don't *currently* have users specify up-front what the ExperimentSpec should be
// (they just give us a pinpoint job ID, rather than telling us the actual build/run details),
// we do a bit of inference here to reconstruct that information from what we have in the
// available swarming task metadata.
func inferExperimentSpec(controlSpecs, treatmentSpecs []*cpb.ArmSpec, controlResults, treatmentResults []map[string]perfresults.PerfResults) (*cpb.ExperimentSpec, error) {
	if len(controlSpecs) != len(treatmentSpecs) || len(controlSpecs) == 0 || len(treatmentSpecs) == 0 {
		return nil, fmt.Errorf("control and treatment spec length must be equal and non-zero: %d vs %d", len(controlSpecs), len(treatmentSpecs))
	}

	ret := &cpb.ExperimentSpec{}

	// accumulate the common Spec proto field values that are identical across all tasks within three
	// subsets of tasks in the experiment data:
	// - commonArmSpecIntersection for Spec proto fields that are the same across all tasks
	// - controlArmSpecIntersection for Spec proto files that are the same across all control tasks
	// - treatmentArmSpecIntersection for Spec proto fields that are the same across all treatment tasks
	controlArmSpecIntersection := controlSpecs[0]
	treatmentArmSpecIntersection := treatmentSpecs[0]
	commonArmSpecIntersection := intersectArmSpecs(controlArmSpecIntersection, treatmentArmSpecIntersection)

	for _, cArmSpec := range controlSpecs[1:] {
		controlArmSpecIntersection = intersectArmSpecs(controlArmSpecIntersection, cArmSpec)
		commonArmSpecIntersection = intersectArmSpecs(commonArmSpecIntersection, cArmSpec)
	}

	for _, tArmSpec := range treatmentSpecs[1:] {
		treatmentArmSpecIntersection = intersectArmSpecs(treatmentArmSpecIntersection, tArmSpec)
		commonArmSpecIntersection = intersectArmSpecs(commonArmSpecIntersection, tArmSpec)
	}

	// Now remove the Spec proto fields that are common to both arms from each arms' CommonArmSpec
	// so that they only reflect the differences between control and treatment relative to the attributes
	// that are common between them.
	controlArmSpecIntersection = diffArmSpecs(controlArmSpecIntersection, commonArmSpecIntersection)
	treatmentArmSpecIntersection = diffArmSpecs(treatmentArmSpecIntersection, commonArmSpecIntersection)

	// We only need to infer *common* benchmark/workload measurement values (no diffs) reported by both
	// arms' tasks, because there's no way to compare response variables that don't appear in both arms.
	// So we just ignore values that do not appear in every tasks' output files.
	//
	// Note that in practice, many jobs produce disjoint sets of "metrics", because they report
	// things that are not actual response variables (e.g. optional diagnostic info used for debugging)
	// that just happen to use the same data format used by response variables in their json files. Ignoring
	// any of these "metrics" that do not appear in every task output is an admittedly coarse heuristic,
	// but a scalable solution requires either cleaner benchmark output files, or more explicit
	// analysis requests that enumerate the exact benchmark/workloads to look for (neither of which
	// is something expect to have by 2023Q2).
	commonHistograms, err := commonBenchmarkWorkloads(controlResults, treatmentResults)
	if err != nil {
		return nil, err
	}
	benchmarks := []*cpb.Benchmark{}

	for benchmarkName, histograms := range commonHistograms {
		workloads := histograms.Keys()
		sort.Strings(workloads)
		benchmarks = append(benchmarks, &cpb.Benchmark{
			Name:     benchmarkName,
			Workload: workloads,
		})
	}
	ret.Analysis = &cpb.AnalysisSpec{
		Benchmark: benchmarks,
	}
	ret.Common = commonArmSpecIntersection
	ret.Control = controlArmSpecIntersection
	ret.Treatment = treatmentArmSpecIntersection

	return ret, nil
}
