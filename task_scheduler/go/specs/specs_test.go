package specs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		CipdPackages: []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "23",
			},
		},
		Command:      []string{"do", "something"},
		Dependencies: []string{"chocolate", "coffee"},
		Dimensions:   []string{"height:17", "width:13"},
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
		Template:       "tmpl",
	}
}

func fakeJobSpec() *JobSpec {
	return &JobSpec{
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
}

func TestCopyTasksCfg(t *testing.T) {
	unittest.SmallTest(t)
	v := &TasksCfg{
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

func TestTaskSpecTemplates(t *testing.T) {
	unittest.SmallTest(t)

	testAgainstCopy := func(modify func(*TaskSpec, *TaskSpec, *TaskSpec)) {
		task := CopyTaskSpec.Copy()
		task.Template = "tmpl"
		tmpl := CopyTaskSpec.Copy()
		tmpl.Template = ""
		expect := CopyTaskSpec.Copy()
		expect.Template = ""
		modify(task, tmpl, expect)
		cfg := &TasksCfg{
			Jobs: map[string]*JobSpec{
				"job1": &JobSpec{
					TaskSpecs: []string{"task1"},
				},
			},
			Tasks: map[string]*TaskSpec{
				"task1": task,
			},
			Templates: map[string]*TaskSpec{
				task.Template: tmpl,
			},
		}
		for _, d := range append(task.Dependencies, tmpl.Dependencies...) {
			cfg.Tasks[d] = &TaskSpec{
				Isolate: "isolate",
			}
		}
		cfg2, err := ParseTasksCfg(testutils.MarshalJSON(t, cfg))
		assert.NoError(t, err)
		// Command and ExtraArgs are special cases; we concatenate the
		// two lists, so we need to do that here rather than asserting
		// that they're equal.
		expect.Command = append(tmpl.Command, task.Command...)
		expect.ExtraArgs = append(tmpl.ExtraArgs, task.ExtraArgs...)
		deepequal.AssertDeepEqual(t, cfg2.Tasks["task1"], expect)
	}

	// Test cases.

	// 1. Caches.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Caches = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Caches = []*Cache{
			&Cache{
				Name: "git",
				Path: "git/cache",
			},
		}
		expect.Caches = []*Cache{
			&Cache{
				Name: "cache-me",
				Path: "if/you/can",
			},
			&Cache{
				Name: "git",
				Path: "git/cache",
			},
		}
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Caches = []*Cache{
			&Cache{
				Name: "cache-me",
				Path: "seriously/you/cant",
			},
		}
		expect.Caches = []*Cache{
			&Cache{
				Name: "cache-me",
				Path: "if/you/can",
			},
			&Cache{
				Name: "cache-me",
				Path: "seriously/you/cant",
			},
		}
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Caches = []*Cache{
			&Cache{
				Name: "can-can",
				Path: "if/you/can",
			},
		}
		expect.Caches = []*Cache{
			&Cache{
				Name: "cache-me",
				Path: "if/you/can",
			},
			&Cache{
				Name: "can-can",
				Path: "if/you/can",
			},
		}
	})

	// 2. CipdPackages.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.CipdPackages = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.CipdPackages = []*CipdPackage{
			{
				Name:    "pkg2",
				Path:    "/home/chrome-bot/2",
				Version: "99",
			},
		}
		expect.CipdPackages = []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "23",
			},
			{
				Name:    "pkg2",
				Path:    "/home/chrome-bot/2",
				Version: "99",
			},
		}
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.CipdPackages = []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "24",
			},
		}
		expect.CipdPackages = []*CipdPackage{
			{
				Name:    "pkg",
				Path:    "/home/chrome-bot",
				Version: "24",
			},
		}
	})

	// 3. Command.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Command = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Command = []string{"a", "b", "c", "d"}
	})

	// 4. Dependencies
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Dependencies = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Dependencies = []string{
			"coffee",
			"abc",
		}
		expect.Dependencies = []string{
			"abc",
			"chocolate",
			"coffee",
		}
	})

	// 5. Dimensions.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Dimensions = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Dimensions = []string{"width:15", "depth:8"}
		expect.Dimensions = []string{"depth:8", "height:17", "width:15"}
	})

	// 6. Environment.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Environment = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Environment = map[string]string{
			"var": "val",
		}
		expect.Environment = map[string]string{
			"var":      "val",
			"Polluted": "true",
		}
	})

	// 7. EnvPrefixes.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.EnvPrefixes = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.EnvPrefixes = map[string][]string{
			"var":  []string{"val"},
			"PATH": []string{"otherdir"},
		}
		expect.EnvPrefixes = map[string][]string{
			"var":  []string{"val"},
			"PATH": []string{"curdir", "otherdir"},
		}
	})

	// 8. ExecutionTimeout.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExecutionTimeout = 0
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExecutionTimeout = 42
		expect.ExecutionTimeout = 42
	})

	// 9. Expiration.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Expiration = 0
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Expiration = 88
		expect.Expiration = 88
	})

	// 10. ExtraArgs.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExtraArgs = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExtraArgs = []string{"a", "b", "c", "d"}
	})

	// 11. ExtraTags.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExtraTags = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ExtraTags = map[string]string{
			"key":       "value",
			"dummy_tag": "other_val",
		}
		expect.ExtraTags = map[string]string{
			"key":       "value",
			"dummy_tag": "other_val",
		}
	})

	// 12. IoTimeout.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.IoTimeout = 0
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.IoTimeout = 99
		expect.IoTimeout = 99
	})

	// 13. Isolate.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Isolate = ""
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Isolate = "custom.isolate"
		expect.Isolate = "custom.isolate"
	})

	// 14. MaxAttempts.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.MaxAttempts = 0
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.MaxAttempts = 73
		expect.MaxAttempts = 73
	})

	// 15. Outputs.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Outputs = nil
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Outputs = []string{"build"}
		expect.Outputs = []string{"build", "out"}
	})

	// 16. Priority.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Priority = 0
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.Priority = 0.99
		expect.Priority = 0.99
	})

	// 17. ServiceAccount.
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ServiceAccount = ""
	})
	testAgainstCopy(func(task, tmpl, expect *TaskSpec) {
		task.ServiceAccount = "my-service-account"
		expect.ServiceAccount = "my-service-account"
	})

	// Verify that we catch circular template references.
	_, err := ParseTasksCfg(`{
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "template": "build_android",
      "command": ["skia"]
    }
  },
  "jobs": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "priority": 0.8,
      "tasks": ["Build-Ubuntu-GCC-Arm7-Release-Android"]
    }
  },
  "templates": {
    "build_android": {
      "template": "build_android",
      "cipd_packages": [{
        "name": "android_sdk",
        "path": "android_sdk",
        "version": "version:0"
      }]
    }
  }
}`)
	assert.EqualError(t, err, "Circular template reference.")

	_, err = ParseTasksCfg(`{
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "template": "build_android",
      "command": ["skia"]
    }
  },
  "jobs": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "priority": 0.8,
      "tasks": ["Build-Ubuntu-GCC-Arm7-Release-Android"]
    }
  },
  "templates": {
    "build": {
      "command": ["ninja"],
      "dimensions": ["pool:Skia", "os:Ubuntu"],
      "extra_tags": {"is_build_task": "true"},
      "isolate": "compile_skia.isolate",
      "max_attempts": 5,
      "priority": 0.8,
      "template": "build_android"
    },
    "build_android": {
      "template": "build",
      "cipd_packages": [{
        "name": "android_sdk",
        "path": "android_sdk",
        "version": "version:0"
      }]
    }
  }
}`)
	assert.EqualError(t, err, "Circular template reference.")

	_, err = ParseTasksCfg(`{
  "tasks": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "template": "build_android",
      "command": ["skia"]
    }
  },
  "jobs": {
    "Build-Ubuntu-GCC-Arm7-Release-Android": {
      "priority": 0.8,
      "tasks": ["Build-Ubuntu-GCC-Arm7-Release-Android"]
    }
  },
  "templates": {
    "build": {
      "command": ["ninja"],
      "dimensions": ["pool:Skia", "os:Ubuntu"],
      "extra_tags": {"is_build_task": "true"},
      "isolate": "compile_skia.isolate",
      "max_attempts": 5,
      "priority": 0.8,
      "template": "build2"
    },
    "build2": {
      "template": "build_android"
    },
    "build_android": {
      "template": "build",
      "cipd_packages": [{
        "name": "android_sdk",
        "path": "android_sdk",
        "version": "version:0"
      }]
    }
  }
}`)
	assert.EqualError(t, err, "Circular template reference.")
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
