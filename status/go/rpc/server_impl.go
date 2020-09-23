package rpc

import (
	context "context"
	fmt "fmt"
	"net/http"
	strconv "strconv"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/status/go/incremental"
	"go.skia.org/infra/task_scheduler/go/types"
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
	// TODO(westont): Fix timestamps, support updates on the client side, and stop always loading
	// everything.
	if (true || expectPodId != "" && expectPodId != s.podID) || fromTime.IsZero() {
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

func (s *statusServerImpl) AddComment(ctx context.Context,
	req *AddCommentRequest) (*AddCommentResponse, error) { 

		return &AddCommentResponse{}, nil
}

func (s *statusServerImpl) DeleteComment(ctx context.Context,
	req *DeleteCommentRequest) (*DeleteCommentResponse, error) {
	defer metrics2.FuncTimer().Stop()
	_, repoUrl, err := getRepo(r)
	commit := mux.Vars(r)["commit"]
	taskSpec, ok := mux.Vars(r)["taskSpec"]
	id, ok := mux.Vars(r)["id"] // taskid

	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	//task
	if !ok {
		httputils.ReportError(w, fmt.Errorf("No task ID given!"), "No task ID given!", http.StatusInternalServerError)
		return
	}
	task, err := taskDb.GetTaskById(id)
	if err != nil {
		httputils.ReportError(w, err, "Failed to obtain task details.", http.StatusInternalServerError)
		return
	}
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid comment id: %v", err), http.StatusInternalServerError)
		return
	}
	c := &types.TaskComment{
		Repo:      task.Repo,
		Revision:  task.Revision,
		Name:      task.Name,
		Timestamp: time.Unix(0, timestamp),
		TaskId:    task.Id,
	}

	if err := taskDb.DeleteTaskComment(c); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete comment: %v", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}


	//taskpec
	if !ok {
		httputils.ReportError(w, nil, "No taskSpec provided!", http.StatusInternalServerError)
		return
	}
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid timestamp: %v", err), http.StatusInternalServerError)
		return
	}
	c := types.TaskSpecComment{
		Repo:      repoUrl,
		Name:      taskSpec,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteTaskSpecComment(&c); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete comment: %v", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}

    //commit
	_, repoUrl, err := getRepo(r)
	if err != nil {
		httputils.ReportError(w, err, err.Error(), http.StatusInternalServerError)
		return
	}
	timestamp, err := strconv.ParseInt(mux.Vars(r)["timestamp"], 10, 64)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Invalid comment id: %v", err), http.StatusInternalServerError)
		return
	}
	c := types.CommitComment{
		Repo:      repoUrl,
		Revision:  commit,
		Timestamp: time.Unix(0, timestamp),
	}
	if err := taskDb.DeleteCommitComment(&c); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to delete commit comment: %s", err), http.StatusInternalServerError)
		return
	}
	if err := iCache.Update(context.Background(), false); err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("Failed to update cache: %s", err), http.StatusInternalServerError)
		return
	}
		return &DeleteCommentResponse{}, nil
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
