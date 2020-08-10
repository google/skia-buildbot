package child

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch/mocks"
)

// TODO(borenet): Split up the tests in no_checkout_deps_repo_manager_test.go
// and move the relevant parts here.

// TODO(borenet): This was copied from no_checkout_deps_repo_manager_test.go.
func masterBranchTmpl(t *testing.T) *config_vars.Template {
	master, err := config_vars.NewTemplate("master")
	require.NoError(t, err)
	return master
}

// TODO(borenet): This was copied from repo_manager_test.go.
func setupRegistry(t *testing.T) *config_vars.Registry {
	cbc := &mocks.Client{}
	cbc.On("Get", mock.Anything).Return(config_vars.DummyVars().Branches.Chromium, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return reg
}
