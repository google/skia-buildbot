package checker

import (
	"context"
	"testing"

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

	checker := NewChecker(store, provider)
	err := checker.Check(ctx)
	require.NoError(t, err)

	store.AssertExpectations(t)
	provider.AssertExpectations(t)
}
