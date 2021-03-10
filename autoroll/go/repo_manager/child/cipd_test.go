package child

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCIPDInstanceToRevision(t *testing.T) {
	unittest.SmallTest(t)

	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "instanceID123",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd.UnixTime(ts),
		},
		Tags: []cipd.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}
	expect := &revision.Revision{
		Id:     "instanceID123",
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID123",
		Display:     "instanceI...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID123",
	}
	rev := CIPDInstanceToRevision("some/package", "", pkg)
	require.Equal(t, expect, rev)

	// Test TagAsID.
	expect.Id = "version:5"
	expect.Display = "version:5"
	rev = CIPDInstanceToRevision("some/package", "version", pkg)
	require.Equal(t, expect, rev)

	// If we're missing the TagAsID, the Revision should be invalid.
	expect.Id = "instanceID123"
	expect.Display = "instanceI..."
	expect.InvalidReason = "No \"version\" tag"
}
