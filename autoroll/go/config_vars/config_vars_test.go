package config_vars

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestTemplate(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	raw := "refs/branch-heads/{{.Branches.Chromium.Beta.Number}}"
	tmpl := (*Template)(&raw)
	m := &mocks.Client{}
	globalBranchClient = m
	m.On("Get", ctx).Return(&chrome_branch.Branches{
		Beta: &chrome_branch.Branch{
			Milestone: 81,
			Number:    4044,
		},
		Stable: &chrome_branch.Branch{
			Milestone: 80,
			Number:    3987,
		},
	}, nil)
	require.NoError(t, Update(ctx))
	require.Equal(t, "refs/branch-heads/4044", tmpl.String())
}
