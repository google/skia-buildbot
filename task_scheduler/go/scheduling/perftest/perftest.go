package main

/*
	Performance test for TaskScheduler.
*/

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"reflect"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/davecgh/go-spew/spew"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	swarming_task_execution "go.skia.org/infra/task_scheduler/go/task_execution/swarming"
	"go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	fsInstance  = flag.String("firestore_instance", "", "Firestore instance to use, eg. \"testing\"")
	rbeInstance = flag.String("rbe_instance", "projects/chromium-swarm-dev/instances/default_instance", "CAS instance to use")
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
	gitExec, err := git.Executable(ctx)
	assertNoError(err)
	assertNoError(exec.Run(ctx, &exec.Command{
		Name:        gitExec,
		Args:        []string{"commit", "-m", message},
		Env:         []string{fmt.Sprintf("GIT_AUTHOR_DATE=%d +0000", commitDate.Unix()), fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", commitDate.Unix())},
		InheritPath: true,
		Dir:         repoDir,
		Verbose:     exec.Silent,
	}))
	commitDate = commitDate.Add(10 * time.Second)
}

func makeDummyCommits(ctx context.Context, repoDir string, numCommits int) {
	gd := git.GitDir(repoDir)
	_, err := gd.Git(ctx, "checkout", git.MasterBranch)
	assertNoError(err)
	dummyFile := path.Join(repoDir, "dummyfile.txt")
	for i := 0; i < numCommits; i++ {
		title := fmt.Sprintf("Dummy #%d", i)
		assertNoError(ioutil.WriteFile(dummyFile, []byte(title), os.ModePerm))
		_, err = gd.Git(ctx, "add", dummyFile)
		assertNoError(err)
		commit(ctx, repoDir, title)
		_, err = gd.Git(ctx, "push", git.DefaultRemote, git.MasterBranch)
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
	_, err := git.GitDir(repoDir).Git(ctx, "add", subPath)
	assertNoError(err)
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
	gd := git.GitDir(repoDir)
	_, err = gd.Git(ctx, "init")
	assertNoError(err)
	_, err = gd.Git(ctx, "remote", "add", git.DefaultRemote, ".")
	assertNoError(err)

	// Write some files.
	assertNoError(ioutil.WriteFile(path.Join(workdir, ".gclient"), []byte("placeholder"), os.ModePerm))
	addFile(ctx, repoDir, "a.txt", "placeholder2")
	addFile(ctx, repoDir, "somefile.txt", "placeholder3")
	infraBotsSubDir := path.Join("infra", "bots")
	infraBotsDir := path.Join(repoDir, infraBotsSubDir)
	assertNoError(os.MkdirAll(infraBotsDir, os.ModePerm))

	// CAS inputs.
	casSpecs := map[string]*specs.CasSpec{
		"compile": {
			Root:  ".",
			Paths: []string{"somefile.txt"},
		},
		"perf": {
			Root:  ".",
			Paths: []string{"somefile.txt"},
		},
		"test": {
			Root:  ".",
			Paths: []string{"somefile.txt"},
		},
	}

	// Add tasks to the repo.
	var tasks = map[string]*specs.TaskSpec{
		"Build-Ubuntu-GCC-Arm7-Release-Android": {
			CasSpec:      "compile",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{},
			Dimensions:   []string{"pool:Skia", "os:Ubuntu"},
			Priority:     0.9,
		},
		"Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
			CasSpec:      "test",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{"Build-Ubuntu-GCC-Arm7-Release-Android"},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
			Priority:     0.9,
		},
		"Perf-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release": {
			CasSpec:      "perf",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{"Build-Ubuntu-GCC-Arm7-Release-Android"},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
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
				CasSpec:      task.CasSpec,
				CipdPackages: task.CipdPackages,
				Dependencies: deps,
				Dimensions:   task.Dimensions,
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
		CasSpecs: casSpecs,
		Tasks:    moarTasks,
		Jobs:     jobs,
	}
	assertNoError(util.WithWriteFile(path.Join(repoDir, specs.TASKS_CFG_FILE), func(w io.Writer) error {
		return json.NewEncoder(w).Encode(&cfg)
	}))
	_, err = gd.Git(ctx, "add", specs.TASKS_CFG_FILE)
	assertNoError(err)
	commit(ctx, repoDir, "Add more tasks!")
	_, err = gd.Git(ctx, "push", git.DefaultRemote, git.MasterBranch)
	assertNoError(err)
	_, err = gd.Git(ctx, "branch", "-u", git.DefaultRemoteBranch)
	assertNoError(err)

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
	repo, err := repograph.NewLocalGraph(ctx, repoName, workdir)
	assertNoError(err)
	assertNoError(repo.Update(ctx))
	headCommit := repo.Get(git.MasterBranch)
	if headCommit == nil {
		sklog.Fatalf("Could not find HEAD of %s.", git.MasterBranch)
	}
	head := headCommit.Hash

	commits, err := repo.Get(head).AllCommits()
	assertNoError(err)
	assertDeepEqual([]string{head}, commits)

	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	d, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts)
	assertNoError(err)
	w, err := window.New(time.Hour, 0, nil)
	assertNoError(err)
	tCache, err := cache.NewTaskCache(ctx, d, w, nil)
	assertNoError(err)
	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	assertNoError(err)

	swarmingClient := testutils.NewTestClient()

	repos := repograph.Map{repoName: repo}
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, "test-project", "test-instance", nil)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCfgCache: %s", err)
	}
	cas, err := rbe.NewClient(ctx, *rbeInstance, ts)
	assertNoError(err)
	taskExec := swarming_task_execution.NewSwarmingTaskExecutor(swarmingClient, *rbeInstance, "")
	s, err := scheduling.NewTaskScheduler(ctx, d, nil, time.Duration(math.MaxInt64), 0, repos, cas, *rbeInstance, taskExec, http.DefaultClient, 0.9, swarming.POOLS_PUBLIC, "", taskCfgCache, nil, nil, "")
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
	commits, err = repo.RevList(head, git.MasterBranch)
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
