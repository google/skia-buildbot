package scheduling

import (
	"encoding/json"
	"path"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestTaskSpecs(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repos := gitinfo.NewRepoMap(tr.Dir)
	cache := newTaskCfgCache(repos)

	c1 := "81add9e329cde292667a1ce427007b5ff701fad1"
	c2 := "4595a2a2662d6cef863870ca68f64824c4b5ef2d"
	repo := "skia.git"
	rs1 := db.RepoState{
		Repo:     repo,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repo,
		Revision: c2,
	}
	specs, err := cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)

	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name, _ := range byName {
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
	cache := newTaskCfgCache(repos)

	// Load configs into the cache.
	c1 := "81add9e329cde292667a1ce427007b5ff701fad1"
	c2 := "4595a2a2662d6cef863870ca68f64824c4b5ef2d"
	repo := "skia.git"
	rs1 := db.RepoState{
		Repo:     repo,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repo,
		Revision: c2,
	}
	_, err := cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := gitinfo.NewGitInfo(path.Join(tr.Dir, repo), false, false)
	assert.NoError(t, err)
	d1, err := r.Details(c1, false)
	assert.NoError(t, err)
	// c1 and c2 are about 5 seconds apart.
	period := time.Now().Sub(d1.Timestamp) - 2*time.Second
	assert.NoError(t, cache.Cleanup(period))
	assert.Equal(t, 1, len(cache.cache))
}

func TestTasksCircularDependency(t *testing.T) {
	makeTasksCfg := func(tasks map[string][]string) string {
		specs := make(map[string]*TaskSpec, len(tasks))
		for name, deps := range tasks {
			specs[name] = &TaskSpec{
				CipdPackages: []*CipdPackage{},
				Dependencies: deps,
				Dimensions:   []string{},
				Isolate:      "abc123",
				Priority:     0.0,
			}
		}
		cfg := TasksCfg{
			Tasks: specs,
		}
		c, err := json.Marshal(&cfg)
		assert.NoError(t, err)
		return string(c)
	}

	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"b"},
	}))
	assert.EqualError(t, err, "Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{"b"},
		"b": []string{"a"},
	}))
	// Can't use a specific error message because map iteration order is non-deterministic.
	assert.Error(t, err)

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
	}))
	assert.Error(t, err)

	// No false positive on a complex-ish graph.
	_, err = ParseTasksCfg(makeTasksCfg(map[string][]string{
		"a": []string{},
		"b": []string{"a"},
		"c": []string{"a"},
		"d": []string{"b"},
		"e": []string{"b"},
		"f": []string{"c"},
		"g": []string{"d", "e", "f"},
	}))
	assert.NoError(t, err)
}
