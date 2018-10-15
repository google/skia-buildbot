// busywork is an end-to-end test for local_db. It performs inserts and updates
// roughly mimicking what we might expect from task_scheduler. It also tracks
// performance for various operations.
package main

import (
	"container/heap"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"path"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/scheduling"
	"go.skia.org/infra/task_scheduler/go/window"
)

var (
	// Flags.
	local    = flag.Bool("local", true, "Whether we're running on a dev machine vs in production.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workdir  = flag.String("workdir", "workdir", "Working directory to use.")

	// Counters.
	inserts            = 0
	insertDur          = time.Duration(0)
	mInserts           = sync.RWMutex{}
	insertAndUpdates   = 0
	insertAndUpdateDur = time.Duration(0)
	mInsertAndUpdates  = sync.RWMutex{}
	updates            = 0
	updateDur          = time.Duration(0)
	mUpdates           = sync.RWMutex{}
	reads              = 0
	readDur            = time.Duration(0)
	mReads             = sync.RWMutex{}

	// epoch is a time before local_db was written.
	epoch = time.Date(2016, 8, 1, 0, 0, 0, 0, time.UTC)
)

const (
	// Parameters for creating random tasks.
	kNumTaskNames          = 50
	kNumRepos              = 3
	kRecentCommitRange     = 30
	kMedianBlamelistLength = 2

	// Parameters for randomly updating tasks.
	kMedianPendingDuration = 10 * time.Second
	kMedianRunningDuration = 10 * time.Minute
)

// itoh converts an integer to a commit hash. Task.Revision is always set to
// the result of itoh.
func itoh(i int) string {
	return strconv.Itoa(i)
}

// htoi converts a commit hash to an integer. A commit's parent is
// itoh(htoi(hash)-1).
func htoi(h string) int {
	i, err := strconv.Atoi(h)
	if err != nil {
		sklog.Fatal(err)
	}
	return i
}

// makeTask generates task with random Name, Repo, and Revision. Revision will
// be picked randomly from a range starting at recentCommitsBegin.
func makeTask(recentCommitsBegin int) *db.Task {
	return &db.Task{
		TaskKey: db.TaskKey{
			RepoState: db.RepoState{
				Repo:     fmt.Sprintf("Repo-%d", rand.Intn(kNumRepos)),
				Revision: itoh(recentCommitsBegin + rand.Intn(kRecentCommitRange)),
			},
			Name: fmt.Sprintf("Task-%d", rand.Intn(kNumTaskNames)),
		},
	}
}

// updateBlamelists sets t's Commits based on t.Revision and previously-inserted
// tasks' Commits and returns t. If another task's Commits needs to change, also
// returns that task with its updated Commits.
func updateBlamelists(cache db.TaskCache, t *db.Task) ([]*db.Task, error) {
	if !cache.KnownTaskName(t.Repo, t.Name) {
		t.Commits = []string{t.Revision}
		return []*db.Task{t}, nil
	}
	stealFrom, err := cache.GetTaskForCommit(t.Repo, t.Revision, t.Name)
	if err != nil {
		return nil, fmt.Errorf("Could not find task %q for commit %q: %s", t.Name, t.Revision, err)
	}

	lastCommit := htoi(t.Revision)
	firstCommit := lastCommit
	// Work backwards until prev changes.
	for i := lastCommit - 1; i > 0; i-- {
		if lastCommit-firstCommit+1 > scheduling.MAX_BLAMELIST_COMMITS && stealFrom == nil {
			t.Commits = []string{t.Revision}
			return []*db.Task{t}, nil
		}
		hash := itoh(i)
		prev, err := cache.GetTaskForCommit(t.Repo, hash, t.Name)
		if err != nil {
			return nil, fmt.Errorf("Could not find task %q for commit %q: %s", t.Name, hash, err)
		}
		if stealFrom != prev {
			break
		}
		firstCommit = i
	}

	t.Commits = make([]string, lastCommit-firstCommit+1)
	for i := 0; i <= lastCommit-firstCommit; i++ {
		t.Commits[i] = itoh(i + firstCommit)
	}
	sort.Strings(t.Commits)

	if stealFrom != nil {
		newCommits := make([]string, 0, len(stealFrom.Commits)-len(t.Commits))
		for _, h := range stealFrom.Commits {
			idx := sort.SearchStrings(t.Commits, h)
			if idx == len(t.Commits) || t.Commits[idx] != h {
				newCommits = append(newCommits, h)
			}
		}
		stealFrom.Commits = newCommits
		return []*db.Task{t, stealFrom}, nil
	} else {
		return []*db.Task{t}, nil
	}
}

// findApproxLatestCommit scans the DB backwards and returns the commit # of the
// last-created task.
func findApproxLatestCommit(d db.TaskDB) int {
	sklog.Infof("findApproxLatestCommit begin")
	for t := time.Now(); t.After(epoch); t = t.Add(-24 * time.Hour) {
		begin := t.Add(-24 * time.Hour)
		sklog.Infof("findApproxLatestCommit loading %s to %s", begin, t)
		before := time.Now()
		t, err := d.GetTasksFromDateRange(begin, t, "")
		getTasksDur := time.Now().Sub(before)
		if err != nil {
			sklog.Fatal(err)
		}
		mReads.Lock()
		if len(t) > 0 {
			reads += len(t)
		} else {
			reads++
		}
		readDur += getTasksDur
		mReads.Unlock()
		if len(t) > 0 {
			// Return revision of last task.
			lastTask := t[len(t)-1]
			i := htoi(lastTask.Revision)
			sklog.Infof("findApproxLatestCommit returning %d from %s", i, lastTask.Id)
			return i
		}

	}
	sklog.Infof("findApproxLatestCommit found empty DB")
	return 0
}

// putTasks inserts randomly-generated tasks into the DB. Does not return.
func putTasks(d db.TaskDB) {
	sklog.Infof("putTasks begin")
	w, err := window.New(4*24*time.Hour, 0, nil)
	if err != nil {
		sklog.Fatal(err)
	}
	cache, err := db.NewTaskCache(d, w)
	if err != nil {
		sklog.Fatal(err)
	}
	// If we're restarting, try to pick up where we left off.
	currentCommit := findApproxLatestCommit(d)
	meanTasksPerCommit := float64(kNumTaskNames * kNumRepos / kMedianBlamelistLength)
	maxTasksPerIter := float64(kNumTaskNames * kNumRepos * kRecentCommitRange)
	for {
		if err := w.Update(); err != nil {
			sklog.Fatal(err)
		}
		iterTasks := int(math.Max(0, math.Min(maxTasksPerIter, (rand.NormFloat64()+1)*meanTasksPerCommit)))
		sklog.Infof("Adding %d tasks with revisions %s - %s", iterTasks, itoh(currentCommit), itoh(currentCommit+kRecentCommitRange))
		for i := 0; i < iterTasks; i++ {
			t := makeTask(currentCommit)
			putTasksDur := time.Duration(0)
			before := time.Now()
			updatedTasks, err := db.UpdateTasksWithRetries(d, func() ([]*db.Task, error) {
				putTasksDur += time.Now().Sub(before)
				t := t.Copy()
				if err := cache.Update(); err != nil {
					sklog.Fatal(err)
				}
				tasksToUpdate, err := updateBlamelists(cache, t)
				if err != nil {
					sklog.Fatal(err)
				}
				before = time.Now()
				if err := d.AssignId(t); err != nil {
					sklog.Fatal(err)
				}
				putTasksDur += time.Now().Sub(before)
				t.Created = time.Now()
				t.SwarmingTaskId = fmt.Sprintf("%x", rand.Int31())
				before = time.Now()
				return tasksToUpdate, nil
			})
			putTasksDur += time.Now().Sub(before)
			if err != nil {
				sklog.Fatal(err)
			}
			if len(updatedTasks) > 1 {
				mInsertAndUpdates.Lock()
				if err == nil {
					insertAndUpdates += len(updatedTasks)
				}
				insertAndUpdateDur += putTasksDur
				mInsertAndUpdates.Unlock()
			} else {
				mInserts.Lock()
				if err == nil {
					inserts++
				}
				insertDur += putTasksDur
				mInserts.Unlock()
			}
		}
		currentCommit++
	}
}

// updateEntry is an item in updateEntryHeap.
type updateEntry struct {
	task *db.Task
	// updateTime is the key for updateEntryHeap.
	updateTime time.Time
	// heapIndex is the index of this updateEntry in updateEntryHeap. It is kept
	// up-to-date by updateEntryHeap methods.
	heapIndex int
}

// updateEntryHeap implements a queue of updateEntry's ordered by updateTime. It
// implements heap.Interface.
type updateEntryHeap []*updateEntry

func (h updateEntryHeap) Len() int           { return len(h) }
func (h updateEntryHeap) Less(i, j int) bool { return h[i].updateTime.Before(h[j].updateTime) }
func (h updateEntryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *updateEntryHeap) Push(x interface{}) {
	item := x.(*updateEntry)
	item.heapIndex = len(*h)
	*h = append(*h, item)
}

func (h *updateEntryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	x.heapIndex = -1
	return x
}

// updateTasks makes random updates to pending and running tasks in the DB. Does
// not return.
func updateTasks(d db.TaskDB) {
	sklog.Infof("updateTasks begin")
	updateQueue := updateEntryHeap{}
	idMap := map[string]*updateEntry{}

	freshenQueue := func(task *db.Task) {
		entry := idMap[task.Id]
		// Currently only updating pending and running tasks.
		if task.Status == db.TASK_STATUS_PENDING || task.Status == db.TASK_STATUS_RUNNING {
			meanUpdateDelay := kMedianPendingDuration
			if task.Status == db.TASK_STATUS_RUNNING {
				meanUpdateDelay = kMedianRunningDuration
			}
			updateDelayNanos := int64(math.Max(0, (rand.NormFloat64()+1)*float64(meanUpdateDelay)))
			updateTime := time.Now().Add(time.Duration(updateDelayNanos) * time.Nanosecond)
			if entry == nil {
				entry = &updateEntry{
					task:       task,
					updateTime: updateTime,
					heapIndex:  -1,
				}
				heap.Push(&updateQueue, entry)
			} else {
				entry.task = task
				entry.updateTime = updateTime
				heap.Fix(&updateQueue, entry.heapIndex)
			}
			if entry.heapIndex < 0 {
				sklog.Fatalf("you lose %#v %#v", entry, updateQueue)
			}
			idMap[task.Id] = entry
		} else if entry != nil {
			heap.Remove(&updateQueue, entry.heapIndex)
			delete(idMap, task.Id)
		}
	}

	token, err := d.StartTrackingModifiedTasks()
	if err != nil {
		sklog.Fatal(err)
	}
	// Initial read to find pending and running tasks.
	for t := time.Now(); t.After(epoch); t = t.Add(-24 * time.Hour) {
		begin := t.Add(-24 * time.Hour)
		sklog.Infof("updateTasks loading %s to %s", begin, t)
		before := time.Now()
		t, err := d.GetTasksFromDateRange(begin, t, "")
		getTasksDur := time.Now().Sub(before)
		if err != nil {
			sklog.Fatal(err)
		}
		mReads.Lock()
		if len(t) > 0 {
			reads += len(t)
		} else {
			reads++
		}
		readDur += getTasksDur
		mReads.Unlock()
		for _, task := range t {
			freshenQueue(task)
		}
	}
	sklog.Infof("updateTasks finished loading; %d pending and running", len(idMap))
	// Rate limit so we're not constantly taking locks for GetModifiedTasks.
	for range time.Tick(time.Millisecond) {
		now := time.Now()
		t, err := d.GetModifiedTasks(token)
		if err != nil {
			sklog.Fatal(err)
		}
		for _, task := range t {
			freshenQueue(task)
		}
		sklog.Infof("updateTasks performing updates; %d tasks on queue", len(updateQueue))
		for len(updateQueue) > 0 && updateQueue[0].updateTime.Before(now) {
			if time.Now().Sub(now) >= db.MODIFIED_DATA_TIMEOUT-5*time.Second {
				break
			}
			entry := heap.Pop(&updateQueue).(*updateEntry)
			task := entry.task
			delete(idMap, task.Id)
			putTasksDur := time.Duration(0)
			before := time.Now()
			_, err := db.UpdateTaskWithRetries(d, task.Id, func(task *db.Task) error {
				putTasksDur += time.Now().Sub(before)
				switch task.Status {
				case db.TASK_STATUS_PENDING:
					task.Started = now
					isMishap := rand.Intn(100) == 0
					if isMishap {
						task.Status = db.TASK_STATUS_MISHAP
						task.Finished = now
					} else {
						task.Status = db.TASK_STATUS_RUNNING
					}
				case db.TASK_STATUS_RUNNING:
					task.Finished = now
					statusRand := rand.Intn(25)
					isMishap := statusRand == 0
					isFailure := statusRand < 5
					if isMishap {
						task.Status = db.TASK_STATUS_MISHAP
					} else if isFailure {
						task.Status = db.TASK_STATUS_FAILURE
					} else {
						task.Status = db.TASK_STATUS_SUCCESS
						task.IsolatedOutput = fmt.Sprintf("%x", rand.Int63())
					}
				default:
					sklog.Fatalf("Task %s in update queue has status %s. %#v", task.Id, task.Status, task)
				}
				before = time.Now()
				return nil
			})
			putTasksDur += time.Now().Sub(before)
			if err != nil {
				sklog.Fatal(err)
			}
			mUpdates.Lock()
			updates++
			updateDur += putTasksDur
			mUpdates.Unlock()
		}
	}
}

// readTasks reads the last hour of tasks every second. Does not return.
func readTasks(d db.TaskDB) {
	sklog.Infof("readTasks begin")
	var taskCount uint64 = 0
	var readCount uint64 = 0
	var totalDuration time.Duration = 0
	lastMessage := time.Now()
	for range time.Tick(time.Second) {
		now := time.Now()
		t, err := d.GetTasksFromDateRange(now.Add(-time.Hour), now, "")
		dur := time.Now().Sub(now)
		if err != nil {
			sklog.Fatal(err)
		}
		taskCount += uint64(len(t))
		readCount++
		totalDuration += dur
		mReads.Lock()
		reads += len(t)
		readDur += dur
		mReads.Unlock()
		if now.Sub(lastMessage) > time.Minute {
			lastMessage = now
			if readCount > 0 && totalDuration > 0 {
				sklog.Infof("readTasks %d tasks in last hour; %f reads/sec; %f tasks/sec", taskCount/readCount, float64(readCount)/totalDuration.Seconds(), float64(taskCount)/totalDuration.Seconds())
			} else {
				sklog.Fatalf("readTasks 0 reads in last minute")
			}
			taskCount = 0
			readCount = 0
			totalDuration = 0
		}
	}
}

// reportStats logs the performance of the DB as seen by putTasks, updateTasks,
// and readTasks. Does not return.
func reportStats() {
	lastInserts := 0
	lastInsertDur := time.Duration(0)
	lastInsertAndUpdates := 0
	lastInsertAndUpdateDur := time.Duration(0)
	lastUpdates := 0
	lastUpdateDur := time.Duration(0)
	lastReads := 0
	lastReadDur := time.Duration(0)
	for range time.Tick(5 * time.Second) {
		mInserts.RLock()
		totalInserts := inserts
		totalInsertDur := insertDur
		mInserts.RUnlock()
		mInsertAndUpdates.RLock()
		totalInsertAndUpdates := insertAndUpdates
		totalInsertAndUpdateDur := insertAndUpdateDur
		mInsertAndUpdates.RUnlock()
		mUpdates.RLock()
		totalUpdates := updates
		totalUpdateDur := updateDur
		mUpdates.RUnlock()
		mReads.RLock()
		totalReads := reads
		totalReadDur := readDur
		mReads.RUnlock()
		curInserts := totalInserts - lastInserts
		lastInserts = totalInserts
		curInsertDur := totalInsertDur - lastInsertDur
		lastInsertDur = totalInsertDur
		curInsertAndUpdates := totalInsertAndUpdates - lastInsertAndUpdates
		lastInsertAndUpdates = totalInsertAndUpdates
		curInsertAndUpdateDur := totalInsertAndUpdateDur - lastInsertAndUpdateDur
		lastInsertAndUpdateDur = totalInsertAndUpdateDur
		curUpdates := totalUpdates - lastUpdates
		lastUpdates = totalUpdates
		curUpdateDur := totalUpdateDur - lastUpdateDur
		lastUpdateDur = totalUpdateDur
		curReads := totalReads - lastReads
		lastReads = totalReads
		curReadDur := totalReadDur - lastReadDur
		lastReadDur = totalReadDur
		sklog.Infof("reportStats total; %d inserts %f/s; %d insert-and-updates %f/s; %d updates %f/s; %d reads %f/s", totalInserts, float64(totalInserts)/totalInsertDur.Seconds(), totalInsertAndUpdates, float64(totalInsertAndUpdates)/totalInsertAndUpdateDur.Seconds(), totalUpdates, float64(totalUpdates)/totalUpdateDur.Seconds(), totalReads, float64(totalReads)/totalReadDur.Seconds())
		if curInsertDur.Nanoseconds() == 0 {
			curInsertDur += time.Nanosecond
		}
		if curInsertAndUpdateDur.Nanoseconds() == 0 {
			curInsertAndUpdateDur += time.Nanosecond
		}
		if curUpdateDur.Nanoseconds() == 0 {
			curUpdateDur += time.Nanosecond
		}
		if curReadDur.Nanoseconds() == 0 {
			curReadDur += time.Nanosecond
		}
		sklog.Infof("reportStats current; %d inserts %f/s; %d insert-and-updates %f/s; %d updates %f/s; %d reads %f/s", curInserts, float64(curInserts)/curInsertDur.Seconds(), curInsertAndUpdates, float64(curInsertAndUpdates)/curInsertAndUpdateDur.Seconds(), curUpdates, float64(curUpdates)/curUpdateDur.Seconds(), curReads, float64(curReads)/curReadDur.Seconds())
	}
}

func main() {

	// Global init.
	common.InitWithMust(
		"busywork",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	d, err := local_db.NewDB("busywork", path.Join(*workdir, "busywork.bdb"))
	if err != nil {
		sklog.Fatal(err)
	}

	go reportStats()

	go putTasks(d)
	go updateTasks(d)
	go readTasks(d)

	// Block forever while goroutines do the work.
	select {}
}
