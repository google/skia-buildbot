package specs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func fakeTaskSpec() *TaskSpec {
	return &TaskSpec{
		Caches: []*Cache{
			{
				Name: "cache-me",
				Path: "if/you/can",
			},
		},
		CasSpec: "my-cas",
		CipdPackages: []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "23",
			},
		},
		Command:      []string{"do", "something"},
		Dependencies: []string{"coffee", "chocolate"},
		Dimensions:   []string{"width:13", "height:17"},
		Environment: map[string]string{
			"Polluted": "true",
		},
		EnvPrefixes: map[string][]string{
			"PATH": {"curdir"},
		},
		ExecutionTimeout: 60 * time.Minute,
		Expiration:       90 * time.Minute,
		ExtraArgs:        []string{"--do-really-awesome-stuff"},
		ExtraTags: map[string]string{
			"dummy_tag": "dummy_val",
		},
		Idempotent:     true,
		IoTimeout:      10 * time.Minute,
		Isolate:        "abc123",
		MaxAttempts:    5,
		Outputs:        []string{"out"},
		Priority:       19.0,
		ServiceAccount: "fake-account@gmail.com",
	}
}

func fakeJobSpec() *JobSpec {
	return &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
}

func fakeCasSpec() *CasSpec {
	return &CasSpec{
		Root:   ".",
		Paths:  []string{"a/b", "c/d"},
		Digest: "abc123/32",
	}
}

func TestCopyTasksCfg(t *testing.T) {
	unittest.SmallTest(t)
	v := &TasksCfg{
		CasSpecs: map[string]*CasSpec{
			"my-cas": fakeCasSpec(),
		},
		Jobs: map[string]*JobSpec{
			"job-name": fakeJobSpec(),
		},
		Tasks: map[string]*TaskSpec{
			"task-name": fakeTaskSpec(),
		},
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyTaskSpec(t *testing.T) {
	unittest.SmallTest(t)
	v := fakeTaskSpec()
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	unittest.SmallTest(t)
	v := fakeJobSpec()
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyCasSpec(t *testing.T) {
	unittest.SmallTest(t)
	v := fakeCasSpec()
	assertdeep.Copy(t, v, v.Copy())
}

// makeTasksCfg generates a JSON representation of a TasksCfg based on the given
// tasks and jobs.
func makeTasksCfg(t *testing.T, tasks, jobs map[string][]string) string {
	taskSpecs := make(map[string]*TaskSpec, len(tasks))
	for name, deps := range tasks {
		taskSpecs[name] = &TaskSpec{
			CasSpec:      "my-cas",
			CipdPackages: []*CipdPackage{},
			Dependencies: deps,
			Dimensions:   []string{"os:whatever"},
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
	if len(taskSpecs) > 0 {
		cfg.CasSpecs = map[string]*CasSpec{
			"my-cas": {
				Digest: "abc123/45",
			},
		}
	}
	return testutils.MarshalIndentJSON(t, &cfg)
}

func TestTasksCircularDependency(t *testing.T) {
	unittest.SmallTest(t)
	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
	}, map[string][]string{
		"j": {"a"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{}, map[string][]string{}))
	require.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"b\" and \"a\"")

	// Longer cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"d"},
		"d": {"e"},
		"e": {"f"},
		"f": {"g"},
		"g": {"h"},
		"h": {"i"},
		"i": {"j"},
		"j": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"j\" and \"a\"")

	// No false positive on a complex-ish graph.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"d", "e", "f"},
	}, map[string][]string{
		"j": {"a", "g"},
	}))
	require.NoError(t, err)

	// Unreachable task (d)
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"e", "f"},
	}, map[string][]string{
		"j": {"g"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Task \"d\" is not reachable by any Job!")

	// Dependency on unknown task.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"e", "f"},
	}, map[string][]string{
		"j": {"q"},
	}))
	require.EqualError(t, err, "Invalid TasksCfg: Job \"j\" has unknown task \"q\" as a dependency.")
}

func TestGetTaskSpecDAG(t *testing.T) {
	unittest.SmallTest(t)
	test := func(dag map[string][]string, jobDeps []string) {
		cfg, err := ParseTasksCfg(makeTasksCfg(t, dag, map[string][]string{
			"j": jobDeps,
		}))
		require.NoError(t, err)
		j, ok := cfg.Jobs["j"]
		require.True(t, ok)
		res, err := j.GetTaskSpecDAG(cfg)
		require.NoError(t, err)
		assertdeep.Equal(t, res, dag)
	}

	test(map[string][]string{"a": {}}, []string{"a"})

	test(map[string][]string{
		"a": {"b"},
		"b": {},
	}, []string{"a"})

	test(map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"d", "e", "f"},
	}, []string{"a", "g"})
}
