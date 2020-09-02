package rpc

import (
	context "context"
	json "encoding/json"
	"net/http"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/status/go/incremental"
)

// Generate Go structs and Typescript classes from protobuf definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. statusFe.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/statusFe.twirp.go ./statusFe.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w statusFe.pb.go
//go:generate goimports -w statusFe.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc statusFe.proto

// TODO(westont): Implement a server.

type statusServerImpl struct {
	iCache               *incremental.IncrementalCache
	getRepo              func(*IncrementalCommitsRequest) (string, string, error)
	maxCommitsToLoad     int
	defaultCommitsToLoad int
	podID                string
}

func (s *statusServerImpl) GetIncrementalCommits(ctx context.Context, req *IncrementalCommitsRequest) (*IncrementalCommitsResponse, error) {
	defer metrics2.FuncTimer().Stop()
	_, repoURL, err := s.getRepo(req)
	if err != nil {
		sklog.Error(err)
		return nil, err
	}
	from := req.From
	to := req.To
	n := req.N
	expectPodId := req.Pod
	numCommits := s.defaultCommitsToLoad
	if n != 0 {
		numCommits = int(n)
		if numCommits > s.maxCommitsToLoad {
			numCommits = s.maxCommitsToLoad
		}
	}
	var update *incremental.Update
	if (expectPodId != "" && expectPodId != s.podID) || from == 0 {
		sklog.Infof("Getting data from icache: %s commits", numCommits)

		update, err = s.iCache.GetAll(repoURL, numCommits)
		//up, _ := json.Marshal(update)
		//sklog.Info("update pre convert: " + string(resp))
	} else {
		fromTime := time.Unix(0, from*int64(time.Millisecond))
		if to != 0 {
			toTime := time.Unix(0, to*int64(time.Millisecond))
			update, err = s.iCache.GetRange(repoURL, fromTime, toTime, numCommits)
		} else {
			update, err = s.iCache.Get(repoURL, fromTime, numCommits)
		}
	}
	if err != nil {
		sklog.Error(err)
		return nil, err
	}
	ret := ConvertUpdate(update, s.podID)
	resp, _ := json.Marshal(ret)
	sklog.Info(string(resp))
	return ret, nil
}

// NewStatusServer creates and returns a Twirp HTTP Server.
func NewStatusServer(
	iCache *incremental.IncrementalCache,
	getRepo func(*IncrementalCommitsRequest) (string, string, error),
	maxCommitsToLoad int,
	defaultCommitsToLoad int,
	podID string) http.Handler {
	return NewStatusFeServer(&statusServerImpl{
		iCache,
		getRepo,
		maxCommitsToLoad,
		defaultCommitsToLoad,
		podID}, nil)
}

/*
ConvertUpdate converts an incremental.Update and Pod Id to a struct generated from a .proto, with matching clientside TS definition.
*/
func ConvertUpdate(u *incremental.Update, podID string) *IncrementalCommitsResponse {
	if u == nil {
		return nil
	}

	rv := IncrementalCommitsResponse{
		Metadata: &ResponseMetadata{},
		Update:   &IncrementalUpdate{},
	}
	rv.Metadata.Pod = podID
	update := rv.Update
	branchHeads := update.BranchHeads
	for _, b := range u.BranchHeads {
		branchHeads = append(branchHeads, &Branch{Name: b.Name, Head: b.Head})
	}
	commits := update.Commits
	for _, c := range u.Commits {
		commits = append(commits, &LongCommit{Hash: c.Hash, Author: c.Author, Subject: c.Subject, Parents: c.Parents, Body: c.Body, Timestamp: c.Timestamp.Format(time.RFC3339)})
	}
	update.StartOver = *u.StartOver
	update.SwarmingUrl = u.SwarmingUrl
	tasks := update.Tasks
	for _, t := range u.Tasks {
		tasks = append(tasks, &Task{Commits: t.Commits, Name: t.Name, Id: t.Id, Revision: t.Revision, Status: string(t.Status), SwarmingTaskId: t.SwarmingTaskId})
	}
	update.TaskSchedulerUrl = u.TaskSchedulerUrl
	comments := update.Comments
	for hash, srcComments := range u.CommitComments {
		for _, c := range srcComments {
			comments = append(comments, &Comment{CommitHash: hash, Id: c.Id, Repo: c.Repo, Revision: c.Revision, Timestamp: c.Timestamp.Format(time.RFC3339), User: c.User, IgnoreFailure: c.IgnoreFailure, Message: c.Message, Deleted: *c.Deleted})
		}
	}
	for taskSpec, srcComments := range u.TaskSpecComments {
		for _, c := range srcComments {
			if taskSpec != c.Name {
				panic("taskspec isn't what I thought:" + taskSpec + " " + c.Name)
			}
			comments = append(comments, &Comment{Id: c.Id, Repo: c.Repo, TaskSpecName: c.Name, Timestamp: c.Timestamp.Format(time.RFC3339), User: c.User, Flaky: c.Flaky, IgnoreFailure: c.IgnoreFailure, Message: c.Message, Deleted: *c.Deleted})
		}
	}
	for taskSpec, commitComments := range u.TaskComments {
		for hash, srcComments := range commitComments {
			for _, c := range srcComments {
				if taskSpec != c.Name {
					panic("taskspec isn't what I thought:" + taskSpec + " " + c.Name)
				}
				comments = append(comments, &Comment{CommitHash: hash, Id: c.Id, Repo: c.Repo, Revision: c.Revision, TaskSpecName: c.Name, Timestamp: c.Timestamp.Format(time.RFC3339), TaskId: c.TaskId, User: c.User, Message: c.Message, Deleted: *c.Deleted})
			}
		}
	}
	update.Commits = commits
	update.Tasks = tasks
	update.Comments = comments

	update.Timestamp = u.Timestamp.Format(time.RFC3339)

	return &rv
}
