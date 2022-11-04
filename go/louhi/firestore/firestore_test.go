package firestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/louhi"
)

func setup(t *testing.T) (context.Context, louhi.DB, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	d := newDBWithClient(ctx, c)
	return ctx, d, func() {
		cancel()
		cleanup()
	}
}

func TestFirestoreDB_PutGet(t *testing.T) {
	ctx, db, cleanup := setup(t)
	defer cleanup()
	startTs := time.Unix(1667570100, 0).UTC()
	finishTs := time.Unix(1667570500, 0).UTC()
	fe := &louhi.FlowExecution{
		Artifacts:    []string{"artifact-1"},
		CreatedAt:    startTs,
		FinishedAt:   finishTs,
		FlowName:     "my flow",
		FlowID:       "my-flow-123",
		GeneratedCLs: []string{"skia-review/12345"},
		GitBranch:    "main",
		GitCommit:    "abc123",
		ID:           "456",
		Link:         "https://flows/456",
		ModifiedAt:   finishTs,
		ProjectID:    "my-project",
		Result:       louhi.FlowResultSuccess,
		SourceCL:     "skia-review/54321",
		StartedBy:    "Louhi",
		TriggerType:  louhi.TriggerTypeCommit,
	}
	require.NoError(t, db.PutFlowExecution(ctx, fe))
	actual, err := db.GetFlowExecution(ctx, fe.ID)
	require.NoError(t, err)
	require.Equal(t, fe, actual)
}

func TestFirestoreDB_GetLatestFlowExecutions(t *testing.T) {
	ctx, db, cleanup := setup(t)
	defer cleanup()
	startTs := time.Unix(1667570100, 0).UTC()
	finishTs := time.Unix(1667570500, 0).UTC()
	// This is an execution of a previous revision of the flow. Note that the
	// flow name is the same as the other two executions, but the flow ID is
	// different. Because there is a newer flow execution with the same name,
	// this will be retrieved from the DB but not returned by
	// GetLatestFlowExecutions, as it will get deduplicated.
	fe0 := &louhi.FlowExecution{
		CreatedAt:   startTs.Add(-time.Hour),
		FinishedAt:  finishTs.Add(-time.Hour),
		FlowName:    "my flow",
		FlowID:      "my-flow-original",
		GitBranch:   "main",
		GitCommit:   "123abc",
		ID:          "123",
		Link:        "https://flows/123",
		ModifiedAt:  finishTs.Add(-time.Hour),
		ProjectID:   "my-project",
		Result:      louhi.FlowResultFailure,
		SourceCL:    "skia-review/54321",
		StartedBy:   "Louhi",
		TriggerType: louhi.TriggerTypeCommit,
	}
	// This is the last finished execution of the most recent revision of the
	// flow. We expect this execution to be the only one returned by
	// GetLatestFlowExecutions.
	fe1 := &louhi.FlowExecution{
		Artifacts:    []string{"artifact-1"},
		CreatedAt:    startTs,
		FinishedAt:   finishTs,
		FlowName:     "my flow",
		FlowID:       "my-flow-123",
		GeneratedCLs: []string{"skia-review/12345"},
		GitBranch:    "main",
		GitCommit:    "abc123",
		ID:           "456",
		Link:         "https://flows/456",
		ModifiedAt:   finishTs,
		ProjectID:    "my-project",
		Result:       louhi.FlowResultSuccess,
		SourceCL:     "skia-review/54321",
		StartedBy:    "Louhi",
		TriggerType:  louhi.TriggerTypeCommit,
	}
	// This is an unfinished execution of the most recent revision of the flow.
	// it should be passed over in favor of fe1.
	fe2 := &louhi.FlowExecution{
		CreatedAt:   startTs.Add(time.Hour),
		FinishedAt:  finishTs.Add(time.Hour),
		FlowName:    "my flow",
		FlowID:      "my-flow-123",
		GitBranch:   "main",
		GitCommit:   "def456",
		ID:          "919",
		Link:        "https://flows/919",
		ModifiedAt:  finishTs.Add(time.Hour),
		ProjectID:   "my-project",
		Result:      louhi.FlowResultUnknown,
		SourceCL:    "skia-review/54321",
		StartedBy:   "Louhi",
		TriggerType: louhi.TriggerTypeCommit,
	}
	require.NoError(t, db.PutFlowExecution(ctx, fe0))
	require.NoError(t, db.PutFlowExecution(ctx, fe1))
	require.NoError(t, db.PutFlowExecution(ctx, fe2))
	actual, err := db.GetLatestFlowExecutions(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]*louhi.FlowExecution{
		fe1.FlowName: fe1,
	}, actual)
}
