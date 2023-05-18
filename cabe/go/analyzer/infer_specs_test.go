package analyzer

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"go.skia.org/infra/cabe/go/perfresults"
	cpb "go.skia.org/infra/cabe/go/proto"
	"go.skia.org/infra/go/util"
)

func TestBuildSpecForChangeString(t *testing.T) {
	for name, test := range map[string]struct {
		changeString string
		want         *cpb.BuildSpec
		wantErr      bool
	}{
		"invalid change prefix": {
			"not a valid change string",
			nil,
			true,
		},
		"valid prefix but invalid remainder": {
			"base: this is still not a parseable change string",
			nil,
			true,
		},
		"exp, commit plus gitiles patch hash": {
			"exp: chromium@d879632 + 0dd4ae0",
			&cpb.BuildSpec{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "d879632",
				},
				GerritChanges: []*cpb.GerritChange{
					{
						PatchsetHash: "0dd4ae0",
					},
				},
			},
			false,
		},
		"exp, commit with extra args and variant": {
			"exp: chromium@cdc19e6 (--disable-features=MojoTaskPerMessage) (Variant: 1)",
			&cpb.BuildSpec{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "cdc19e6",
				},
			},
			false,
		},
		"base, commit with variant": {
			"base: chromium@cdc19e6 (Variant: 0)",
			&cpb.BuildSpec{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "cdc19e6",
				},
			},
			false,
		},
		"base, commit plus gerrit patch, with variant": {
			"base: chromium@cdc19e6 + 0dd4ae0 (Variant: 0)",
			&cpb.BuildSpec{
				GitilesCommit: &cpb.GitilesCommit{
					Project: "chromium",
					Id:      "cdc19e6",
				},
				GerritChanges: []*cpb.GerritChange{
					{
						PatchsetHash: "0dd4ae0",
					},
				},
			},
			false,
		},
	} {
		got, err := buildSpecForChangeString(test.changeString)
		if err != nil && !test.wantErr {
			t.Errorf("%q: unexpected error: %v\n", name, err)
		} else if err == nil && test.wantErr {
			t.Errorf("%q: did not get expected error", name)
		}
		if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}

func TestIntersectBuildSpecs(t *testing.T) {
	for name, test := range map[string]struct {
		a, b []*cpb.BuildSpec
		want []*cpb.BuildSpec
	}{
		"empty": {
			a:    []*cpb.BuildSpec{},
			b:    []*cpb.BuildSpec{},
			want: []*cpb.BuildSpec{},
		},
		"identical": {
			a: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
			b: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
			want: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
		},
		"different gitiles commit IDs": {
			a: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
			b: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "0000000",
					},
				},
			},
			want: nil,
		},
		"base commit in both, gerrit patch in one": {
			a: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
			b: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
					GerritChanges: []*cpb.GerritChange{
						{
							PatchsetHash: "1234567",
						},
					},
				},
			},
			want: []*cpb.BuildSpec{
				{
					GitilesCommit: &cpb.GitilesCommit{
						Project: "chromium",
						Id:      "cdc19e6",
					},
				},
			},
		},
	} {
		got := intersectBuildSpecs(test.a, test.b)
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty(), protocmp.Transform()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}

func TestCommonBenchmarkWorkloads(t *testing.T) {
	for name, test := range map[string]struct {
		a, b []map[string]perfresults.PerfResults
		want map[string]util.StringSet
	}{
		"empty": {},
		"identical, but empty sample values": {
			a: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{},
							},
						},
					},
				},
			},
			b: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{},
							},
						},
					},
				},
			},
			want: nil,
		},
		"identical, non-empty sample values": {
			a: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			b: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			want: map[string]util.StringSet{
				"benchmark 0": util.NewStringSet([]string{"workload 0"}),
			},
		},
		"disjoint, non-empty sample values": {
			a: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{42},
							},
							{
								Name:         "workload 2",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			b: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{42},
							},
							{
								Name:         "workload 1",
								SampleValues: []float64{42},
							},
							{
								Name:         "workload 2",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			want: map[string]util.StringSet{
				"benchmark 0": util.NewStringSet([]string{"workload 0", "workload 2"}),
			},
		},
		"disjoint, partially-empty sample values": {
			a: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{},
							},
							{
								Name:         "workload 2",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			b: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": {
						Histograms: []perfresults.Histogram{
							{
								Name:         "workload 0",
								SampleValues: []float64{42},
							},
							{
								Name:         "workload 1",
								SampleValues: []float64{42},
							},
							{
								Name:         "workload 2",
								SampleValues: []float64{42},
							},
						},
					},
				},
			},
			want: map[string]util.StringSet{
				"benchmark 0": util.NewStringSet([]string{"workload 2"}),
			},
		},
	} {
		got, err := commonBenchmarkWorkloads(test.a, test.b)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", name, err)
		}
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}

func TestIntersectRunSpecs(t *testing.T) {
	for name, test := range map[string]struct {
		a, b []*cpb.RunSpec
		want []*cpb.RunSpec
	}{
		"empty": {
			a:    []*cpb.RunSpec{},
			b:    []*cpb.RunSpec{},
			want: []*cpb.RunSpec{},
		},
		"identical": {
			a: []*cpb.RunSpec{
				{
					Os: "linux",
				},
			},
			b: []*cpb.RunSpec{
				{
					Os: "linux",
				},
			},
			want: []*cpb.RunSpec{
				{
					Os: "linux",
				},
			},
		},
		"different Finch seeds": {
			a: []*cpb.RunSpec{
				{
					Os: "linux",
					FinchConfig: &cpb.FinchConfig{
						SeedHash: "123",
					},
				},
			},
			b: []*cpb.RunSpec{
				{
					Os: "linux",
					FinchConfig: &cpb.FinchConfig{
						SeedHash: "345",
					},
				},
			},
			want: []*cpb.RunSpec{
				{
					Os: "linux",
				},
			},
		},
	} {
		got := intersectRunSpecs(test.a, test.b)
		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty(), protocmp.Transform()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}

func TestIntersectArmSpecs(t *testing.T) {
	for name, test := range map[string]struct {
		a, b, want *cpb.ArmSpec
	}{
		"empty": {
			a:    &cpb.ArmSpec{},
			b:    &cpb.ArmSpec{},
			want: &cpb.ArmSpec{},
		},
		"identical": {
			a: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
			b: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
			want: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
		},
		"different gitiles commit IDs": {
			a: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
			b: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "0000000",
						},
					},
				},
			},
			want: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{},
			},
		},
		"base commit in both, gerrit patch in one": {
			a: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
			b: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
						GerritChanges: []*cpb.GerritChange{
							{
								PatchsetHash: "1234567",
							},
						},
					},
				},
			},
			want: &cpb.ArmSpec{
				BuildSpec: []*cpb.BuildSpec{
					{
						GitilesCommit: &cpb.GitilesCommit{
							Project: "chromium",
							Id:      "cdc19e6",
						},
					},
				},
			},
		},
	} {
		got := intersectArmSpecs(test.a, test.b)
		if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}

func TestInferExperimentSpec(t *testing.T) {
	for name, test := range map[string]struct {
		controlArmSpecs, treatmentArmSpecs []*cpb.ArmSpec
		controlResults, treatmentResults   []map[string]perfresults.PerfResults
		want                               *cpb.ExperimentSpec
		wantError                          bool
	}{
		"empty": {
			want:      nil,
			wantError: true,
		},
		"identical gitiles commits, e.g. an A/A test": {
			controlArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
			},
			treatmentArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
			},
			controlResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{1, 2, 3}},
							{Name: "workload 1", SampleValues: []float64{1, 2, 3}},
						},
					},
				},
			},
			treatmentResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{4, 5, 6}},
							{Name: "workload 1", SampleValues: []float64{4, 5, 6}},
						},
					},
				},
			},
			want: &cpb.ExperimentSpec{
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     "benchmark 0",
							Workload: []string{"workload 0", "workload 1"},
						},
					},
				},
				Common: &cpb.ArmSpec{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
				Control:   &cpb.ArmSpec{},
				Treatment: &cpb.ArmSpec{},
			},
			wantError: false,
		},
		"different gitiles commits without gerrit patches, e.g. a bisection test": {
			controlArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
			},
			treatmentArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "decafbad",
							},
						},
					},
				},
			},
			controlResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{1, 2, 3}},
							{Name: "workload 1", SampleValues: []float64{1, 2, 3}},
						},
					},
				},
			},
			treatmentResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{4, 5, 6}},
							{Name: "workload 1", SampleValues: []float64{4, 5, 6}},
						},
					},
				},
			},
			want: &cpb.ExperimentSpec{
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     "benchmark 0",
							Workload: []string{"workload 0", "workload 1"},
						},
					},
				},
				Control: &cpb.ArmSpec{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
				Treatment: &cpb.ArmSpec{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "decafbad",
							},
						},
					},
				},
				Common: &cpb.ArmSpec{},
			},
			wantError: false,
		},
		"same gitiles commit but treatment arm has gerrit patch, e.g. an A/B tyjob": {
			controlArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
			},
			treatmentArmSpecs: []*cpb.ArmSpec{
				{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
							GerritChanges: []*cpb.GerritChange{
								{
									PatchsetHash: "abcd",
								},
							},
						},
					},
				},
			},
			controlResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{1, 2, 3}},
							{Name: "workload 1", SampleValues: []float64{1, 2, 3}},
						},
					},
				},
			},
			treatmentResults: []map[string]perfresults.PerfResults{
				{
					"benchmark 0": perfresults.PerfResults{
						Histograms: []perfresults.Histogram{
							{Name: "workload 0", SampleValues: []float64{4, 5, 6}},
							{Name: "workload 1", SampleValues: []float64{4, 5, 6}},
						},
					},
				},
			},
			want: &cpb.ExperimentSpec{
				Analysis: &cpb.AnalysisSpec{
					Benchmark: []*cpb.Benchmark{
						{
							Name:     "benchmark 0",
							Workload: []string{"workload 0", "workload 1"},
						},
					},
				},
				Common: &cpb.ArmSpec{
					BuildSpec: []*cpb.BuildSpec{
						{
							GitilesCommit: &cpb.GitilesCommit{
								Project: "chromium",
								Id:      "cdc19e6",
							},
						},
					},
				},
				Control: &cpb.ArmSpec{},
				Treatment: &cpb.ArmSpec{
					BuildSpec: []*cpb.BuildSpec{
						{
							GerritChanges: []*cpb.GerritChange{
								{
									PatchsetHash: "abcd",
								},
							},
						}},
				},
			},
			wantError: false,
		},
	} {
		got, err := inferExperimentSpec(test.controlArmSpecs, test.treatmentArmSpecs, test.controlResults, test.treatmentResults)
		if err != nil && !test.wantError {
			t.Errorf("%q: unexpected error: %v\n", name, err)
		} else if err == nil && test.wantError {
			t.Errorf("%q: did not get expected error", name)
		}

		if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty(), protocmp.Transform()); diff != "" {
			t.Errorf("%q: unexpected return value: %s", name, diff)
		}
	}
}
