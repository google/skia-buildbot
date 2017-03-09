package testutils

import (
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
)

const (
	BuildTask = "Build-Ubuntu-GCC-Arm7-Release-Android"
	TestTask  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	PerfTask  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
)

var (
	depotToolsMtx sync.Mutex
)

// GetDepotTools returns the path to depot_tools, syncing it if necessary.
func GetDepotTools(t *testing.T) string {
	depotToolsMtx.Lock()
	defer depotToolsMtx.Unlock()

	found, err := depot_tools.Find()
	if err == nil {
		return found
	}

	// Sync to a special location.
	workdir := path.Join(os.TempDir(), "sktest_depot_tools")
	c, err := git.NewCheckout(common.REPO_DEPOT_TOOLS, workdir)
	assert.NoError(t, err)
	assert.NoError(t, c.Update())
	return c.Dir()
}

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
func SetupTestRepo(t *testing.T) (*git_testutils.GitBuilder, string, string) {
	gb := git_testutils.GitInit(t)

	// Commit 1.
	gb.Add("infra/bots/compile_skia.isolate", `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add("infra/bots/perf_skia.isolate", `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add("infra/bots/swarm_recipe.isolate", `{
  'variables': {
    'command': [
      'python', 'recipes.py', 'run',
    ],
    'files': [
      '../../somefile.txt',
    ],
  },
}`)
	gb.Add("infra/bots/tasks.json", `{
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "cipd_packages": [{
        "name": "android_sdk",
        "path": "android_sdk",
        "version": "version:0"
      }],
      "dimensions": ["pool:Skia", "os:Ubuntu"],
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
	gb.Add("infra/bots/test_skia.isolate", `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	gb.Add("somefile.txt", "blahblah")
	gb.Add("a.txt", "blah")
	now := time.Now()
	c1 := gb.CommitMsgAt("c1", now.Add(-5*time.Second))

	// Commit 2.
	gb.Add("infra/bots/tasks.json", `{
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
	c2 := gb.CommitMsgAt("c2", now)

	return gb, c1, c2
}
