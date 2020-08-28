package rpc

import (
	"time"

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

	update.Timestamp = u.Timestamp.Format(time.RFC3339)

	return &rv
}
