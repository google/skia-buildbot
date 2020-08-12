package snapshots

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/bigtable"
	"cloud.google.com/go/datastore"
	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	fs "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/task_scheduler/go/types"
)

type CommitQuery struct {
	cancel  func()
	clients map[string]func(*firestore.QuerySnapshot)
	mtx     sync.Mutex
}

func NewCommitQuery(client *fs.Client, hash string) *CommitQuery {
	ctx, cancel := context.WithCancel(context.TODO())
	rv := &CommitQuery{
		cancel:  cancel,
		clients: map[string]func(*firestore.QuerySnapshot){},
	}
	go func() {
		q := client.Collection("tasks").Where("Commits", "array-contains", hash)
		for snap := range fs.QuerySnapshotChannel(ctx, q) {
			rv.handleSnapshot(snap)
		}
	}()
	return rv
}

func (q *CommitQuery) handleSnapshot(snap *firestore.QuerySnapshot) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	var wg sync.WaitGroup
	for _, cb := range q.clients {
		wg.Add(1)
		go func(cb func(*firestore.QuerySnapshot)) {
			defer wg.Done()
			cb(snap)
		}(cb)
	}
	wg.Wait()
}

func (q *CommitQuery) AddClient(callback func(*firestore.QuerySnapshot)) string {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	id := uuid.New().String()
	q.clients[id] = callback
	return id
}

// Returns true iff the snapshot iterator was stopped.
func (q *CommitQuery) RemoveClient(id string) bool {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	delete(q.clients, id)
	if len(q.clients) == 0 {
		q.cancel()
		return true
	}
	return false
}

type CommitQueryManager struct {
	commits map[string]*CommitQuery
	mtx     sync.Mutex // Protects commits.
	client  *fs.Client
}

func (q *CommitQueryManager) Add(hash string, callback func(*firestore.QuerySnapshot)) string {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	c, ok := q.commits[hash]
	if !ok {
		c = NewCommitQuery(q.client, hash)
		q.commits[hash] = c
	}
	return c.AddClient(callback)
}

func (q *CommitQueryManager) Remove(hash, id string) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	c, ok := q.commits[hash]
	if ok && c.RemoveClient(id) {
		delete(q.commits, hash)
	}
}

type Query struct {
	commitQueryManager *CommitQueryManager
	commits            map[string]string // Maps commit hash to query ID.
	clients            map[string]func(*QueryDiff)
	mtx                sync.Mutex
	queryFunc          RepoQueryFunc
	taskRegistry       *TaskRegistry
}

type QueryDiff struct {
	// QuerySnapshot(s) for tasks, keyed by commit hash then task ID.
	Tasks map[string]map[string]*types.Task

	// New commits.
	AddedCommits map[string]*vcsinfo.LongCommit

	// Removed commits.
	RemovedCommits []string
}

func (q *Query) handleSnapshot(hash string, snap *firestore.QuerySnapshot) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	tasks := q.taskRegistry.FromSnapshot(snap)
	diff := &QueryDiff{
		Tasks: map[string]map[string]*types.Task{
			hash: tasks,
		},
	}
	var wg sync.WaitGroup
	for _, cb := range q.clients {
		wg.Add(1)
		go func(cb func(*QueryDiff)) {
			defer wg.Done()
			cb(diff)
		}(cb)
	}
	wg.Wait()
}

func (q *Query) AddClient(callback func(*QueryDiff)) string {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	id := uuid.New().String()
	q.clients[id] = callback
	return id
}

// Returns true iff the snapshot iterators were stopped.
func (q *Query) RemoveClient(id string) bool {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	delete(q.clients, id)
	if len(q.clients) == 0 {
		for hash, queryId := range q.commits {
			q.commitQueryManager.Remove(hash, queryId)
		}
		return true
	}
	return false
}

func (q *Query) handleRepoUpdate() {
	commits := q.queryFunc()
	q.mtx.Lock()
	defer q.mtx.Unlock()

	// Create queries for any new commits. Determine which commits need to
	// be removed.
	remove := make(map[string]string, len(q.commits))
	for hash, queryId := range q.commits {
		remove[hash] = queryId
	}
	added := make(map[string]*vcsinfo.LongCommit, len(commits))
	newCommits := make(map[string]string, len(commits))
	for _, c := range commits {
		queryId, ok := q.commits[c.Hash]
		if ok {
			delete(remove, c.Hash)
		} else {
			queryId = q.commitQueryManager.Add(c.Hash, q.handleSnapshot)
			added[c.Hash] = c
		}
		newCommits[c.Hash] = queryId
	}
	// Stop the queries for the removed commits.
	removed := make([]string, 0, len(remove))
	for hash, queryId := range remove {
		q.commitQueryManager.Remove(hash, queryId)
		removed = append(removed, hash)
	}
	q.commits = newCommits

	// Send the diffs to the clients.
	if len(added) != 0 || len(removed) != 0 {
		diff := &QueryDiff{
			AddedCommits:   added,
			RemovedCommits: removed,
		}
		var wg sync.WaitGroup
		for _, cb := range q.clients {
			wg.Add(1)
			go func(cb func(*QueryDiff)) {
				defer wg.Done()
				cb(diff)
			}(cb)
		}
		wg.Wait()
	}
}

type QueryManager struct {
	queries map[string]*Query
	mtx     sync.Mutex
}

func NewQueryManager(repos repograph.Map) *QueryManager {
	return &QueryManager{
		queries: map[string]*Query{},
	}
}

func (qm *QueryManager) add(descriptor string, fn RepoQueryFunc, cb func(*QueryDiff)) string {
	qm.mtx.Lock()
	defer qm.mtx.Unlock()
	q, ok := q.queries[descriptor]
	if !ok {
		q = NewQuery(fn)
		q.queries[descriptor] = q
	}
	return q.AddClient(cb)
}

func (q *QueryManager) Remove(descriptor, id string) {
	q.mtx.Lock()
	defer q.mtx.Unlock()
	c, ok := q.commits[descriptor]
	if ok && c.RemoveClient(id) {
		delete(q.commits, hash)
	}
}

func (qm *QueryManager) HandleRepoUpdate() {
	qm.mtx.Lock()
	defer qm.mtx.Unlock()
	var wg sync.WaitGroup
	for _, q := range qm.queries {
		wg.Add(1)
		go func(q *Query) {
			defer wg.Done()
			q.HandleRepoUpdate()
		}(q)
	}
	wg.Wait()
}

func main() {
	common.Init()

	ts, err := auth.NewDefaultTokenSource(true, datastore.ScopeDatastore, pubsub.ScopePubsub, bigtable.Scope)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	client, err := fs.NewClient(ctx, "skia-firestore", "task-scheduler", "production", ts)
	if err != nil {
		panic(err)
	}

	autoUpdateRepos, err := gs_pubsub.NewAutoUpdateMap(ctx, *repoUrls, btConf)
	if err != nil {
		sklog.Fatal(err)
	}
	repos := autoUpdateRepos.Map

	qm := NewQueryManager(repos)

	if err := autoUpdateRepos.Start(ctx, GITSTORE_SUBSCRIBER_ID, ts, 5*time.Minute, qm.HandleRepoUpdate); err != nil {
		sklog.Fatal(err)
	}

	cancel := watchRepoTasks(ctx, client, common.REPO_SKIA, 1)
	defer cancel()
	select {}
}
