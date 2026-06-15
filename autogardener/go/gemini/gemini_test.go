package gemini

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/mcp/services/skia/task_details"
	"go.skia.org/infra/task_driver/go/display"
	"go.skia.org/infra/task_driver/go/td"
)

func TestPruneRecipeStep(t *testing.T) {
	tree := &task_details.RecipeStep{
		Name:   "root",
		Status: "SUCCESS",
		Substeps: []*task_details.RecipeStep{
			{
				Name:   "setup",
				Status: "SUCCESS",
			},
			{
				Name:         "build",
				Status:       "FAILURE",
				StdoutStream: "build/stdout",
				Substeps: []*task_details.RecipeStep{
					{
						Name:   "compile",
						Status: "SUCCESS",
					},
					{
						Name:         "link",
						Status:       "FAILURE",
						StdoutStream: "link/stdout",
					},
				},
			},
			{
				Name:   "upload",
				Status: "SUCCESS",
			},
		},
	}

	pruned, flattened := pruneSuccessfulRecipeSteps(tree)
	require.Equal(t, &task_details.RecipeStep{
		Name:   "root",
		Status: "SUCCESS",
		Substeps: []*task_details.RecipeStep{
			{
				Name:         "build",
				Status:       "FAILURE",
				StdoutStream: "build/stdout",
				Substeps: []*task_details.RecipeStep{
					{
						Name:         "link",
						Status:       "FAILURE",
						StdoutStream: "link/stdout",
					},
				},
			},
		},
	}, pruned)
	require.Equal(t, []*task_details.RecipeStep{
		{
			Name:         "build",
			Status:       "FAILURE",
			StdoutStream: "build/stdout",
			Substeps: []*task_details.RecipeStep{
				{
					Name:   "compile",
					Status: "SUCCESS",
				},
				{
					Name:         "link",
					Status:       "FAILURE",
					StdoutStream: "link/stdout",
				},
			},
		},
		{
			Name:         "link",
			Status:       "FAILURE",
			StdoutStream: "link/stdout",
		},
	}, flattened)
}

func TestPruneRecipeStep_AllSuccess(t *testing.T) {
	tree := &task_details.RecipeStep{
		Name:   "root",
		Status: "SUCCESS",
		Substeps: []*task_details.RecipeStep{
			{
				Name:   "setup",
				Status: "SUCCESS",
			},
		},
	}
	pruned, flattened := pruneSuccessfulRecipeSteps(tree)
	require.Nil(t, pruned)
	require.Nil(t, flattened)
}

func TestPruneTaskDriverStep(t *testing.T) {
	tree := &display.StepDisplay{
		Result: td.StepResultSuccess,
		Steps: []*display.StepDisplay{
			{
				Result: td.StepResultSuccess,
			},
			{
				Result: td.StepResultFailure,
				Steps: []*display.StepDisplay{
					{
						Result: td.StepResultSuccess,
					},
					{
						Result: td.StepResultException,
					},
				},
			},
		},
	}

	pruned, flattened := pruneSuccessfulTaskDriverSteps(tree)
	require.Equal(t, &display.StepDisplay{
		Result: td.StepResultSuccess,
		Steps: []*display.StepDisplay{
			{
				Result: td.StepResultFailure,
				Steps: []*display.StepDisplay{
					{
						Result: td.StepResultException,
					},
				},
			},
		},
	}, pruned)
	require.Equal(t, []*display.StepDisplay{
		{
			Result: td.StepResultFailure,
			Steps: []*display.StepDisplay{
				{
					Result: td.StepResultSuccess,
				},
				{
					Result: td.StepResultException,
				},
			},
		},
		{
			Result: td.StepResultException,
		},
	}, flattened)
}
