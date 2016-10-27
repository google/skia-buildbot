package specs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/skia-dev/glog"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitrepo"
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

func TestCopyTaskSpec(t *testing.T) {
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
		ExtraArgs: []string{"--do-really-awesome-stuff"},
		Isolate:   "abc123",
		Priority:  19.0,
	}
	testutils.AssertCopy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	v := &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Priority:  753,
	}
	testutils.AssertCopy(t, v, v.Copy())
}

func TestTaskSpecs(t *testing.T) {
	testutils.SkipIfShort(t)

	tr := util.NewTempRepo()
	defer tr.Cleanup()

	repoUrl := path.Join(tr.Dir, repoName)
	repo, err := gitrepo.NewRepo(repoUrl, tr.Dir)
	assert.NoError(t, err)
	repos := map[string]*gitrepo.Repo{
		repoUrl: repo,
	}
	cache := NewTaskCfgCache(repos)

	rs1 := db.RepoState{
		Repo:     repoUrl,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repoUrl,
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

	repoUrl := path.Join(tr.Dir, repoName)
	repo, err := gitrepo.NewRepo(repoUrl, tr.Dir)
	assert.NoError(t, err)
	repos := map[string]*gitrepo.Repo{
		repoUrl: repo,
	}
	cache := NewTaskCfgCache(repos)

	// Load configs into the cache.
	rs1 := db.RepoState{
		Repo:     repoUrl,
		Revision: c1,
	}
	rs2 := db.RepoState{
		Repo:     repoUrl,
		Revision: c2,
	}
	_, err = cache.GetTaskSpecsForRepoStates([]db.RepoState{rs1, rs2})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cache.cache))

	// Cleanup, with a period intentionally designed to remove c1 but not c2.
	r, err := git.NewRepo(repoUrl, tr.Dir)
	assert.NoError(t, err)
	d1, err := r.Details(c1)
	assert.NoError(t, err)
	// c1 and c2 are about 5 seconds apart.
	period := time.Now().Sub(d1.Timestamp) - 2*time.Second
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

func tempGitRepoSetup(t *testing.T) (*testutils.GitBuilder, string, string) {
	testutils.SkipIfShort(t)

	gb := testutils.GitInit(t)
	gb.Add("codereview.settings", `CODE_REVIEW_SERVER: codereview.chromium.org
PROJECT: skia`)
	c1 := gb.CommitMsg("initial commit")
	c2 := gb.CommitGen("somefile")
	return gb, c1, c2
}

func tempGitRepoTests(t *testing.T, repo *gitrepo.Repo, cases map[db.RepoState]error) {
	for rs, expectErr := range cases {
		c, err := TempGitRepo(repo.Repo(), rs)
		if expectErr != nil {
			assert.EqualError(t, err, expectErr.Error())
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
	gb, c1, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	repo, err := gitrepo.NewRepo(gb.Dir(), tmpDir)
	assert.NoError(t, err)

	cases := map[db.RepoState]error{
		{
			Repo:     repo.Repo().Dir(),
			Revision: c1,
		}: nil,
		{
			Repo:     repo.Repo().Dir(),
			Revision: c2,
		}: nil,
		{
			Repo:     repo.Repo().Dir(),
			Revision: "bogusRev",
		}: fmt.Errorf("Command exited with exit status 1: git checkout bogusRev; Stdout+Stderr:\nerror: pathspec 'bogusRev' did not match any file(s) known to git.\n"),
	}
	tempGitRepoTests(t, repo, cases)
}

func TestTempGitRepoPatch(t *testing.T) {
	t.Skip("This test uploads to production servers. Don't run it by default.")

	gb, _, c2 := tempGitRepoSetup(t)
	defer gb.Cleanup()

	gb.AddGen("somefile")
	testutils.Run(t, gb.Dir(), "git", "commit", "-m", "commit")

	testutils.Run(t, gb.Dir(), "git", "cl", "upload", "--bypass-hooks", "--rietveld", "-m", "test", "-f")
	output := testutils.Run(t, gb.Dir(), "git", "cl", "issue")
	m := regexp.MustCompile(`Issue number: (\d+)`).FindStringSubmatch(output)
	assert.Equal(t, 2, len(m))
	rvIssue := m[1]

	tmpDir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmpDir)
	repo, err := gitrepo.NewRepo(gb.Dir(), tmpDir)
	assert.NoError(t, err)

	// TODO(borenet): Also upload to Gerrit and verify that we can apply
	// those patches successfully as well. This is difficult because it's
	// hard to trick git-cl into uploading to a particular Gerrit instance
	// from a dummy repo.

	cases := map[db.RepoState]error{
		{
			Patch: db.Patch{
				Server:   "https://codereview.chromium.org",
				Issue:    rvIssue,
				Patchset: "1",
			},
			Repo:     repo.Repo().Dir(),
			Revision: c2,
		}: nil,
	}
	tempGitRepoTests(t, repo, cases)
}

func TestGetTaskSpecDAG(t *testing.T) {
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
