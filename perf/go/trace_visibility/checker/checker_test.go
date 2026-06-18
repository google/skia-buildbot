package checker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/trace_visibility/provider/mocks"
	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore/schema"
	store_mocks "go.skia.org/infra/perf/go/trace_visibility/store/mocks"
)

func TestCheck_Success(t *testing.T) {
	ctx := context.Background()

	provider := &mocks.Provider{}

	dbConfigs := []schema.PublicTraceRulesSchema{
		{RuleExpression: "bot=builder1"},
		{RuleExpression: "bot=extra-builder"},
	}

	provider.On("GetExpectedRules", mock.Anything).Return(map[string]bool{
		"bot=builder1": true,
		"bot=builder2": true,
	}, nil)

	store := &store_mocks.Store{}
	store.On("GetAll", mock.Anything).Return(dbConfigs, nil)
	store.On("Set", mock.Anything, "bot=builder2").Return(nil).Once()

	checker := NewChecker(store, provider)
	addedCount, extraCount, err := checker.Check(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, addedCount, "Expected 1 rule to be added")
	assert.Equal(t, 1, extraCount, "Expected 1 extra rule to be found")

	store.AssertExpectations(t)
	provider.AssertExpectations(t)
}

func TestCheck_RemediationFailure(t *testing.T) {
	ctx := context.Background()
	provider := &mocks.Provider{}
	provider.On("GetExpectedRules", mock.Anything).Return(map[string]bool{
		"bot=builder2": true,
	}, nil)

	store := &store_mocks.Store{}
	store.On("GetAll", mock.Anything).Return([]schema.PublicTraceRulesSchema{}, nil)
	// Set returns an error
	store.On("Set", mock.Anything, "bot=builder2").Return(fmt.Errorf("database down")).Once()

	checker := NewChecker(store, provider)
	addedCount, extraCount, err := checker.Check(ctx)
	require.NoError(t, err) // The check itself finishes successfully, logging remediation error
	assert.Equal(t, 0, addedCount, "Expected 0 rules to be added since DB save failed")
	assert.Equal(t, 0, extraCount, "Expected 0 extra rules to be found")

	store.AssertExpectations(t)
	provider.AssertExpectations(t)
}
