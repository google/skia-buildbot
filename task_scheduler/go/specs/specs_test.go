package specs

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	// The test repo has two commits. The first commit adds a tasks.cfg file
	// with two task specs: a build task and a test task, the test task
	// depending on the build task. The second commit adds a perf task spec,
	// which also depends on the build task. Therefore, there are five total
	// possible tasks we could run:
	//
	// Build@c1, Test@c1, Build@c2, Test@c2, Perf@c2
	//
	c1        = "10ca3b86bac8991967ebe15cc89c22fd5396a77b"
	c2        = "d4fa60ab35c99c886220c4629c36b9785cc89c8b"
	buildTask = "Build-Ubuntu-GCC-Arm7-Release-Android"
	testTask  = "Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	perfTask  = "Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release"
	repoName  = "skia.git"
)

func TestTaskSpecs(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repos := gitinfo.NewRepoMap(tr.Dir)
	cache := NewTaskCfgCache(repos)

	rs1 := db.RepoState{
		Repo:     repoName,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repoName,
		Revision: c2,
	}
	specs, err := cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name, _ := range byName {
			glog.Infof("%s %s", rs, name)
			total++
			if rs.Revision == c1 {
				countC1++
			} else if rs.Revision == c2 {
				countC2++
			} else {
				t.Fatalf("Unknown commit: %q", rs.Revision)
			}
			if strings.HasPrefix(name, "Build") {
				countBuild++
			} else if strings.HasPrefix(name, "Test") {
				countTest++
			} else if strings.HasPrefix(name, "Perf") {
				countPerf++
			} else {
				t.Fatalf("Unknown task spec name: %q", name)
			}
		}
	}
	assert.Equal(t, 2, countC1)
	assert.Equal(t, 3, countC2)
	assert.Equal(t, 2, countBuild)
	assert.Equal(t, 2, countTest)
	assert.Equal(t, 1, countPerf)
	assert.Equal(t, 5, total)
}

func TestTaskCfgCacheCleanup(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repos := gitinfo.NewRepoMap(tr.Dir)
	cache := NewTaskCfgCache(repos)

	// Load configs into the cache.
	rs1 := db.RepoState{
		Repo:     repoName,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repoName,
		Revision: c2,
	}
	_, err := cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := gitinfo.NewGitInfo(path.Join(tr.Dir, repoName), false, false)
	assert.NoError(t, err)
	d1, err := r.Details(c1, false)
	assert.NoError(t, err)
	// c1 and c2 are about 5 seconds apart.
	period := time.Now().Sub(d1.Timestamp) - 2*time.Second
	assert.NoError(t, cache.Cleanup(period))
	assert.Equal(t, 1, len(cache.cache))
}

func TestTasksCircularDependency(t *testing.T) {
	// tasks is a map[task_spec_name][]dependency_name
	makeTasksCfg := func(tasks, jobs map[string][]string) string {
		taskSpecs := make(map[string]*TaskSpec, len(tasks))
		for name, deps := range tasks {
			taskSpecs[name] = &TaskSpec{
				CipdPackages: []*CipdPackage{},
				Dependencies: deps,
				Dimensions:   []string{},
				Isolate:      "abc123",
				Priority:     0.0,
			}
		}
		jobSpecs := make(map[string]*JobSpec, len(jobs))
		for name, deps := range jobs {
			jobSpecs[name] = &JobSpec{
				TaskSpecs: deps,
			}
		}
		cfg := TasksCfg{
			Tasks: taskSpecs,
			Jobs:  jobSpecs,
		}
		c, err := json.Marshal(&cfg)
		assert.NoError(t, err)
		return string(c)
	}

	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"b"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{}, map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"a"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"b"},
		"b": []string{"a"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"b\" and \"a\"")

	// Longer cycle.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"b"},
		"b": []string{"c"},
		"c": []string{"d"},
		"d": []string{"e"},
		"e": []string{"f"},
		"f": []string{"g"},
		"g": []string{"h"},
		"h": []string{"i"},
		"i": []string{"j"},
		"j": []string{"a"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"j\" and \"a\"")

	// No false positive on a complex-ish graph.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{},
		"b": []string{"a"},
		"c": []string{"a"},
		"d": []string{"b"},
		"e": []string{"b"},
		"f": []string{"c"},
		"g": []string{"d", "e", "f"},
	}, map[string][]string{
		"j": []string{"a", "g"},
	}))
	assert.NoError(t, err)

	// Unreachable task (d)
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{},
		"b": []string{"a"},
		"c": []string{"a"},
		"d": []string{"b"},
		"e": []string{"b"},
		"f": []string{"c"},
		"g": []string{"e", "f"},
	}, map[string][]string{
		"j": []string{"g"},
	}))
	assert.EqualError(t, err, "Task \"d\" is not reachable by any Job!")

	// Dependency on unknown task.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{},
		"b": []string{"a"},
		"c": []string{"a"},
		"d": []string{"b"},
		"e": []string{"b"},
		"f": []string{"c"},
		"g": []string{"e", "f"},
	}, map[string][]string{
		"j": []string{"q"},
	}))
	assert.EqualError(t, err, "Job \"j\" has unknown task \"q\" as a dependency.")
}
