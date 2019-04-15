package testutils

import (
	"context"
	"time"

	bt_testutil "go.skia.org/infra/go/bt/testutil"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	BuildTaskName = "Build-Ubuntu-GCC-Arm7-Release-Android"
	TestTaskName  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	PerfTaskName  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
)

var (
	BuildTask = &specs.TaskSpec{
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
		Isolate:     "compile_skia.isolate",
		MaxAttempts: 5,
		Priority:    0.8,
	}
	TestTask = &specs.TaskSpec{
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
		Isolate:  "test_skia.isolate",
		Priority: 0.8,
	}
	PerfTask = &specs.TaskSpec{
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
		Isolate:      "perf_skia.isolate",
		Priority:     0.8,
	}

	BuildJob = &specs.JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{BuildTaskName},
	}
	TestJob = &specs.JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{TestTaskName},
	}
	PerfJob = &specs.JobSpec{
		Priority:  0.8,
		TaskSpecs: []string{PerfTaskName},
	}
	TasksCfg1 = &specs.TasksCfg{
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
func SetupTestRepo(t testutils.TestingT) (context.Context, *git_testutils.GitBuilder, string, string) {
	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)

	// Commit 1.
	gb.Add(ctx, "infra/bots/compile_skia.isolate", `{
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add(ctx, "infra/bots/perf_skia.isolate", `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add(ctx, "infra/bots/swarm_recipe.isolate", `{
  'variables': {
    'command': [
      'python', 'recipes.py', 'run',
    ],
    'files': [
      '../../somefile.txt',
    ],
  },
}`)
	gb.Add(ctx, "infra/bots/tasks.json", `{
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "cipd_packages": [{
        "name": "android_sdk",
        "path": "android_sdk",
        "version": "version:0"
      }],
      "command": ["ninja", "skia"],
      "dimensions": ["pool:Skia", "os:Ubuntu"],
      "extra_tags": {"is_build_task": "true"},
      "isolate": "compile_skia.isolate",
      "max_attempts": 5,
      "priority": 0.8
    },
    "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "cipd_packages": [{
        "name": "skimage",
        "path": "skimage",
        "version": "version:0"
      },
      {
        "name": "skp",
        "path": "skp",
        "version": "version:0"
      }],
      "dependencies": ["Build-Ubuntu-GCC-Arm7-Release-Android"],
      "dimensions": ["pool:Skia", "os:Android", "device_type:grouper"],
      "env_prefixes": {
        "PATH": ["curdir"]
      },
      "isolate": "test_skia.isolate",
      "priority": 0.8
    }
  },
  "jobs": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "priority": 0.8,
      "tasks": ["Build-Ubuntu-GCC-Arm7-Release-Android"]
    },
    "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "priority": 0.8,
      "tasks": ["Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"]
    }
  }
}`)
	gb.Add(ctx, "infra/bots/test_skia.isolate", `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add(ctx, "somefile.txt", "blahblah")
	gb.Add(ctx, "a.txt", "blah")
	now := time.Now()
	c1 := gb.CommitMsgAt(ctx, "c1", now.Add(-5*time.Second))

	// Commit 2.
	gb.Add(ctx, "infra/bots/tasks.json", `{
  "jobs": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "priority": 0.8,
      "tasks": [
        "Build-Ubuntu-GCC-Arm7-Release-Android"
      ]
    },
    "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "priority": 0.8,
      "tasks": [
        "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
      ]
    },
    "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "priority": 0.8,
      "tasks": [
        "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
      ]
    }
  },
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "cipd_packages": [
        {
          "name": "android_sdk",
          "path": "android_sdk",
          "version": "version:0"
        }
      ],
      "dimensions": [
        "pool:Skia",
        "os:Ubuntu"
      ],
      "isolate": "compile_skia.isolate",
      "max_attempts": 5,
      "priority": 0.8
    },
    "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "cipd_packages": [
        {
          "name": "skimage",
          "path": "skimage",
          "version": "version:0"
        },
        {
          "name": "skp",
          "path": "skp",
          "version": "version:0"
        }
      ],
      "dependencies": [
        "Build-Ubuntu-GCC-Arm7-Release-Android"
      ],
      "dimensions": [
        "pool:Skia",
        "os:Android",
        "device_type:grouper"
      ],
      "isolate": "perf_skia.isolate",
      "priority": 0.8
    },
    "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
      "cipd_packages": [
        {
          "name": "skimage",
          "path": "skimage",
          "version": "version:0"
        },
        {
          "name": "skp",
          "path": "skp",
          "version": "version:0"
        }
      ],
      "dependencies": [
        "Build-Ubuntu-GCC-Arm7-Release-Android"
      ],
      "dimensions": [
        "pool:Skia",
        "os:Android",
        "device_type:grouper"
      ],
      "isolate": "test_skia.isolate",
      "priority": 0.8
    }
  }
}`)
	c2 := gb.CommitMsgAt(ctx, "c2", now)

	return ctx, gb, c1, c2
}

// SetupBigTable performs setup for the TaskCfgCache in BigTable. Returns the
// BigTable instance name which should be used to instantiate TaskCfgCache and a
// cleanup function which should be deferred.
func SetupBigTable(t testutils.TestingT) (string, string, func()) {
	// The table and column family names are specs.BT_TABLE and
	// specs.BT_COLUMN_FAMILY, but are hard-coded here to avoid a dependency
	// cycle.
	return bt_testutil.SetupBigTable(t, "tasks-cfg", "CFGS")
}
