package internal

import (
	"errors"

	"go.skia.org/infra/bisection/go/workflows"
	"go.temporal.io/sdk/workflow"
)

// BuildChrome is a Workflow definition that builds Chrome.
func BuildChrome(ctx workflow.Context, params workflows.BuildChromeParams) (error, string) {
	return errors.New("BuildChrome is not implemented."), ""
}
