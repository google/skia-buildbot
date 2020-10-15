package rpc

import (
	context "context"
	fmt "fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/status/go/incremental"
	"go.skia.org/infra/task_scheduler/go/db"
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
	iCache                *incremental.IncrementalCache
	taskDb                db.RemoteDB
	getAutorollerStatuses func() *GetAutorollerStatusesResponse
	getRepo               func(string) (string, string, error)
	maxCommitsToLoad      int
	defaultCommitsToLoad  int
	podID                 string
}

// This is incrementalJsonHandler, adjusted for Twirp, using ConvertUpdate to use generated types.
func (s *statusServerImpl) GetIncrementalCommits(ctx context.Context,
	req *GetIncrementalCommitsRequest) (*GetIncrementalCommitsResponse, error) {
	defer metrics2.FuncTimer().Stop()
	_, repoURL, err := s.getRepo(req.RepoPath)
	if err != nil {
		return nil, err
	}

	hasFromTime := req.From.IsValid()
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
	if (expectPodId != "" && expectPodId != s.podID) || !hasFromTime {
		update, err = s.iCache.GetAll(repoURL, numCommits)
	} else {
		fromTime := req.From.AsTime()
		if req.To.IsValid() {
			update, err = s.iCache.GetRange(repoURL, fromTime, req.To.AsTime(), numCommits)
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
	defer metrics2.FuncTimer().Stop()
	_, repoURL, err := s.getRepo(req.Repo)
	if err != nil {
		return nil, err
	}
	message := req.Message
	now := time.Now().UTC()
	switch req.Type.(type) {
	case *AddCommentRequest_TaskId:
		task, err := s.taskDb.GetTaskById(req.GetTaskId())
		if err != nil {
			return nil, fmt.Errorf("failed to obtain task details: %v", err)
		}
		c := types.TaskComment{
			Repo:      task.Repo,
			Revision:  task.Revision,
			Name:      task.Name,
			Timestamp: now,
			TaskId:    task.Id,
			User:      login.AuthorizedEmail(ctx),
			Message:   message,
		}
		if err := s.taskDb.PutTaskComment(&c); err != nil {
			return nil, fmt.Errorf("failed to add task comment: %v", err)
		}
	case *AddCommentRequest_TaskSpec:
		c := types.TaskSpecComment{
			Repo:          repoURL,
			Name:          req.GetTaskSpec(),
			Timestamp:     now,
			User:          login.AuthorizedEmail(ctx),
			Flaky:         req.Flaky,
			IgnoreFailure: req.IgnoreFailure,
			Message:       req.Message,
		}
		if err := s.taskDb.PutTaskSpecComment(&c); err != nil {
			return nil, fmt.Errorf("failed to add task spec  comment: %v", err)
		}
	case *AddCommentRequest_Commit:
		c := types.CommitComment{
			Repo:          repoURL,
			Revision:      req.GetCommit(),
			Timestamp:     now,
			User:          login.AuthorizedEmail(ctx),
			IgnoreFailure: req.IgnoreFailure,
			Message:       req.Message,
		}
		if err := s.taskDb.PutCommitComment(&c); err != nil {
			return nil, fmt.Errorf("failed to add commit comment: %v", err)
		}
	case nil:
		return nil, fmt.Errorf("no Task ID, Task Spec, or Commit given")
	default:
		return nil, fmt.Errorf("unsupported comment type given")
	}
	if err := s.iCache.Update(context.Background(), false); err != nil {
		return nil, fmt.Errorf("failed to update cache: %s", err)
	}
	return &AddCommentResponse{Timestamp: timestamppb.New(now)}, nil
}

func (s *statusServerImpl) DeleteComment(ctx context.Context,
	req *DeleteCommentRequest) (*DeleteCommentResponse, error) {
	defer metrics2.FuncTimer().Stop()
	_, repoURL, err := s.getRepo(req.Repo)
	if err != nil {
		return nil, err
	}
	timestamp := req.Timestamp.AsTime()
	if timestamp.IsZero() {
		return nil, fmt.Errorf("no timestamp (comment ID) given")
	}
	commit := req.Commit
	taskSpec := req.TaskSpec
	taskID := req.TaskId

	if taskID != "" {
		// This references a comment on an individual task.
		task, err := s.taskDb.GetTaskById(taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain task details: %v", err)
		}
		c := &types.TaskComment{
			Repo:      task.Repo,
			Revision:  task.Revision,
			Name:      task.Name,
			Timestamp: timestamp,
			TaskId:    task.Id,
		}

		if err := s.taskDb.DeleteTaskComment(c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}
	} else if taskSpec != "" {
		// This references a comment on a Task Spec.
		c := types.TaskSpecComment{
			Repo:      repoURL,
			Name:      taskSpec,
			Timestamp: timestamp,
		}
		if err := s.taskDb.DeleteTaskSpecComment(&c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}
	} else if commit != "" {
		// This references a comment on a commit.
		c := types.CommitComment{
			Repo:      repoURL,
			Revision:  commit,
			Timestamp: timestamp,
		}
		if err := s.taskDb.DeleteCommitComment(&c); err != nil {
			return nil, fmt.Errorf("failed to delete comment: %v", err)
		}

	} else {
		return nil, fmt.Errorf("no Task ID, Task Spec, or Commit given")
	}

	if err := s.iCache.Update(context.Background(), false); err != nil {
		return nil, fmt.Errorf("failed to update cache: %s", err)
	}
	return &DeleteCommentResponse{}, nil
}

func (s *statusServerImpl) GetAutorollerStatuses(ctx context.Context, req *GetAutorollerStatusesRequest) (*GetAutorollerStatusesResponse, error) {
	return s.getAutorollerStatuses(), nil
}

// NewStatusServer creates and returns a Twirp HTTP Server.
func NewStatusServer(
	iCache *incremental.IncrementalCache,
	taskDb db.RemoteDB,
	getAutorollStatuses func() *GetAutorollerStatusesResponse,
	getRepo func(string) (string, string, error),
	maxCommitsToLoad int,
	defaultCommitsToLoad int,
	podID string) http.Handler {
	return NewStatusServiceServer(&statusServerImpl{
		iCache,
		taskDb,
		getAutorollStatuses,
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
	for _, t := range u.Tasks {
		update.Tasks = append(update.Tasks, &Task{
			Commits:        t.Commits,
			Name:           t.Name,
			Id:             t.Id,
			Revision:       t.Revision,
			Status:         string(t.Status),
			SwarmingTaskId: t.SwarmingTaskId})
	}
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
