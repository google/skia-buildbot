package revision_filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd/mocks"
)

func TestCIPDRevisionFilter(t *testing.T) {

	ctx := context.Background()
	rf := &CIPDRevisionFilter{
		client:    nil, // Filled in later.
		packages:  []string{"my/pkg/1", "my/pkg/2"},
		platforms: []string{"linux-amd64", "mac-amd64"},
	}

	oneResult := common.PinSlice([]common.Pin{{}})
	zeroResult := common.PinSlice([]common.Pin{})

	// We should reject revisions which don't follow the CIPD tag format.
	t.Run("reject_bad_rev_id", func(t *testing.T) {
		rev := &revision.Revision{
			Id: "bad",
		}
		skipReason, err := rf.Skip(ctx, rev)
		require.NoError(t, err)
		require.Equal(t, "Revision ID \"bad\" doesn't follow CIPD tag format", skipReason)
	})

	// All platforms and packages exist.
	t.Run("check_all_platforms", func(t *testing.T) {
		client := &mocks.CIPDClient{}
		rf.client = client
		rev := &revision.Revision{
			Id: "git_revision:abc123def456",
		}
		client.On("SearchInstances", ctx, "my/pkg/1/linux-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/1/mac-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/2/linux-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/2/mac-amd64", []string{rev.Id}).Return(oneResult, nil)

		skipReason, err := rf.Skip(ctx, rev)
		require.NoError(t, err)
		require.Equal(t, "", skipReason)

		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/1/linux-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/1/mac-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/2/linux-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/2/mac-amd64", []string{rev.Id})
	})

	// One platform is missing.
	t.Run("check_all_platforms", func(t *testing.T) {
		client := &mocks.CIPDClient{}
		rf.client = client
		rev := &revision.Revision{
			Id: "git_revision:abc123def456",
		}
		client.On("SearchInstances", ctx, "my/pkg/1/linux-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/1/mac-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/2/linux-amd64", []string{rev.Id}).Return(oneResult, nil)
		client.On("SearchInstances", ctx, "my/pkg/2/mac-amd64", []string{rev.Id}).Return(zeroResult, nil)

		skipReason, err := rf.Skip(ctx, rev)
		require.NoError(t, err)
		require.Equal(t, "CIPD package \"my/pkg/2/mac-amd64\" does not exist at revision \"git_revision:abc123def456\"", skipReason)

		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/1/linux-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/1/mac-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/2/linux-amd64", []string{rev.Id})
		client.AssertCalled(t, "SearchInstances", ctx, "my/pkg/2/mac-amd64", []string{rev.Id})
	})
}
