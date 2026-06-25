package pinpoint

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/mockhttpclient"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

const mockGerritUrl = "https://chromium-review.googlesource.com/a/changes/123456/detail?o=ALL_REVISIONS&o=SUBMITTABLE"

func TestClientGetPatch_Success(t *testing.T) {
	ctx := context.Background()

	respBody := []byte(`)]}'
{
  "id": "chromium%2Fsrc~main~I123456",
  "project": "chromium/src",
  "branch": "main",
  "subject": "Commit message",
  "status": "MERGED",
  "created": "2026-01-01 00:00:00.000000",
  "updated": "2026-06-12 13:00:00.000000",
  "_number": 123456,
  "owner": {
    "email": "somebody@google.com"
  },
  "revisions": {
    "rev123": {
      "_number": 3,
      "created": "2026-01-01 00:00:00.000000",
      "kind": "REWORK",
      "uploader": {
        "email": "somebody@google.com"
      },
      "ref": "refs/changes/56/123456/3"
    }
  }
}`)

	urlMock := mockhttpclient.NewURLMock()
	urlMock.MockOnce(mockGerritUrl, mockhttpclient.MockGetDialogue(respBody))

	c := &Client{
		gerritHttpClient: urlMock.Client(),
	}

	patchset := int64(3)
	resp, err := c.GetPatch(ctx, &pb.GetPatchRequest{
		Host:     "https://chromium-review.googlesource.com",
		Change:   123456,
		Patchset: &patchset,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "https://chromium-review.googlesource.com", resp.Host)
	assert.Equal(t, int64(123456), resp.Change)
	assert.Equal(t, int64(3), resp.Patchset)
	assert.Equal(t, "chromium/src", resp.Project)
	assert.Equal(t, "somebody@google.com", resp.Author)
	assert.Equal(t, "Commit message", resp.Subject)
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), resp.Created.AsTime())

	// Test with latest patchset (nil patchset in request).
	urlMock.MockOnce(mockGerritUrl, mockhttpclient.MockGetDialogue(respBody))
	resp, err = c.GetPatch(ctx, &pb.GetPatchRequest{
		Host:   "https://chromium-review.googlesource.com",
		Change: 123456,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(3), resp.Patchset)
}

func TestClientGetPatch_GerritError(t *testing.T) {
	ctx := context.Background()

	urlMock := mockhttpclient.NewURLMock()
	urlMock.MockOnce(mockGerritUrl, mockhttpclient.MockGetError("Not Found", http.StatusNotFound))

	c := &Client{
		gerritHttpClient: urlMock.Client(),
	}

	resp, err := c.GetPatch(ctx, &pb.GetPatchRequest{
		Host:   "https://chromium-review.googlesource.com",
		Change: 123456,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Failed to get change 123456 from Gerrit")
}

func TestClientGetPatch_InvalidHost(t *testing.T) {
	ctx := context.Background()
	c := &Client{}

	// Test with a malicious host
	resp, err := c.GetPatch(ctx, &pb.GetPatchRequest{
		Host:   "https://malicious-site.com",
		Change: 123456,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Invalid or untrusted Gerrit host")

	// Test with a host lacking HTTPS scheme
	resp, err = c.GetPatch(ctx, &pb.GetPatchRequest{
		Host:   "http://chromium-review.googlesource.com",
		Change: 123456,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Invalid or untrusted Gerrit host")
}
