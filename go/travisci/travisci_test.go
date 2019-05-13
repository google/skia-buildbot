package travisci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPullRequestBuilds(t *testing.T) {
	unittest.SmallTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintln(w, `
{
  "@type": "builds",
  "builds": [
    {
	  "@type": "build",
	  "id": 358526331,
	  "pull_request_number": 4868,
	  "duration": 1157,
	  "state": "failed"
	},
	{
	  "@type": "build",
	  "id": 987654321,
	  "pull_request_number": 4868,
	  "duration": 1542,
	  "state": "passed"
	},
	{
	  "@type": "build",
	  "id": 123456789,
	  "pull_request_number": 1111,
	  "duration": 234,
	  "state": "passed"
	}
  ]
}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	travisClient, err := NewTravisCI(context.Background(), "kryptonians", "krypton", "")
	travisClient.apiURL = ts.URL
	assert.NoError(t, err)
	builds, err := travisClient.GetPullRequestBuilds(4868, "superman")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(builds))
	assert.Equal(t, 358526331, builds[0].Id)
	assert.Equal(t, 1157, builds[0].Duration)
	assert.Equal(t, "failed", builds[0].State)
	assert.Equal(t, 987654321, builds[1].Id)
	assert.Equal(t, 1542, builds[1].Duration)
	assert.Equal(t, "passed", builds[1].State)
}
