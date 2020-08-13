package db

import (
	"context"
	"time"

	"go.skia.org/infra/task_scheduler/go/candidate"
)

type TaskCandidateDB interface {
	GetActive(context.Context) ([]*candidate.TaskCandidate, error)
	GetCandidatesForJobs(context.Context, []string) (map[string][]*candidate.TaskCandidate, error)
	GetRange(context.Context, time.Time, time.Time) ([]*candidate.TaskCandidate, error)
	Put(context.Context, []*candidate.TaskCandidate) error
}
