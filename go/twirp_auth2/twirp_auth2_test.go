package twirp_auth2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/roles"
)

var (
	unauthorizedStatus = alogin.Status{
		EMail: alogin.NotLoggedIn,
		Roles: roles.Roles{},
	}

	viewerStatus = alogin.Status{
		EMail: alogin.EMail("viewer@example.com"),
		Roles: roles.Roles{roles.Viewer},
	}
	editorStatus = alogin.Status{
		EMail: alogin.EMail("editor@example.com"),
		Roles: roles.Roles{roles.Editor},
	}
)

func TestAuthHelperGetEditor_UserIsViewer_ReturnsError(t *testing.T) {
	ctx := alogin.FakeStatus(context.Background(), &viewerStatus)

	email, err := New().GetEditor(ctx)
	require.Error(t, err)
	require.Equal(t, alogin.NotLoggedIn.String(), email)
}

func TestAuthHelperGetEditor_UserIsEditor_Success(t *testing.T) {
	ctx := alogin.FakeStatus(context.Background(), &editorStatus)

	email, err := New().GetEditor(ctx)
	require.NoError(t, err)
	require.Equal(t, editorStatus.EMail.String(), email)
}

func TestAuthHelperGetViewer_UserIsViewer_ReturnsSuccess(t *testing.T) {
	ctx := alogin.FakeStatus(context.Background(), &viewerStatus)

	email, err := New().GetViewer(ctx)
	require.NoError(t, err)
	require.Equal(t, viewerStatus.EMail.String(), email)
}

func TestAuthHelperGetViewer_UserIsEditor_Success(t *testing.T) {
	ctx := alogin.FakeStatus(context.Background(), &editorStatus)

	email, err := New().GetViewer(ctx)
	require.NoError(t, err)
	require.Equal(t, editorStatus.EMail.String(), email)
}

func TestAuthHelperGetViewer_UserIsNotLoggedIn_ReturnsError(t *testing.T) {
	ctx := alogin.FakeStatus(context.Background(), &unauthorizedStatus)

	email, err := New().GetViewer(ctx)
	require.Error(t, err)
	require.Equal(t, alogin.NotLoggedIn.String(), email)
}
