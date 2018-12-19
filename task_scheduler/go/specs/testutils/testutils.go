package testutils

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

const (
	BuildTask = "Build-Ubuntu-GCC-Arm7-Release-Android"
	TestTask  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	PerfTask  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"

	BT_PROJECT = "test-project"
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
// project and instance names which should be used to instantiate TaskCfgCache
// and a cleanup function which should be deferred.
func SetupBigTable(t testutils.TestingT) (string, string, func()) {
	// The table and column family names are specs.BT_TABLE and
	// specs.BT_COLUMN_FAMILY, but are hard-coded here to avoid a dependency
	// cycle.
	cfg := bt.TableConfig{
		"tasks-cfg": {
			"CFGS",
		},
	}
	instance := fmt.Sprintf("specs-testutils-%s", uuid.New())
	assert.NoError(t, bt.InitBigtable(BT_PROJECT, instance, cfg))
	return BT_PROJECT, instance, func() {
		assert.NoError(t, bt.DeleteTables(BT_PROJECT, instance, cfg))
	}
}
