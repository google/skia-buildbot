package specs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	specs_testutils "go.skia.org/infra/task_scheduler/go/specs/testutils"
)

var (
	// Use this as an expected error when you don't care about the actual
	// error which is returned.
	ERR_DONT_CARE = fmt.Errorf("DONTCARE")
)

func TestCopyTaskSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &TaskSpec{
		CipdPackages: []*CipdPackage{
			&CipdPackage{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "23",
			},
		},
		Dependencies: []string{"coffee", "chocolate"},
		Dimensions:   []string{"width:13", "height:17"},
		Environment: map[string]string{
			"Polluted": "true",
		},
		ExecutionTimeout: 60 * time.Minute,
		Expiration:       90 * time.Minute,
		ExtraArgs:        []string{"--do-really-awesome-stuff"},
		IoTimeout:        10 * time.Minute,
		Isolate:          "abc123",
		MaxAttempts:      5,
		Priority:         19.0,
	}
	testutils.AssertCopy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
	testutils.AssertCopy(t, v, v.Copy())
}

func TestTaskSpecs(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}

	cache := NewTaskCfgCache(repos, specs_testutils.GetDepotTools(t), tmp, DEFAULT_NUM_WORKERS)

	rs1 := db.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	specs, err := cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	// c1 has a Build and Test task, c2 has a Build, Test, and Perf task.
	total, countC1, countC2, countBuild, countTest, countPerf := 0, 0, 0, 0, 0, 0
	for rs, byName := range specs {
		for name, _ := range byName {
			sklog.Infof("%s %s", rs, name)
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
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	gb, c1, c2 := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repo, err := repograph.NewGraph(gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	repos := repograph.Map{
		gb.RepoUrl(): repo,
	}
	cache := NewTaskCfgCache(repos, specs_testutils.GetDepotTools(t), path.Join(tmp, "cache"), DEFAULT_NUM_WORKERS)

	// Load configs into the cache.
	rs1 := db.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c2,
	}
	_, err = cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := git.NewRepo(gb.RepoUrl(), tmp)
	assert.NoError(t, err)
	d1, err := r.Details(c1)
	assert.NoError(t, err)
	d2, err := r.Details(c2)
	diff := d2.Timestamp.Sub(d1.Timestamp)
	now := time.Now()
	period := now.Sub(d2.Timestamp) + (diff / 2)
	assert.NoError(t, cache.Cleanup(period))
	assert.Equal(t, 1, len(cache.cache))
}

// makeTasksCfg generates a JSON representation of a TasksCfg based on the given
// tasks and jobs.
func makeTasksCfg(t *testing.T, tasks, jobs map[string][]string) string {
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

func TestTasksCircularDependency(t *testing.T) {
	testutils.SmallTest(t)
	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": []string{"b"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{}, map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": []string{"a"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": []string{"b"},
		"b": []string{"a"},
	}, map[string][]string{
		"j": []string{"a"},
	}))
	assert.EqualError(t, err, "Found a circular dependency involving \"b\" and \"a\"")

	// Longer cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
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
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
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
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
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
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
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

func tempGitRepoSetup(t *testing.T) (*git_testutils.GitBuilder, string, string) {
	testutils.SkipIfShort(t)

	gb := git_testutils.GitInit(t)
	gb.Add("codereview.settings", `CODE_REVIEW_SERVER: codereview.chromium.org
PROJECT: skia`)
	c1 := gb.CommitMsg("initial commit")
	c2 := gb.CommitGen("somefile")
	return gb, c1, c2
}

func tempGitRepoTests(t *testing.T, cases map[db.RepoState]error) {
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	cacheDir := path.Join(tmp, "cache")
	depotTools := specs_testutils.GetDepotTools(t)
	for rs, expectErr := range cases {
		c, err := tempGitRepo(rs, depotTools, cacheDir, tmp)
		if expectErr != nil {
			assert.Error(t, err)
			if expectErr != ERR_DONT_CARE {
				assert.EqualError(t, err, expectErr.Error())
			}
		} else {
			defer c.Delete()
			assert.NoError(t, err)
			output, err := c.Git("remote", "-v")
			gotRepo := "COULD NOT FIND REPO"
			for _, s := range strings.Split(output, "\n") {
				if strings.HasPrefix(s, "origin") {
					split := strings.Fields(s)
					assert.Equal(t, 3, len(split))
					gotRepo = split[1]
					break
				}
			}
			assert.Equal(t, rs.Repo, gotRepo)
			gotRevision, err := c.RevParse("HEAD")
			assert.NoError(t, err)
			assert.Equal(t, rs.Revision, gotRevision)
			// If not a try job, we expect a clean checkout,
			// otherwise we expect a dirty checkout, from the
			// applied patch.
			_, err = c.Git("diff", "--exit-code", "--no-patch", rs.Revision)
			if rs.IsTryJob() {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestTempGitRepo(t *testing.T) {
	testutils.LargeTest(t)
	gb, c1, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	cases := map[db.RepoState]error{
		{
			Repo:     gb.RepoUrl(),
			Revision: c1,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
		{
			Repo:     gb.RepoUrl(),
			Revision: "bogusRev",
		}: ERR_DONT_CARE,
	}
	tempGitRepoTests(t, cases)
}

func TestTempGitRepoPatch(t *testing.T) {
	testutils.LargeTest(t)

	gb, _, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	issue := "12345"
	patchset := "3"
	gb.CreateFakeGerritCLGen(issue, patchset)

	cases := map[db.RepoState]error{
		{
			Patch: db.Patch{
				Server:   gb.RepoUrl(),
				Issue:    issue,
				Patchset: patchset,
			},
			Repo:     gb.RepoUrl(),
			Revision: c2,
		}: nil,
	}
	tempGitRepoTests(t, cases)
}

func TestTempGitRepoParallel(t *testing.T) {
	testutils.LargeTest(t)
	testutils.SkipIfShort(t)

	gb, c1, _ := specs_testutils.SetupTestRepo(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	repos, err := repograph.NewMap([]string{gb.RepoUrl()}, tmp)
	assert.NoError(t, err)

	cache := NewTaskCfgCache(repos, specs_testutils.GetDepotTools(t), tmp, DEFAULT_NUM_WORKERS)

	rs := db.RepoState{
		Repo:     gb.RepoUrl(),
		Revision: c1,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, cache.TempGitRepo(rs, func(g *git.TempCheckout) error {
				return nil
			}))
		}()
	}
	wg.Wait()
}

func TestGetTaskSpecDAG(t *testing.T) {
	testutils.SmallTest(t)
	test := func(dag map[string][]string, jobDeps []string) {
		cfg, err := ParseTasksCfg(makeTasksCfg(t, dag, map[string][]string{
			"j": jobDeps,
		}))
		assert.NoError(t, err)
		j, ok := cfg.Jobs["j"]
		assert.True(t, ok)
		res, err := j.GetTaskSpecDAG(cfg)
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, res, dag)
	}

	test(map[string][]string{"a": []string{}}, []string{"a"})

	test(map[string][]string{
		"a": []string{"b"},
		"b": []string{},
	}, []string{"a"})

	test(map[string][]string{
		"a": []string{},
		"b": []string{"a"},
		"c": []string{"a"},
		"d": []string{"b"},
		"e": []string{"b"},
		"f": []string{"c"},
		"g": []string{"d", "e", "f"},
	}, []string{"a", "g"})
}
