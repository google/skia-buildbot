package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

const (
	fakeIssueID      = int64(333705433)
	fakeIssueComment = "comment"
)

func TestPostBugCommentWorkflow_GivenValidInputs_ReturnsTrue(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(PostBugCommentActivity, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Once()
	env.ExecuteWorkflow(PostBugCommentWorkflow, fakeIssueID, fakeIssueComment)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result bool
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.True(t, result)
	env.AssertExpectations(t)
}

func TestPostBugCommentWorkflow_GivenActivityError_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(PostBugCommentActivity, mock.Anything, mock.Anything, mock.Anything).Return(false, fmt.Errorf("error")).Times(int(regularActivityOptions.RetryPolicy.MaximumAttempts))
	env.ExecuteWorkflow(PostBugCommentWorkflow, fakeIssueID, fakeIssueComment)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	var result bool
	require.Error(t, env.GetWorkflowResult(&result))
	assert.False(t, result)
	env.AssertExpectations(t)
}

func TestPostBugCommentWorkflow_GivenActivityUnsuccessful_ReturnsFalse(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.OnActivity(PostBugCommentActivity, mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()
	env.ExecuteWorkflow(PostBugCommentWorkflow, fakeIssueID, fakeIssueComment)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result bool
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.False(t, result)
	env.AssertExpectations(t)
}
