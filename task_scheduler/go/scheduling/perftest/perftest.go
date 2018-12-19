package main

/*
	Performance test for TaskScheduler.
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/tryjobs"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

func assertNoError(err error) {
	if err != nil {
		sklog.Fatalf("Expected no error but got: %s", err.Error())
	}
}

func assertEqual(a, b interface{}) {
	if a != b {
		sklog.Fatalf("Expected %v but got %v", a, b)
	}
}

func assertDeepEqual(a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		sklog.Fatalf("Objects do not match: \na:\n%s\n\nb:\n%s\n", spew.Sprint(a), spew.Sprint(b))
	}
}

func makeBot(id string, dims map[string]string) *swarming_api.SwarmingRpcsBotInfo {
	dimensions := make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(dims))
	for k, v := range dims {
		dimensions = append(dimensions, &swarming_api.SwarmingRpcsStringListPair{
			Key:   k,
			Value: []string{v},
		})
	}
	return &swarming_api.SwarmingRpcsBotInfo{
		BotId:      id,
		Dimensions: dimensions,
	}
}

var commitDate = time.Unix(1472647568, 0)

func commit(ctx context.Context, repoDir, message string) {
	assertNoError(exec.Run(ctx, &exec.Command{
		Name:        "git",
		Args:        []string{"commit", "-m", message},
		Env:         []string{fmt.Sprintf("GIT_AUTHOR_DATE=%d +0000", commitDate.Unix()), fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", commitDate.Unix())},
		InheritPath: true,
		Dir:         repoDir,
		Verbose:     exec.Silent,
	}))
	commitDate = commitDate.Add(10 * time.Second)
}

func makeDummyCommits(ctx context.Context, repoDir string, numCommits int) {
	_, err := exec.RunCwd(ctx, repoDir, "git", "checkout", "master")
	assertNoError(err)
	dummyFile := path.Join(repoDir, "dummyfile.txt")
	for i := 0; i < numCommits; i++ {
		title := fmt.Sprintf("Dummy #%d", i)
		assertNoError(ioutil.WriteFile(dummyFile, []byte(title), os.ModePerm))
		_, err = exec.RunCwd(ctx, repoDir, "git", "add", dummyFile)
		assertNoError(err)
		commit(ctx, repoDir, title)
		_, err = exec.RunCwd(ctx, repoDir, "git", "push", "origin", "master")
		assertNoError(err)
	}
}

func run(ctx context.Context, dir string, cmd ...string) {
	if _, err := exec.RunCwd(ctx, dir, cmd...); err != nil {
		sklog.Fatal(err)
	}
}

func addFile(ctx context.Context, repoDir, subPath, contents string) {
	assertNoError(ioutil.WriteFile(path.Join(repoDir, subPath), []byte(contents), os.ModePerm))
	run(ctx, repoDir, "git", "add", subPath)
}

func main() {
	common.Init()

	// Create a repo with lots of commits.
	workdir, err := ioutil.TempDir("", "")
	assertNoError(err)
	defer func() {
		if err := os.RemoveAll(workdir); err != nil {
			sklog.Fatal(err)
		}
	}()
	ctx := context.Background()
	repoName := "skia.git"
	repoDir := path.Join(workdir, repoName)
	assertNoError(os.Mkdir(path.Join(workdir, repoName), os.ModePerm))
	run(ctx, repoDir, "git", "init")
	run(ctx, repoDir, "git", "remote", "add", "origin", ".")

	// Write some files.
	assertNoError(ioutil.WriteFile(path.Join(workdir, ".gclient"), []byte("dummy"), os.ModePerm))
	addFile(ctx, repoDir, "a.txt", "dummy2")
	addFile(ctx, repoDir, "somefile.txt", "dummy3")
	infraBotsSubDir := path.Join("infra", "bots")
	infraBotsDir := path.Join(repoDir, infraBotsSubDir)
	assertNoError(os.MkdirAll(infraBotsDir, os.ModePerm))
	addFile(ctx, repoDir, path.Join(infraBotsSubDir, "compile_skia.isolate"), `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	addFile(ctx, repoDir, path.Join(infraBotsSubDir, "perf_skia.isolate"), `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	addFile(ctx, repoDir, path.Join(infraBotsSubDir, "test_skia.isolate"), `{
  'includes': [
    'swarm_recipe.isolate',
  ],
  'variables': {
    'files': [
      '../../../.gclient',
    ],
  },
}`)
	addFile(ctx, repoDir, path.Join(infraBotsSubDir, "swarm_recipe.isolate"), `{
  'variables': {
    'command': [
      'python', 'recipes.py', 'run',
    ],
    'files': [
      '../../somefile.txt',
    ],
  },
}`)

	// Add tasks to the repo.
	var tasks = map[string]*specs.TaskSpec{
		"Build-Ubuntu-GCC-Arm7-Release-Android": {
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{},
			Dimensions:   []string{"pool:Skia", "os:Ubuntu"},
			Isolate:      "compile_skia.isolate",
			Priority:     0.9,
		},
		"Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{"Build-Ubuntu-GCC-Arm7-Release-Android"},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
			Isolate:      "test_skia.isolate",
			Priority:     0.9,
		},
		"Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{"Build-Ubuntu-GCC-Arm7-Release-Android"},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
			Isolate:      "perf_skia.isolate",
			Priority:     0.9,
		},
	}
	moarTasks := map[string]*specs.TaskSpec{}
	jobs := map[string]*specs.JobSpec{}
	for name, task := range tasks {
		for i := 0; i < 100; i++ {
			newName := fmt.Sprintf("%s%d", name, i)
			deps := make([]string, 0, len(task.Dependencies))
			for _, d := range task.Dependencies {
				deps = append(deps, fmt.Sprintf("%s%d", d, i))
			}
			newTask := &specs.TaskSpec{
				CipdPackages: task.CipdPackages,
				Dependencies: deps,
				Dimensions:   task.Dimensions,
				Isolate:      task.Isolate,
				Priority:     task.Priority,
			}
			moarTasks[newName] = newTask
			jobs[newName] = &specs.JobSpec{
				Priority:  task.Priority,
				TaskSpecs: []string{newName},
			}
		}
	}
	cfg := specs.TasksCfg{
		Tasks: moarTasks,
		Jobs:  jobs,
	}
	f, err := os.Create(path.Join(repoDir, specs.TASKS_CFG_FILE))
	assertNoError(err)
	assertNoError(json.NewEncoder(f).Encode(&cfg))
	assertNoError(f.Close())
	run(ctx, repoDir, "git", "add", specs.TASKS_CFG_FILE)
	commit(ctx, repoDir, "Add more tasks!")
	run(ctx, repoDir, "git", "push", "origin", "master")
	run(ctx, repoDir, "git", "branch", "-u", "origin/master")

	// Create a bunch of bots.
	bots := make([]*swarming_api.SwarmingRpcsBotInfo, 100)
	for idx := range bots {
		dims := map[string]string{
			"pool": "Skia",
		}
		if idx >= 50 {
			dims["os"] = "Ubuntu"
		} else {
			dims["os"] = "Android"
			dims["device_type"] = "grouper"
		}
		bots[idx] = makeBot(fmt.Sprintf("bot%d", idx), dims)
	}

	// Create the task scheduler.
	repo, err := repograph.NewGraph(ctx, repoName, workdir)
	assertNoError(err)
	assertNoError(repo.Update(ctx))
	head, err := repo.Repo().RevParse(ctx, "HEAD")
	assertNoError(err)

	commits, err := repo.Repo().RevList(ctx, head)
	assertNoError(err)
	assertDeepEqual([]string{head}, commits)

	d, err := local_db.NewDB("testdb", path.Join(workdir, "tasks.db"), nil)
	assertNoError(err)
	w, err := window.New(time.Hour, 0, nil)
	assertNoError(err)
	tCache, err := cache.NewTaskCache(d, w)
	assertNoError(err)
	// Use dummy GetRevisionTimestamp function so that nothing ever expires from
	// the cache.
	dummyGetRevisionTimestamp := func(string, string) (time.Time, error) {
		return time.Now(), nil
	}
	jCache, err := cache.NewJobCache(d, w, dummyGetRevisionTimestamp)
	assertNoError(err)

	isolateClient, err := isolate.NewClient(workdir, isolate.ISOLATE_SERVER_URL_FAKE)
	assertNoError(err)
	swarmingClient := testutils.NewTestClient()

	// This is okay since this only runs locally.
	root, err := repo_root.Get()
	assertNoError(err)
	recipesCfgFile := filepath.Join(root, recipe_cfg.RECIPE_CFG_PATH)
	depotTools, err := depot_tools.Sync(ctx, workdir, recipesCfgFile)
	assertNoError(err)
	urlMock := mockhttpclient.NewURLMock()
	gitcookies := path.Join(workdir, "gitcookies_fake")
	assertNoError(ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit("https://fake-skia-review.googlesource.com", gitcookies, urlMock.Client())
	assertNoError(err)
	s, err := scheduling.NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, workdir, "fake.server", repograph.Map{repoName: repo}, isolateClient, swarmingClient, http.DefaultClient, 0.9, tryjobs.API_URL_TESTING, tryjobs.BUCKET_TESTING, map[string]string{"skia": repoName}, swarming.POOLS_PUBLIC, "", depotTools, g, "test-project", "test-instance", nil)
	assertNoError(err)

	runTasks := func(bots []*swarming_api.SwarmingRpcsBotInfo) {
		swarmingClient.MockBots(bots)
		assertNoError(s.MainLoop(ctx))
		assertNoError(w.Update())
		assertNoError(tCache.Update())
		tasks, err := tCache.GetTasksForCommits(repoName, commits)
		assertNoError(err)
		newTasks := map[string]*types.Task{}
		for _, v := range tasks {
			for _, task := range v {
				if task.Status == types.TASK_STATUS_PENDING {
					if _, ok := newTasks[task.Id]; !ok {
						newTasks[task.Id] = task
					}
				}
			}
		}
		insert := make([]*types.Task, 0, len(newTasks))
		for _, task := range newTasks {
			task.Status = types.TASK_STATUS_SUCCESS
			task.Finished = time.Now()
			task.IsolatedOutput = "abc123"
			insert = append(insert, task)
		}
		assertNoError(d.PutTasks(insert))
		assertNoError(tCache.Update())
		assertNoError(jCache.Update())
	}

	// Consume all tasks.
	for {
		runTasks(bots)
		unfinished, err := jCache.UnfinishedJobs()
		assertNoError(err)
		sklog.Infof("Found %d unfinished jobs.", len(unfinished))
		if len(unfinished) == 0 {
			tasks, err := tCache.GetTasksForCommits(repoName, commits)
			assertNoError(err)
			assertEqual(s.QueueLen(), 0)
			assertEqual(len(moarTasks), len(tasks[head]))
			break
		}
	}

	// Add more commits to the repo.
	makeDummyCommits(ctx, repoDir, 200)
	commits, err = repo.Repo().RevList(ctx, fmt.Sprintf("%s..HEAD", head))
	assertNoError(err)

	// Start the profiler.
	go func() {
		sklog.Fatal(http.ListenAndServe("localhost:6060", nil))
	}()

	// Actually run the test.
	i := 0
	for ; ; i++ {
		runTasks(bots)
		if s.QueueLen() == 0 {
			break
		}
	}
	sklog.Infof("Finished in %d iterations.", i)
}
