package rpc

import(
	"go.skia.org/infra/status/go/incremental"
	"time"
)


// Generate Go structs and Typescript classes from protobuf defintions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. statusFe.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/statusFe.twirp.go ./statusFe.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w statusFe.pb.go
//go:generate goimports -w statusFe.twirp.go
//go:Agenerate protoc --proto_path=.:../../.. --twirp_ts_out=../../modules/rpc statusFe.proto
//go:Agenerate ../../node_modules/typescript-formatter/bin/tsfmt -r ../../modules/rpc/status.ts ../../modules/rpc/twirp.ts
//go:Agenerate sed -i 's/\.\/rpc/go.skia.org\/infra\/status\/go\/rpc/g' statusFe.pb.go

// TODO(westont): Implement a server.

/*
ConvertUpdate converts an incremental.Update to a typesafe .proto generated type.
*/
func ConvertUpdate(u *incremental.Update, podID string) *IncrementalCommitsResponse {
	if u == nil {
	  return nil
	}

	rv := IncrementalCommitsResponse{
		Metadata: &ResponseMetadata{},
		Update: &IncrementalUpdate{},
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
		tasks = append(tasks, &Task{ Commits: t.Commits, Name: t.Name, Id: t.Id, Revision: t.Revision, Status: string(t.Status), SwarmingTaskId: t.SwarmingTaskId})
	}
	update.TaskSchedulerUrl = u.TaskSchedulerUrl
	comments := update.Comments
	// this will be a thing.
	for hash, srcComments := range u.CommitComments {
		// TaskSpec is empty, comment applies to the whole TaskSpec.
		commitComments := comments[""].Comments[hash].Comments;
		for _, c := range srcComments {
			commitComments = append(commitComments, &Comment{Id: c.Id, Repo: c.Repo, Revision: c.Revision, Timestamp: c.Timestamp.Format(time.RFC3339), User: c.User, IgnoreFailure: c.IgnoreFailure, Message: c.Message, Deleted: *c.Deleted})
		}
	}
	for taskSpec, srcComments := range u.TaskSpecComments {
		// Commit hash is empty, comment applies to the whole TaskSpec.
		taskSpecComments := comments[taskSpec].Comments[""].Comments;
		for _, c := range srcComments {
			taskSpecComments = append(taskSpecComments, &Comment{Id: c.Id, Repo: c.Repo, TaskSpecName: c.Name, Timestamp: c.Timestamp.Format(time.RFC3339), User: c.User, Flaky: c.Flaky, IgnoreFailure: c.IgnoreFailure, Message: c.Message, Deleted: *c.Deleted})
		}
	}
	for taskSpec, commitComments := range u.TaskComments {
		for hash, srcComments := range commitComments {
			taskComments := comments[taskSpec].Comments[hash].Comments;
			for _, c := range srcComments {
				taskComments = append(taskComments, &Comment{Id: c.Id, Repo: c.Repo, Revision: c.Revision, TaskSpecName: c.Name, Timestamp: c.Timestamp.Format(time.RFC3339), TaskId: c.TaskId, User: c.User, Message: c.Message, Deleted: *c.Deleted})
			}
		}
	}

	update.Timestamp = u.Timestamp.Format(time.RFC3339)
	
	return &rv
}
 