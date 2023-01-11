package specs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/task_scheduler/go/types"
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
		MaxAttempts:    5,
		Outputs:        []string{"out"},
		Priority:       19.0,
		ServiceAccount: "fake-account@gmail.com",
		TaskExecutor:   types.TaskExecutor_Swarming,
	}
}

func fakeJobSpec() *JobSpec {
	return &JobSpec{
		IsCD:      true,
		TaskSpecs: []string{"Build", "Test"},
		Trigger:   "trigger-name",
		Priority:  753,
	}
}

func fakeCasSpec() *CasSpec {
	return &CasSpec{
		Root:     ".",
		Paths:    []string{"a/b", "c/d"},
		Excludes: []string{"skip", "me"},
		Digest:   "abc123/32",
	}
}

func fakeCommitQueueJobConfig() *CommitQueueJobConfig {
	return &CommitQueueJobConfig{
		LocationRegexes: []string{"infra/canvaskit/.*", "modules/canvaskit/.*"},
		Experimental:    true,
	}
}

func TestCopyTasksCfg(t *testing.T) {
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
		CommitQueue: map[string]*CommitQueueJobConfig{
			"job-name": fakeCommitQueueJobConfig(),
		},
	}
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyTaskSpec(t *testing.T) {
	v := fakeTaskSpec()
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyJobSpec(t *testing.T) {
	v := fakeJobSpec()
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyCasSpec(t *testing.T) {
	v := fakeCasSpec()
	assertdeep.Copy(t, v, v.Copy())
}

func TestCommitQueueJobConfig(t *testing.T) {
	v := fakeCommitQueueJobConfig()
	assertdeep.Copy(t, v, v.Copy())
}

// makeTasksCfg creates a TasksCfg based on the given tasks and jobs.
func makeTasksCfg(t *testing.T, tasks, jobs map[string][]string) *TasksCfg {
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
	return &cfg
}

func TestTasksCircularDependency(t *testing.T) {

	type testCase struct {
		name      string
		tasks     map[string][]string
		jobs      map[string][]string
		expectErr string
	}
	test := func(tc testCase) {
		t.Run(tc.name, func(t *testing.T) {
			err := makeTasksCfg(t, tc.tasks, tc.jobs).Validate()
			if tc.expectErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			}
		})
	}

	for _, tc := range []testCase{
		{
			name: "Unknown dependency",
			tasks: map[string][]string{
				"a": {"b"},
			},
			jobs: map[string][]string{
				"j": {"a"},
			},
			expectErr: "Invalid TasksCfg: Task \"a\" has unknown task \"b\" as a dependency.",
		},
		{
			name: "Single-node cycle",
			tasks: map[string][]string{
				"a": {"a"},
			},
			jobs: map[string][]string{
				"j": {"a"},
			},
			expectErr: "Invalid TasksCfg: Found a circular dependency involving \"a\" and \"a\"",
		},
		{
			name: "Small cycle",
			tasks: map[string][]string{
				"a": {"b"},
				"b": {"a"},
			},
			jobs: map[string][]string{
				"j": {"a"},
			},
			expectErr: "Invalid TasksCfg: Found a circular dependency involving \"b\" and \"a\"",
		},
		{
			name: "Longer cycle",
			tasks: map[string][]string{
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
			},
			jobs: map[string][]string{
				"j": {"a"},
			},
			expectErr: "Invalid TasksCfg: Found a circular dependency involving \"j\" and \"a\"",
		},
		{
			name: "No false positive on a complex-ish graph",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
				"d": {"b"},
				"e": {"b"},
				"f": {"c"},
				"g": {"d", "e", "f"},
			},
			jobs: map[string][]string{
				"j": {"a", "g"},
			},
			expectErr: "",
		},
		{
			name: "Unreachable task",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
				"d": {"b"},
				"e": {"b"},
				"f": {"c"},
				"g": {"e", "f"},
			},
			jobs: map[string][]string{
				"j": {"g"},
			},
			expectErr: "Invalid TasksCfg: Task \"d\" is not reachable by any Job!",
		},
		{
			name: "",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
				"d": {"b"},
				"e": {"b"},
				"f": {"c"},
				"g": {"e", "f"},
			},
			jobs: map[string][]string{
				"j": {"q"},
			},
			expectErr: "Invalid TasksCfg: Job \"j\" has unknown task \"q\" as a dependency.",
		},
	} {
		test(tc)
	}
}

func TestGetTaskSpecDAG(t *testing.T) {
	test := func(name string, dag map[string][]string, jobDeps []string) {
		t.Run(name, func(t *testing.T) {
			cfg := makeTasksCfg(t, dag, map[string][]string{
				"j": jobDeps,
			})
			require.NoError(t, cfg.Validate())
			j, ok := cfg.Jobs["j"]
			require.True(t, ok)
			res, err := j.GetTaskSpecDAG(cfg)
			require.NoError(t, err)
			assertdeep.Equal(t, res, dag)
		})
	}

	test("one task", map[string][]string{"a": {}}, []string{"a"})

	test("two tasks", map[string][]string{
		"a": {"b"},
		"b": {},
	}, []string{"a"})

	test("complex dag", map[string][]string{
		"a": {},
		"b": {"a"},
		"c": {"a"},
		"d": {"b"},
		"e": {"b"},
		"f": {"c"},
		"g": {"d", "e", "f"},
	}, []string{"a", "g"})
}

func TestTasksCfgValidate_NoMixingCDAndNonCD(t *testing.T) {

	type testCase struct {
		name      string
		tasks     map[string][]string
		jobs      map[string][]string
		cdJobs    []string
		expectErr string
	}
	test := func(tc testCase) {
		t.Run(tc.name, func(t *testing.T) {
			cfg := makeTasksCfg(t, tc.tasks, tc.jobs)
			for _, cdJob := range tc.cdJobs {
				cfg.Jobs[cdJob].IsCD = true
				cfg.Jobs[cdJob].Trigger = TRIGGER_MAIN_ONLY
			}
			err := cfg.Validate()
			if tc.expectErr == "" {
				require.Nil(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			}
		})
	}

	for _, tc := range []testCase{
		// 1. One task, and two jobs which depend on it.
		{
			name: "Shared task, no CD jobs",
			tasks: map[string][]string{
				"a": {},
			},
			jobs: map[string][]string{
				"b": {"a"},
				"c": {"a"},
			},
			expectErr: "",
		},

		// 1a. One of the jobs is a CD job; there's a conflict.
		{
			name: "Shared task, one CD job",
			tasks: map[string][]string{
				"a": {},
			},
			jobs: map[string][]string{
				"b": {"a"},
				"c": {"a"},
			},
			cdJobs:    []string{"b"},
			expectErr: `Mixing CD and non-CD tasks: task "a" wanted by job "c"`,
		},

		// 1b. Both jobs are CD jobs; no conflict.
		{
			name: "Shared task, two CD jobs",
			tasks: map[string][]string{
				"a": {},
			},
			jobs: map[string][]string{
				"b": {"a"},
				"c": {"a"},
			},
			cdJobs:    []string{"b", "c"},
			expectErr: "",
		},

		// 2. Two totally independent pipelines.
		{
			name: "No shared tasks, no CD jobs",
			tasks: map[string][]string{
				"a": {},
				"b": {},
			},
			jobs: map[string][]string{
				"c": {"a"},
				"d": {"b"},
			},
			expectErr: "",
		},

		// 2a. One of the jobs is a CD job; no conflict, since no tasks are shared.
		{
			name: "No shared tasks, one CD job",
			tasks: map[string][]string{
				"a": {},
				"b": {},
			},
			jobs: map[string][]string{
				"c": {"a"},
				"d": {"b"},
			},
			cdJobs:    []string{"c"},
			expectErr: "",
		},

		// 2b. Both are CD jobs; still no conflict.
		{
			name: "No shared tasks, two CD jobs",
			tasks: map[string][]string{
				"a": {},
				"b": {},
			},
			jobs: map[string][]string{
				"c": {"a"},
				"d": {"b"},
			},
			cdJobs:    []string{"c"},
			expectErr: "",
		},

		// 3. Shared task further down in the pipeline.
		{
			name: "Larger pipeline with one shared task, no CD jobs",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
			},
			jobs: map[string][]string{
				"d": {"b"},
				"e": {"c"},
			},
			expectErr: "",
		},

		// 3a. One of the jobs is a CD job; there's a conflict.
		{
			name: "Larger pipeline with one shared task, one CD job",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
			},
			jobs: map[string][]string{
				"d": {"b"},
				"e": {"c"},
			},
			cdJobs:    []string{"d"},
			expectErr: `Mixing CD and non-CD tasks: task "a" wanted by job "e"`,
		},

		// 3b. Both jobs are CD jobs; no conflict.
		{
			name: "Larger pipeline with one shared task, two CD jobs",
			tasks: map[string][]string{
				"a": {},
				"b": {"a"},
				"c": {"a"},
			},
			jobs: map[string][]string{
				"d": {"b"},
				"e": {"c"},
			},
			cdJobs:    []string{"d", "e"},
			expectErr: "",
		},
	} {
		test(tc)
	}
}

func TestTasksCfgValidate_CDJobsAreMainBranchOnly(t *testing.T) {

	test := func(name, trigger, expectErr string) {
		t.Run(name, func(t *testing.T) {
			cfg := makeTasksCfg(t, map[string][]string{
				"a": {},
			}, map[string][]string{
				"b": {"a"},
			})
			job := cfg.Jobs["b"]
			job.IsCD = true
			job.Trigger = trigger
			err := cfg.Validate()
			if expectErr == "" {
				require.NoError(t, err)
			} else {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), expectErr)
			}
		})
	}

	// Default trigger is ANY_BRANCH; changing the job to CD causes an error.
	test("Any branch not okay", TRIGGER_ANY_BRANCH, `CD job "b" must trigger on main/master branch only`)

	// Use a non-default trigger that still isn't correct.
	test("Nightly not okay", TRIGGER_NIGHTLY, `CD job "b" must trigger on main/master branch only`)

	// MAIN_ONLY fixes the error.
	test("Main is allowed", TRIGGER_MAIN_ONLY, "")

	// MASTER_ONLY is equivalent.
	test("Master is allowed", TRIGGER_MASTER_ONLY, "")
}
