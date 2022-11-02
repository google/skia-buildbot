package mocks

// Note: We'd prefer this test to be in the go/louhi package, but we can't do so
// because it would create an import cycle between go/louhi and go/louhi/mocks.

import (
	context "context"
	testing "testing"
	"time"

	"github.com/stretchr/testify/require"
	louhi "go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/testutils"
)

const (
	artifactLink   = "http://artifacts/123"
	branch         = "main"
	commit         = "abc123"
	flowID         = "123-456-abc"
	flowLink       = "http://flows/" + id
	flowName       = "My Flow"
	id             = "flow123"
	projectID      = "my-project"
	startedByLouhi = "Louhi"
	triggerType    = louhi.TriggerTypeCommit
)

// Timestamps are arbitrary, but finishedTs is after createdTs.
var createdTs = time.Unix(1667309176, 0)
var finishedTs = time.Unix(1667309500, 0)

func helper(t *testing.T, n *louhi.Notification, oldFlow, expect *louhi.FlowExecution, ts time.Time) {
	ctx := context.Background()
	db := &DB{}
	db.On("GetFlowExecution", testutils.AnyContext, n.PipelineExecutionId).Return(oldFlow, nil)
	db.On("PutFlowExecution", testutils.AnyContext, expect).Return(nil)
	require.NoError(t, louhi.UpdateFlowFromNotification(ctx, db, n, ts))
}

func TestUpdateFlowFromNotification_NewFlow(t *testing.T) {
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_STARTED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        []string{artifactLink},
	}
	var oldFlow *louhi.FlowExecution = nil
	expect := &louhi.FlowExecution{
		Artifacts:   []string{artifactLink},
		CreatedAt:   createdTs,
		FinishedAt:  time.Time{},
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  createdTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultUnknown,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, createdTs)
}

func TestUpdateFlowFromNotification_Success(t *testing.T) {
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_FINISHED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        []string{artifactLink},
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:   nil, // Should get filled in.
		CreatedAt:   createdTs,
		FinishedAt:  time.Time{}, // Should get filled in.
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  createdTs, // Should get updated.
		ProjectID:   projectID,
		Result:      louhi.FlowResultUnknown, // Should get updated.
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:   []string{artifactLink},
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultSuccess,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, finishedTs)
}

func TestUpdateFlowFromNotification_Failure(t *testing.T) {
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_FAILED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        nil,
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:   nil, // Should stay nil.
		CreatedAt:   createdTs,
		FinishedAt:  time.Time{}, // Should get filled in.
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  createdTs, // Should get updated.
		ProjectID:   projectID,
		Result:      louhi.FlowResultUnknown, // Should get updated.
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:   nil,
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultFailure,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, finishedTs)
}

func TestUpdateFlowFromNotification_FailureAfterFinished(t *testing.T) {
	// This tests the case where we first receive a "finished" notification,
	// which we interpret as success, but then receive a "failed" notification.
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_FAILED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        nil,
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:   nil, // Should stay nil.
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs, // Should stay the same.
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs, // Should stay the same.
		ProjectID:   projectID,
		Result:      louhi.FlowResultSuccess, // Should get updated.
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:   nil,
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultFailure,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, finishedTs)
}

func TestUpdateFlowFromNotification_FinishedAfterFailure(t *testing.T) {
	// This tests the case where we first receive a "finished" notification,
	// which we interpret as success, but then receive a "failed" notification.
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_FINISHED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        nil,
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:   nil, // Should stay nil.
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs, // Should stay the same.
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs, // Should stay the same.
		ProjectID:   projectID,
		Result:      louhi.FlowResultFailure, // Should stay the same.
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:   nil,
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultFailure,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, finishedTs)
}

func TestUpdateFlowFromNotification_StartedAfterFinished(t *testing.T) {
	// This tests the case where we receive the "started" notification after we
	// receive the "finished" notification. Pub/sub message ordering is not
	// guaranteed.
	n := &louhi.Notification{
		ProjectId:           projectID,
		FlowUniqueKey:       flowID,
		FlowName:            flowName,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_STARTED,
		Link:                flowLink,
		Branch:              branch,
		RefSha:              commit,
		TriggerType:         triggerType,
		StartedBy:           startedByLouhi,
		ArtifactLink:        nil,
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:   []string{artifactLink}, // Shouldn't be overwritten.
		CreatedAt:   finishedTs,             // Should get updated.
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs, // Shouldn't change.
		ProjectID:   projectID,
		Result:      louhi.FlowResultSuccess, // Shouldn't change.
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:   []string{artifactLink},
		CreatedAt:   createdTs,
		FinishedAt:  finishedTs,
		FlowID:      flowID,
		FlowName:    flowName,
		GitBranch:   branch,
		GitCommit:   commit,
		ID:          id,
		Link:        flowLink,
		ModifiedAt:  finishedTs,
		ProjectID:   projectID,
		Result:      louhi.FlowResultSuccess,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, createdTs)
}
