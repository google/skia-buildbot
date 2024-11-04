package cacher

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cas/mocks"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/exec"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/gitiles"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/syncer"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
)

func setup(t *testing.T) (context.Context, *CacherImpl, task_cfg_cache.TaskCfgCache, *mocks.CAS, types.RepoState, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *exec.CommandCollector, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	const gitPath = "/path/to/fake/git"
	ctx = git_common.WithGitFinder(ctx, func() (string, error) {
		return gitPath, nil
	})
	mockExec := &exec.CommandCollector{}
	ctx = exec.NewContext(ctx, mockExec.Run)

	rs := types.RepoState{
		Repo:     "fake/repo.git",
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	mockExec.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if err := git_common.MocksForFindGit(ctx, cmd); err != nil {
			return err
		}
		if cmd.Name == gitPath {
			// "rev-parse" should be the first argument, but since we're not
			// actually syncing the repo, the .git dir doesn't exist and
			// therefore our code is tricked into thinking we're using a bare
			// clone and adds the "--git-dir=." arg first.
			if util.In("rev-parse", cmd.Args) {
				_, err := cmd.CombinedOutput.Write([]byte(rs.Revision))
				return err
			}
		}
		if strings.Contains(cmd.Name, "patch") {
			// We have to actually run this command in order to apply the patch
			// from Gerrit.
			return exec.DefaultRun(ctx, cmd)
		}
		return nil
	})

	wd, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	s := syncer.New(ctx, "", wd, 1)
	btProject, btInstance, btCleanup := tcc_testutils.SetupBigTable(t)
	tcc, err := task_cfg_cache.NewTaskCfgCache(ctx, nil, btProject, btInstance, nil)
	require.NoError(t, err)
	cas := &mocks.CAS{}
	mockRepo := &gitiles_mocks.GitilesRepo{}
	mockGerrit := &gerrit_mocks.GerritInterface{}
	c := New(s, tcc, cas, map[string]gitiles.GitilesRepo{
		rs.Repo: mockRepo,
	}, mockGerrit)
	return ctx, c, tcc, cas, rs, mockRepo, mockGerrit, mockExec, func() {
		testutils.RemoveAll(t, wd)
		btCleanup()
		cancel()
	}
}

// didSync returns true iff we performed a "gclient sync".
func didSync(mockExec *exec.CommandCollector) bool {
	for _, cmd := range mockExec.Commands() {
		if len(cmd.Args) > 2 && strings.Contains(cmd.Args[1], "gclient") && cmd.Args[2] == "sync" {
			return true
		}
	}
	return false
}

func TestGetOrCacheRepoState_AlreadySet(t *testing.T) {
	ctx, c, tcc, _, rs, _, _, mockExec, cleanup := setup(t)
	defer cleanup()

	// Insert entries into the task config cache.
	expect := &specs.TasksCfg{
		Tasks: map[string]*specs.TaskSpec{
			"task": {},
		},
		Jobs: map[string]*specs.JobSpec{
			"job": {},
		},
	}
	require.NoError(t, tcc.Set(ctx, rs, expect, nil))

	// Retrieve the cached value.
	actual, err := c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)
	assertdeep.Equal(t, expect, actual)

	// Verify that we didn't sync.
	require.False(t, didSync(mockExec))
}

func TestGetOrCacheRepoState_NoPatch_Sync(t *testing.T) {
	ctx, c, tcc, cas, rs, mockGitiles, _, mockExec, cleanup := setup(t)
	defer cleanup()

	// The test TasksCfg contains CasSpecs that are already resolved. Un-resolve
	// them to force a sync.
	tasksCfg := tcc_testutils.TasksCfg2.Copy()
	tasksCfg.CasSpecs = map[string]*specs.CasSpec{
		"my-cas": {
			Root:     ".",
			Paths:    []string{"somefile.txt"},
			Excludes: []string{rbe.ExcludeGitDir},
		},
	}
	for _, task := range tasksCfg.Tasks {
		task.CasSpec = "my-cas"
	}
	tasksJson := testutils.MarshalIndentJSON(t, tasksCfg)

	// Verify that the cache entry doesn't exist yet.
	cached, cachedErr, err := tcc.Get(ctx, rs)
	require.EqualError(t, task_cfg_cache.ErrNoSuchEntry, err.Error())
	require.NoError(t, cachedErr)
	require.Nil(t, cached)

	// Set up mocks.
	mockGitiles.On("ReadFileAtRef", testutils.AnyContext, specs.TASKS_CFG_FILE, rs.Revision).Return([]byte(tasksJson), nil)
	cas.On("Upload", testutils.AnyContext, mock.AnythingOfType("string"), []string{"somefile.txt"}, []string{rbe.ExcludeGitDir}).Return("fake-digest", nil)

	// Run GetOrCacheRepoState to populate the cache.
	_, err = c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)

	// Verify that the cache entry now exists.
	cached, cachedErr, err = tcc.Get(ctx, rs)
	require.NoError(t, err)
	require.NoError(t, cachedErr)
	require.NotNil(t, cached)
	require.Equal(t, "fake-digest", cached.CasSpecs["my-cas"].Digest)

	// Verify that we synced.
	require.True(t, didSync(mockExec))
}

func TestGetOrCacheRepoState_HasPatch_TasksCfgNotModified_NoSync(t *testing.T) {
	ctx, c, tcc, _, rs, mockGitiles, mockGerrit, mockExec, cleanup := setup(t)
	defer cleanup()

	// Add a patch to the RepoState.
	const issue = int64(12345)
	rs.Patch = types.Patch{
		Issue:    strconv.FormatInt(issue, 10),
		Patchset: "1",
		Server:   "fake-gerrit-server",
	}

	// The test TasksCfg contains CasSpecs which are already resolved, so we
	// won't need a sync by default.
	tasksJson := testutils.MarshalIndentJSON(t, tcc_testutils.TasksCfg2)

	// Verify that the cache entry doesn't exist yet.
	cached, cachedErr, err := tcc.Get(ctx, rs)
	require.EqualError(t, task_cfg_cache.ErrNoSuchEntry, err.Error())
	require.NoError(t, cachedErr)
	require.Nil(t, cached)

	// Set up mocks.
	mockGitiles.On("ReadFileAtRef", testutils.AnyContext, specs.TASKS_CFG_FILE, rs.Revision).Return([]byte(tasksJson), nil)
	mockGerrit.On("GetFileNames", testutils.AnyContext, issue, rs.Patchset).Return([]string{"blahblah.txt"}, nil)

	// Run GetOrCacheRepoState to populate the cache.
	_, err = c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)

	// Verify that the cache entry now exists.
	cached, cachedErr, err = tcc.Get(ctx, rs)
	require.NoError(t, err)
	require.NoError(t, cachedErr)
	require.NotNil(t, cached)

	// Verify that we didn't sync.
	require.False(t, didSync(mockExec))
}

func TestGetOrCacheRepoState_HasPatch_TasksCfgNotModified_Sync(t *testing.T) {
	ctx, c, tcc, cas, rs, mockGitiles, mockGerrit, mockExec, cleanup := setup(t)
	defer cleanup()

	// Add a patch to the RepoState.
	const issue = int64(12345)
	rs.Patch = types.Patch{
		Issue:    strconv.FormatInt(issue, 10),
		Patchset: "1",
		Server:   "fake-gerrit-server",
	}

	// The test TasksCfg contains CasSpecs that are already resolved. Un-resolve
	// them to force a sync.
	tasksCfg := tcc_testutils.TasksCfg2.Copy()
	tasksCfg.CasSpecs = map[string]*specs.CasSpec{
		"my-cas": {
			Root:     ".",
			Paths:    []string{"somefile.txt"},
			Excludes: []string{rbe.ExcludeGitDir},
		},
	}
	for _, task := range tasksCfg.Tasks {
		task.CasSpec = "my-cas"
	}
	tasksJson := testutils.MarshalIndentJSON(t, tasksCfg)

	// Verify that the cache entry doesn't exist yet.
	cached, cachedErr, err := tcc.Get(ctx, rs)
	require.EqualError(t, task_cfg_cache.ErrNoSuchEntry, err.Error())
	require.NoError(t, cachedErr)
	require.Nil(t, cached)

	// Set up mocks.
	mockGitiles.On("ReadFileAtRef", testutils.AnyContext, specs.TASKS_CFG_FILE, rs.Revision).Return([]byte(tasksJson), nil)
	mockGerrit.On("GetFileNames", testutils.AnyContext, issue, rs.Patchset).Return([]string{"blahblah.txt"}, nil)
	cas.On("Upload", testutils.AnyContext, mock.AnythingOfType("string"), []string{"somefile.txt"}, []string{rbe.ExcludeGitDir}).Return("fake-digest", nil)

	// Run GetOrCacheRepoState to populate the cache.
	_, err = c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)

	// Verify that the cache entry now exists.
	cached, cachedErr, err = tcc.Get(ctx, rs)
	require.NoError(t, err)
	require.NoError(t, cachedErr)
	require.NotNil(t, cached)

	// Verify that we synced.
	require.True(t, didSync(mockExec))
}

func TestGetOrCacheRepoState_HasPatch_TasksCfgModified_NoSync(t *testing.T) {
	ctx, c, tcc, _, rs, mockGitiles, mockGerrit, mockExec, cleanup := setup(t)
	defer cleanup()

	// Add a patch to the RepoState.
	const issue = int64(12345)
	rs.Patch = types.Patch{
		Issue:    strconv.FormatInt(issue, 10),
		Patchset: "1",
		Server:   "fake-gerrit-server",
	}

	// The test TasksCfg contains CasSpecs which are already resolved, so we
	// won't need a sync by default.
	tasksJson := testutils.MarshalIndentJSON(t, tcc_testutils.TasksCfg2)

	// Verify that the cache entry doesn't exist yet.
	cached, cachedErr, err := tcc.Get(ctx, rs)
	require.EqualError(t, task_cfg_cache.ErrNoSuchEntry, err.Error())
	require.NoError(t, cachedErr)
	require.Nil(t, cached)

	// Set up mocks.
	mockGitiles.On("ReadFileAtRef", testutils.AnyContext, specs.TASKS_CFG_FILE, rs.Revision).Return([]byte(tasksJson), nil)
	mockGerrit.On("GetFileNames", testutils.AnyContext, issue, rs.Patchset).Return([]string{specs.TASKS_CFG_FILE}, nil)
	mockGerrit.On("GetPatch", testutils.AnyContext, issue, rs.Patchset, specs.TASKS_CFG_FILE).Return(`diff --git a/infra/bots/tasks.json b/infra/bots/tasks.json
index c0f0a49..d5733b3 100644
--- a/infra/bots/tasks.json
+++ b/infra/bots/tasks.json
@@ -15,6 +15,7 @@
     "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
       "priority": 0.5,
       "tasks": [
+        "AddedTask",
         "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
       ]
     }
@@ -103,6 +104,39 @@
         ]
       },
       "priority": 0.5
+    },
+    "AddedTask": {
+      "casSpec": "perf",
+      "cipd_packages": [
+        {
+          "name": "skimage",
+          "path": "skimage",
+          "version": "version:0"
+        },
+        {
+          "name": "skp",
+          "path": "skp",
+          "version": "version:0"
+        }
+      ],
+      "command": [
+        "test",
+        "skia"
+      ],
+      "dependencies": [
+        "Build-Ubuntu-GCC-Arm7-Release-Android"
+      ],
+      "dimensions": [
+        "pool:Skia",
+        "os:Android",
+        "device_type:grouper"
+      ],
+      "env_prefixes": {
+        "PATH": [
+          "curdir"
+        ]
+      },
+      "priority": 0.5
     }
   },
   "casSpecs": {
`, nil)

	// Run GetOrCacheRepoState to populate the cache.
	_, err = c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)

	// Verify that the cache entry now exists.
	cached, cachedErr, err = tcc.Get(ctx, rs)
	require.NoError(t, err)
	require.NoError(t, cachedErr)
	require.NotNil(t, cached)

	// Verify that the added task exists.
	require.Contains(t, cached.Jobs["Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"].TaskSpecs, "AddedTask")
	_, ok := cached.Tasks["AddedTask"]
	require.True(t, ok)

	// Verify that we didn't sync.
	require.False(t, didSync(mockExec))
}

func TestGetOrCacheRepoState_HasPatch_TasksCfgModified_Sync(t *testing.T) {
	ctx, c, tcc, cas, rs, mockGitiles, mockGerrit, mockExec, cleanup := setup(t)
	defer cleanup()

	// Add a patch to the RepoState.
	const issue = int64(12345)
	rs.Patch = types.Patch{
		Issue:    strconv.FormatInt(issue, 10),
		Patchset: "1",
		Server:   "fake-gerrit-server",
	}

	// The test TasksCfg contains CasSpecs that are already resolved. Un-resolve
	// them to force a sync.
	tasksCfg := tcc_testutils.TasksCfg2.Copy()
	tasksCfg.CasSpecs = map[string]*specs.CasSpec{
		"my-cas": {
			Root:     ".",
			Paths:    []string{"somefile.txt"},
			Excludes: []string{rbe.ExcludeGitDir},
		},
	}
	for _, task := range tasksCfg.Tasks {
		task.CasSpec = "my-cas"
	}
	tasksJson := testutils.MarshalIndentJSON(t, tasksCfg)

	// Verify that the cache entry doesn't exist yet.
	cached, cachedErr, err := tcc.Get(ctx, rs)
	require.EqualError(t, task_cfg_cache.ErrNoSuchEntry, err.Error())
	require.NoError(t, cachedErr)
	require.Nil(t, cached)

	// Set up mocks.
	mockGitiles.On("ReadFileAtRef", testutils.AnyContext, specs.TASKS_CFG_FILE, rs.Revision).Return([]byte(tasksJson), nil)
	mockGerrit.On("GetFileNames", testutils.AnyContext, issue, rs.Patchset).Return([]string{specs.TASKS_CFG_FILE}, nil)
	mockGerrit.On("GetPatch", testutils.AnyContext, issue, rs.Patchset, specs.TASKS_CFG_FILE).Return(`diff --git a/infra/bots/tasks.json b/infra/bots/tasks.json
index c0f0a49..d5733b3 100644
--- a/infra/bots/tasks.json
+++ b/infra/bots/tasks.json
@@ -15,6 +15,7 @@
     "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
       "priority": 0.5,
       "tasks": [
+        "AddedTask",
         "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
       ]
     }
@@ -103,6 +104,39 @@
         ]
       },
       "priority": 0.5
+    },
+    "AddedTask": {
+      "casSpec": "my-cas",
+      "cipd_packages": [
+        {
+          "name": "skimage",
+          "path": "skimage",
+          "version": "version:0"
+        },
+        {
+          "name": "skp",
+          "path": "skp",
+          "version": "version:0"
+        }
+      ],
+      "command": [
+        "test",
+        "skia"
+      ],
+      "dependencies": [
+        "Build-Ubuntu-GCC-Arm7-Release-Android"
+      ],
+      "dimensions": [
+        "pool:Skia",
+        "os:Android",
+        "device_type:grouper"
+      ],
+      "env_prefixes": {
+        "PATH": [
+          "curdir"
+        ]
+      },
+      "priority": 0.5
     }
   },
   "casSpecs": {
`, nil)
	cas.On("Upload", testutils.AnyContext, mock.AnythingOfType("string"), []string{"somefile.txt"}, []string{rbe.ExcludeGitDir}).Return("fake-digest", nil)

	// Run GetOrCacheRepoState to populate the cache.
	_, err = c.GetOrCacheRepoState(ctx, rs)
	require.NoError(t, err)

	// Verify that the cache entry now exists.
	cached, cachedErr, err = tcc.Get(ctx, rs)
	require.NoError(t, err)
	require.NoError(t, cachedErr)
	require.NotNil(t, cached)

	// Verify that the added task exists.
	require.Contains(t, cached.Jobs["Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"].TaskSpecs, "AddedTask")
	_, ok := cached.Tasks["AddedTask"]
	require.True(t, ok)

	// Verify that we synced.
	require.True(t, didSync(mockExec))
}
