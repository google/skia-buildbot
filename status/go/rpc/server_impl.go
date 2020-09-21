package rpc

import (
	context "context"
	"net/http"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/status/go/incremental"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Generate Go structs and Typescript classes from protobuf definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. status.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/status.twirp.go ./status.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w status.pb.go
//go:generate goimports -w status.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc status.proto

type statusServerImpl struct {
	iCache               *incremental.IncrementalCache
	getRepo              func(*GetIncrementalCommitsRequest) (string, string, error)
	maxCommitsToLoad     int
	defaultCommitsToLoad int
	podID                string
}

// This is incrementalJsonHandler, adjusted for Twirp, using ConvertUpdate to use generated types.
func (s *statusServerImpl) GetIncrementalCommits(ctx context.Context,
	req *GetIncrementalCommitsRequest) (*GetIncrementalCommitsResponse, error) {
	defer metrics2.FuncTimer().Stop()
	_, repoURL, err := s.getRepo(req)
	if err != nil {
		return nil, err
	}
	fromTime := req.From.AsTime()
	toTime := req.To.AsTime()
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
	if (expectPodId != "" && expectPodId != s.podID) || fromTime.IsZero() {
		update, err = s.iCache.GetAll(repoURL, numCommits)
	} else {
		if !toTime.IsZero() {
			update, err = s.iCache.GetRange(repoURL, fromTime, toTime, numCommits)
		} else {
			update, err = s.iCache.Get(repoURL, fromTime, numCommits)
		}
	}
	if err != nil {
		return nil, err
	}
	return ConvertUpdate(update, s.podID), nil
}

// NewStatusServer creates and returns a Twirp HTTP Server.
func NewStatusServer(
	iCache *incremental.IncrementalCache,
	getRepo func(*GetIncrementalCommitsRequest) (string, string, error),
	maxCommitsToLoad int,
	defaultCommitsToLoad int,
	podID string) http.Handler {
	return NewStatusServiceServer(&statusServerImpl{
		iCache,
		getRepo,
		maxCommitsToLoad,
		defaultCommitsToLoad,
		podID}, nil)
}

/*
ConvertUpdate converts an incremental.Update and Pod Id to a struct generated from a .proto,
with matching clientside TS definition.
*/
func ConvertUpdate(u *incremental.Update, podID string) *GetIncrementalCommitsResponse {
	if u == nil {
		return nil
	}

	rv := GetIncrementalCommitsResponse{
		Metadata: &ResponseMetadata{},
		Update:   &IncrementalUpdate{},
	}
	rv.Metadata.Pod = podID
	rv.Metadata.StartOver = u.StartOver != nil && *u.StartOver
	rv.Metadata.Timestamp = timestamppb.New(u.Timestamp)

	update := rv.Update
	for _, b := range u.BranchHeads {
		update.BranchHeads = append(update.BranchHeads, &Branch{Name: b.Name, Head: b.Head})
	}
	for _, c := range u.Commits {
		update.Commits = append(update.Commits, &LongCommit{
			Hash:      c.Hash,
			Author:    c.Author,
			Subject:   c.Subject,
			Parents:   c.Parents,
			Body:      c.Body,
			Timestamp: timestamppb.New(c.Timestamp)})
	}
	update.SwarmingUrl = u.SwarmingUrl
	for _, t := range u.Tasks {
		update.Tasks = append(update.Tasks, &Task{
			Commits:        t.Commits,
			Name:           t.Name,
			Id:             t.Id,
			Revision:       t.Revision,
			Status:         string(t.Status),
			SwarmingTaskId: t.SwarmingTaskId})
	}
	update.TaskSchedulerUrl = u.TaskSchedulerUrl
	for hash, srcComments := range u.CommitComments {
		for _, c := range srcComments {
			update.Comments = append(update.Comments, &Comment{
				Commit:        hash,
				Id:            c.Id,
				Repo:          c.Repo,
				Timestamp:     timestamppb.New(c.Timestamp),
				User:          c.User,
				IgnoreFailure: c.IgnoreFailure,
				Message:       c.Message,
				Deleted:       c.Deleted != nil && *c.Deleted})
		}
	}
	for _, srcComments := range u.TaskSpecComments {
		for _, c := range srcComments {
			update.Comments = append(update.Comments, &Comment{
				Id:            c.Id,
				Repo:          c.Repo,
				TaskSpecName:  c.Name,
				Timestamp:     timestamppb.New(c.Timestamp),
				User:          c.User,
				Flaky:         c.Flaky,
				IgnoreFailure: c.IgnoreFailure,
				Message:       c.Message,
				Deleted:       c.Deleted != nil && *c.Deleted})
		}
	}
	for _, commitComments := range u.TaskComments {
		for hash, srcComments := range commitComments {
			for _, c := range srcComments {
				update.Comments = append(update.Comments, &Comment{
					Commit:       hash,
					Id:           c.Id,
					Repo:         c.Repo,
					TaskSpecName: c.Name,
					Timestamp:    timestamppb.New(c.Timestamp),
					TaskId:       c.TaskId,
					User:         c.User,
					Message:      c.Message,
					Deleted:      c.Deleted != nil && *c.Deleted})
			}
		}
	}

	return &rv
}
