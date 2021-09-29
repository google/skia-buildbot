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
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/job_creation"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/task_cfg_cache"
	tcc_testutils "go.skia.org/infra/task_scheduler/go/task_cfg_cache/testutils"
	swarming_task_execution "go.skia.org/infra/task_scheduler/go/task_execution/swarming"
	"go.skia.org/infra/task_scheduler/go/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	rbeInstance = "projects/chromium-swarm-dev/instances/default_instance"
)

var (
	cpuprofile     = flag.String("cpuprofile", "", "write cpu profile to file")
	tasksPerCommit = flag.Int("tasks_per_commit", 300, "Number of tasks defined per commit.")
	numCommits     = flag.Int("num_commits", 200, "Number of commits.")
	maxRounds      = flag.Int("max_cycles", 0, "Stop after this many scheduling cycles.")
	recipesCfgFile = flag.String("recipes_cfg_file", "", "Path to the recipes.cfg file. If not provided, attempt to find it automatically.")
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

func makeCommits(ctx context.Context, repoDir string, numCommits int) {
	gd := git.GitDir(repoDir)
	_, err := gd.Git(ctx, "checkout", git.MainBranch)
	assertNoError(err)
	fakeFile := path.Join(repoDir, "fakefile.txt")
	for i := 0; i < numCommits; i++ {
		title := fmt.Sprintf("Fake #%d", i)
		assertNoError(ioutil.WriteFile(fakeFile, []byte(title), os.ModePerm))
		_, err = gd.Git(ctx, "add", fakeFile)
		assertNoError(err)
		commit(ctx, repoDir, title)
		_, err = gd.Git(ctx, "push", git.DefaultRemote, git.MainBranch)
		assertNoError(err)
	}
}

func addFile(ctx context.Context, repoDir, subPath, contents string) {
	assertNoError(ioutil.WriteFile(path.Join(repoDir, subPath), []byte(contents), os.ModePerm))
	_, err := git.GitDir(repoDir).Git(ctx, "add", subPath)
	assertNoError(err)
}

// waitForNewJobs waits for new jobs to appear in the DB and cache after new
// commits have landed.
func waitForNewJobs(ctx context.Context, repos repograph.Map, jc *job_creation.JobCreator, jCache cache.JobCache, expectJobs int) {
	for repoDir, repo := range repos {
		assertNoError(repo.Update(ctx))
		assertNoError(jc.HandleRepoUpdate(ctx, repoDir, repo, func() {}, func() {}))
	}
	sklog.Infof("Waiting for QuerySnapshotIterator...")
	for {
		time.Sleep(500 * time.Millisecond)
		assertNoError(jCache.Update(ctx))
		unfinished, err := jCache.UnfinishedJobs()
		assertNoError(err)
		if len(unfinished) == expectJobs {
			break
		}
	}
}

// jobSlice implements sort.Interface to sort Jobs by name.
type jobSlice []*types.Job

func (s jobSlice) Len() int { return len(s) }

func (s jobSlice) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func (s jobSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// taskSummarySlice implements sort.Interface to sort TaskSummary by name.
type taskSummarySlice []*types.TaskSummary

func (s taskSummarySlice) Len() int { return len(s) }

func (s taskSummarySlice) Less(i, j int) bool {
	return s[i].Attempt < s[j].Attempt
}

func (s taskSummarySlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func main() {
	common.Init()

	// Create a repo with one commit.
	workdir, err := ioutil.TempDir("", "")
	assertNoError(err)
	defer func() {
		if err := os.RemoveAll(workdir); err != nil {
			sklog.Fatal(err)
		}
	}()
	ctx := now.TimeTravelingContext(commitDate.Add(24 * time.Hour))
	repoName := "skia.git"
	repoDir := filepath.Join(workdir, repoName)
	assertNoError(os.Mkdir(repoDir, os.ModePerm))
	gd := git.GitDir(repoDir)
	_, err = gd.Git(ctx, "init")
	assertNoError(err)
	// This sets the remote repo to be the repo itself, which prevents us
	// needing to have a separate remote repo, either locally or on a server
	// somewhere.
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
	taskNames := []string{
		tcc_testutils.BuildTaskName,
		tcc_testutils.TestTaskName,
		tcc_testutils.PerfTaskName,
	}
	taskTypes := []*specs.TaskSpec{
		{
			CasSpec:      "compile",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{},
			Dimensions:   []string{"pool:Skia", "os:Ubuntu"},
			Priority:     1.0,
		},
		{
			CasSpec:      "test",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{tcc_testutils.BuildTaskName},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
			Priority:     0.7,
		},
		{
			CasSpec:      "perf",
			CipdPackages: []*specs.CipdPackage{},
			Dependencies: []string{tcc_testutils.BuildTaskName},
			Dimensions:   []string{"pool:Skia", "os:Android", "device_type:grouper"},
			Priority:     0.5,
		},
	}
	// Add the requested number of tasks to the TasksCfg, cycling through Build,
	// Test, and Perf tasks to keep things interesting.
	moarTasks := map[string]*specs.TaskSpec{}
	jobs := map[string]*specs.JobSpec{}
	taskCycleIndex := -1
	for numTasks := 0; numTasks < *tasksPerCommit; numTasks++ {
		taskType := numTasks % 3
		if taskType == 0 {
			taskCycleIndex++
		}
		name := taskNames[taskType]
		task := taskTypes[taskType]
		newName := fmt.Sprintf("%s%d", name, taskCycleIndex)
		deps := make([]string, 0, len(task.Dependencies))
		for _, d := range task.Dependencies {
			deps = append(deps, fmt.Sprintf("%s%d", d, taskCycleIndex))
		}
		priority := task.Priority * math.Pow(0.99999999, float64(numTasks))
		newTask := &specs.TaskSpec{
			CasSpec:      task.CasSpec,
			CipdPackages: task.CipdPackages,
			Dependencies: deps,
			Dimensions:   task.Dimensions,
			Priority:     priority,
		}
		moarTasks[newName] = newTask
		jobs[newName] = &specs.JobSpec{
			Priority:  priority,
			TaskSpecs: []string{newName},
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
	_, err = gd.Git(ctx, "push", git.DefaultRemote, git.MainBranch)
	assertNoError(err)
	_, err = gd.Git(ctx, "branch", "-u", git.DefaultRemote+"/"+git.MainBranch)
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
	repo, err := repograph.NewLocalGraph(ctx, repoDir, workdir)
	assertNoError(err)
	assertNoError(repo.Update(ctx))
	headCommit := repo.Get(git.MainBranch)
	if headCommit == nil {
		sklog.Fatalf("Could not find HEAD of %s.", git.MainBranch)
	}
	head := headCommit.Hash

	commits, err := repo.Get(head).AllCommits()
	assertNoError(err)
	assertDeepEqual([]string{head}, commits)

	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore)
	fsInstance := uuid.New().String()
	d, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, fsInstance, ts)
	assertNoError(err)
	windowPeriod := time.Duration(math.MaxInt64)
	w, err := window.New(ctx, windowPeriod, 0, nil)
	assertNoError(err)
	tCache, err := cache.NewTaskCache(ctx, d, w, nil)
	assertNoError(err)
	jCache, err := cache.NewJobCache(ctx, d, w, nil)
	assertNoError(err)

	swarmingClient := testutils.NewTestClient()

	repos := repograph.Map{repoDir: repo}

	btProject := "test-project"
	btInstance := "test-instance"
	assertNoError(bt.InitBigtable(btProject, btInstance, task_cfg_cache.BT_TABLE, []string{task_cfg_cache.BT_COLUMN_FAMILY}))
	defer func() {
		assertNoError(bt.DeleteTables(btProject, btInstance, task_cfg_cache.BT_TABLE))
	}()
	taskCfgCache, err := task_cfg_cache.NewTaskCfgCache(ctx, repos, btProject, btInstance, nil)
	if err != nil {
		sklog.Fatalf("Failed to create TaskCfgCache: %s", err)
	}
	cas, err := rbe.NewClient(ctx, rbeInstance, ts)
	assertNoError(err)
	taskExec := swarming_task_execution.NewSwarmingTaskExecutor(swarmingClient, rbeInstance, "")
	s, err := scheduling.NewTaskScheduler(ctx, d, nil, windowPeriod, 0, repos, cas, rbeInstance, taskExec, http.DefaultClient, 0.99999, swarming.POOLS_PUBLIC, "", taskCfgCache, nil, nil, "")
	assertNoError(err)

	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()

	if *recipesCfgFile == "" {
		_, filename, _, _ := runtime.Caller(0)
		*recipesCfgFile = filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "infra", "config", "recipes.cfg")
	}
	depotTools, err := depot_tools.GetDepotTools(ctx, workdir, *recipesCfgFile)
	assertNoError(err)
	jc, err := job_creation.NewJobCreator(ctx, d, windowPeriod, 0, workdir, "localhost", repos, cas, client, "", "", nil, depotTools, nil, taskCfgCache, ts)
	assertNoError(err)

	// Wait for job-creator to process the jobs from the repo.
	waitForNewJobs(ctx, repos, jc, jCache, *tasksPerCommit)

	runTasks := func(bots []*swarming_api.SwarmingRpcsBotInfo) {
		swarmingClient.MockBots(bots)
		assertNoError(s.MainLoop(ctx))
		assertNoError(w.Update(ctx))
		assertNoError(tCache.Update(ctx))
		tasks, err := tCache.GetTasksForCommits(repoDir, commits)
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
			task.Finished = now.Now(ctx)
			task.IsolatedOutput = rbe.EmptyDigest
			insert = append(insert, task)
		}
		assertNoError(d.PutTasks(ctx, insert))
		assertNoError(tCache.Update(ctx))
		assertNoError(jCache.Update(ctx))
	}

	assertNoError(jCache.Update(ctx))
	allJobs, err := jCache.GetJobsFromDateRange(time.Time{}, time.Now())
	assertNoError(err)
	sklog.Infof("Found %d total jobs", len(allJobs))
	assertEqual(*tasksPerCommit, len(allJobs))

	// Consume all tasks.
	for {
		sklog.Infof("Running all tasks...")
		runTasks(bots)
		unfinished, err := jCache.UnfinishedJobs()
		assertNoError(err)
		sklog.Infof("Found %d unfinished jobs.", len(unfinished))
		if len(unfinished) == 0 {
			tasks, err := tCache.GetTasksForCommits(repoDir, commits)
			assertNoError(err)
			assertEqual(s.QueueLen(), 0)
			assertEqual(len(moarTasks), len(tasks[head]))
			break
		}
	}
	sklog.Infof("Done consuming initial set of jobs.")

	// Add more commits to the repo.
	makeCommits(ctx, repoDir, *numCommits)
	waitForNewJobs(ctx, repos, jc, jCache, (*numCommits)*(*tasksPerCommit))
	commits, err = repo.RevList(head, git.MainBranch)
	assertNoError(err)

	// Actually run the test.
	sklog.Infof("Starting test...")
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		assertNoError(err)
		assertNoError(pprof.StartCPUProfile(f))
		defer pprof.StopCPUProfile()
	}
	schedulingRounds := 1
	started := time.Now()
	for ; *maxRounds > 0 && schedulingRounds <= *maxRounds; schedulingRounds++ {
		runTasks(bots)
		if s.QueueLen() == 0 {
			break
		}
	}
	elapsed := time.Now().Sub(started)
	sklog.Infof("Finished %d scheduling cycles in %s.", schedulingRounds, elapsed)
}
