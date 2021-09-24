package testutils

import (
	"context"
	"time"

	bt_testutil "go.skia.org/infra/go/bt/testutil"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BuildTaskName = "Build-Ubuntu-GCC-Arm7-Release-Android"
	TestTaskName  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	PerfTaskName  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"

	CompileCASDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/42"
	TestCASDigest    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/42"
	PerfCASDigest    = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc/42"
)

var (
	BuildTask = &specs.TaskSpec{
		CasSpec: "compile",
		CipdPackages: []*specs.CipdPackage{
			{
				Name:    "android_sdk",
				Path:    "android_sdk",
				Version: "version:0",
			},
		},
		Command:    []string{"ninja", "skia"},
		Dimensions: []string{"pool:Skia", "os:Ubuntu"},
		ExtraTags: map[string]string{
			"is_build_task": "true",
		},
		MaxAttempts: 5,
		Priority:    0.5,
	}
	TestTask = &specs.TaskSpec{
		CasSpec: "test",
		CipdPackages: []*specs.CipdPackage{
			{
				Name:    "skimage",
				Path:    "skimage",
				Version: "version:0",
			},
			{
				Name:    "skp",
				Path:    "skp",
				Version: "version:0",
			},
		},
		Command:      []string{"test", "skia"},
		Dependencies: []string{BuildTaskName},
		Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
		EnvPrefixes: map[string][]string{
			"PATH": {"curdir"},
		},
		Priority: 0.5,
	}
	PerfTask = &specs.TaskSpec{
		CasSpec: "perf",
		CipdPackages: []*specs.CipdPackage{
			{
				Name:    "skimage",
				Path:    "skimage",
				Version: "version:0",
			},
			{
				Name:    "skp",
				Path:    "skp",
				Version: "version:0",
			},
		},
		Command:      []string{"perf", "skia"},
		Dependencies: []string{BuildTaskName},
		Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
		Priority:     0.5,
	}

	BuildJob = &specs.JobSpec{
		Priority:  0.5,
		TaskSpecs: []string{BuildTaskName},
	}
	TestJob = &specs.JobSpec{
		Priority:  0.5,
		TaskSpecs: []string{TestTaskName},
	}
	PerfJob = &specs.JobSpec{
		Priority:  0.5,
		TaskSpecs: []string{PerfTaskName},
	}
	CasSpecs1 = map[string]*specs.CasSpec{
		"compile": {
			Digest: CompileCASDigest,
		},
		"test": {
			Digest: TestCASDigest,
		},
	}
	CasSpecs2 = map[string]*specs.CasSpec{
		"compile": {
			Digest: CompileCASDigest,
		},
		"test": {
			Digest: TestCASDigest,
		},
		"perf": {
			Digest: PerfCASDigest,
		},
	}
	TasksCfg1 = &specs.TasksCfg{
		CasSpecs: CasSpecs1,
		Tasks: map[string]*specs.TaskSpec{
			BuildTaskName: BuildTask,
			TestTaskName:  TestTask,
		},
		Jobs: map[string]*specs.JobSpec{
			BuildTaskName: BuildJob,
			TestTaskName:  TestJob,
		},
	}
	TasksCfg2 = &specs.TasksCfg{
		CasSpecs: CasSpecs2,
		Tasks: map[string]*specs.TaskSpec{
			BuildTaskName: BuildTask,
			PerfTaskName:  PerfTask,
			TestTaskName:  TestTask,
		},
		Jobs: map[string]*specs.JobSpec{
			BuildTaskName: BuildJob,
			PerfTaskName:  PerfJob,
			TestTaskName:  TestJob,
		},
	}
)

// The test repo has two commits. The first commit adds a tasks.cfg file
// with two task specs: a build task and a test task, the test task
// depending on the build task. The second commit adds a perf task spec,
// which also depends on the build task. Therefore, there are five total
// possible tasks we could run:
//
// Build@c1, Test@c1, Build@c2, Test@c2, Perf@c2
//
// Returns the GitBuilder instance for the test repo, along with the commit
// hashes for c1 and c2.
func SetupTestRepo(t sktest.TestingT) (context.Context, *git_testutils.GitBuilder, string, string) {
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)

	// Commit 1.
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, TasksCfg1))
	gb.Add(ctx, "somefile.txt", "blahblah")
	gb.Add(ctx, "a.txt", "blah")
	now := time.Now()
	c1 := gb.CommitMsgAt(ctx, "c1", now.Add(-5*time.Second))

	// Commit 2.
	gb.Add(ctx, "infra/bots/tasks.json", testutils.MarshalIndentJSON(t, TasksCfg2))
	c2 := gb.CommitMsgAt(ctx, "c2", now)

	return ctx, gb, c1, c2
}

// SetupBigTable performs setup for the TaskCfgCache in BigTable. Returns the
// BigTable instance name which should be used to instantiate TaskCfgCache and a
// cleanup function which should be deferred.
func SetupBigTable(t sktest.TestingT) (string, string, func()) {
	// The table and column family names are task_cfg_cache.BT_TABLE and
	// task_cfg_cache.BT_COLUMN_FAMILY, but are hard-coded here to avoid a
	// dependency cycle.
	return bt_testutil.SetupBigTable(t, "tasks-cfg", "CFGS")
}
