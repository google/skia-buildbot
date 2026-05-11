package types

import (
	"fmt"

	"go.skia.org/infra/task_scheduler/go/types"
)

type TaskSummary struct {
	Analysis     string `json:"analysis"`
	ErrorMessage string `json:"errorMessage"`
}

func (s TaskSummary) String() string {
	return fmt.Sprintf("**Error Message:**\n```\n%s\n```\n\n**Analysis:** %s\n", s.ErrorMessage, s.Analysis)
}

type TaskAndSummary struct {
	Task    *types.Task
	Summary *TaskSummary
}
