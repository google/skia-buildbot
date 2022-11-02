package ingester

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/gitiles"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/louhi"
	louhi_mocks "go.skia.org/infra/go/louhi/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	artifactLink   = "http://artifacts/123"
	branch         = "main"
	commit         = "abc123"
	flowID         = "123-456-abc"
	flowLink       = "http://flows/" + id
	flowName       = "My Flow"
	id             = "flow123"
	issueNum       = int64(598597)
	issueUrl       = "https://skia-review.googlesource.com/c/buildbot/+/598597"
	projectID      = "my-project"
	startedByLouhi = "Louhi"
	triggerType    = louhi.TriggerTypeCommit
)

// Timestamps are arbitrary, but finishedTs is after createdTs.
var createdTs = time.Unix(1667309176, 0)
var finishedTs = time.Unix(1667309500, 0)

func helper(t *testing.T, n *louhi.Notification, oldFlow, expect *louhi.FlowExecution, ts time.Time) {
	ctx := context.Background()
	db := &louhi_mocks.DB{}
	db.On("GetFlowExecution", testutils.AnyContext, n.PipelineExecutionId).Return(oldFlow, nil)
	db.On("PutFlowExecution", testutils.AnyContext, expect).Return(nil)
	mockGerrit := &gerrit_mocks.GerritInterface{}
	mockRepo := &gitiles_mocks.GitilesRepo{}
	commitDetails := &vcsinfo.LongCommit{
		Body: "Reviewed-on: " + issueUrl,
	}
	mockRepo.On("Details", testutils.AnyContext, commit).Return(commitDetails, nil)
	mockGerrit.On("ExtractIssueFromCommit", commitDetails.Body).Return(issueNum, nil)
	mockGerrit.On("Url", issueNum).Return(issueUrl)
	repos := []gitiles.GitilesRepo{mockRepo}
	ingester := NewIngester(db, mockGerrit, repos)
	require.NoError(t, ingester.UpdateFlowFromNotification(ctx, n, ts))
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
		SourceCL:    issueUrl,
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
		SourceCL:    issueUrl,
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
		SourceCL:    issueUrl,
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
		SourceCL:    issueUrl,
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
		SourceCL:    issueUrl,
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
		SourceCL:    issueUrl,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, createdTs)
}

func TestUpdateFlowFromNotification_GeneratedCLs(t *testing.T) {
	// This tests the case where we receive the "started" notification after we
	// receive the "finished" notification. Pub/sub message ordering is not
	// guaranteed.
	n := &louhi.Notification{
		ProjectId:           projectID,
		PipelineExecutionId: id,
		EventSource:         louhi.EventSource_PIPELINE,
		EventAction:         louhi.EventAction_CREATED_ARTIFACT,
		GeneratedCls:        []string{issueUrl},
	}
	oldFlow := &louhi.FlowExecution{
		Artifacts:    []string{artifactLink}, // Shouldn't be overwritten.
		CreatedAt:    createdTs,              // Should get updated.
		FinishedAt:   time.Time{},
		FlowID:       flowID,
		FlowName:     flowName,
		GeneratedCLs: nil, // Should get filled in.
		GitBranch:    branch,
		GitCommit:    commit,
		ID:           id,
		Link:         flowLink,
		ModifiedAt:   createdTs, // Should get updated.
		ProjectID:    projectID,
		Result:       louhi.FlowResultUnknown, // Shouldn't change.
		StartedBy:    startedByLouhi,
		TriggerType:  triggerType,
	}
	expect := &louhi.FlowExecution{
		Artifacts:    []string{artifactLink},
		CreatedAt:    createdTs,
		FinishedAt:   time.Time{},
		FlowID:       flowID,
		FlowName:     flowName,
		GeneratedCLs: []string{issueUrl},
		GitBranch:    branch,
		GitCommit:    commit,
		ID:           id,
		Link:         flowLink,
		ModifiedAt:   finishedTs,
		ProjectID:    projectID,
		Result:       louhi.FlowResultUnknown,
		// Not provided by the new notification but obtainable via GitCommit.
		SourceCL:    issueUrl,
		StartedBy:   startedByLouhi,
		TriggerType: triggerType,
	}
	helper(t, n, oldFlow, expect, finishedTs)
}
