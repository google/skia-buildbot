package specs

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestCopyTaskSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &TaskSpec{
		Caches: []*Cache{
			{
				Name: "cache-me",
				Path: "if/you/can",
			},
		},
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
		IoTimeout:      10 * time.Minute,
		Isolate:        "abc123",
		MaxAttempts:    5,
		Outputs:        []string{"out"},
		Priority:       19.0,
		ServiceAccount: "fake-account@gmail.com",
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	testutils.SmallTest(t)
	v := &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
	deepequal.AssertCopy(t, v, v.Copy())
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
	return testutils.MarshalIndentJSON(t, &cfg)
}

func TestTasksCircularDependency(t *testing.T) {
	testutils.SmallTest(t)
	// Bonus: Unknown dependency.
	_, err := ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Task \"a\" has unknown task \"b\" as a dependency.")

	// No tasks or jobs.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{}, map[string][]string{}))
	assert.NoError(t, err)

	// Single-node cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"a\" and \"a\"")

	// Small cycle.
	_, err = ParseTasksCfg(makeTasksCfg(t, map[string][]string{
		"a": {"b"},
		"b": {"a"},
	}, map[string][]string{
		"j": {"a"},
	}))
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"b\" and \"a\"")

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
	assert.EqualError(t, err, "Invalid TasksCfg: Found a circular dependency involving \"j\" and \"a\"")

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
	assert.NoError(t, err)

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
	assert.EqualError(t, err, "Invalid TasksCfg: Task \"d\" is not reachable by any Job!")

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
	assert.EqualError(t, err, "Invalid TasksCfg: Job \"j\" has unknown task \"q\" as a dependency.")
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
		deepequal.AssertDeepEqual(t, res, dag)
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
