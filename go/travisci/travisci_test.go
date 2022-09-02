package travisci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPullRequestBuilds(t *testing.T) {
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
		require.NoError(t, err)
	}))
	defer ts.Close()

	travisClient, err := NewTravisCI(context.Background(), "kryptonians", "krypton", "")
	travisClient.apiURL = ts.URL
	require.NoError(t, err)
	builds, err := travisClient.GetPullRequestBuilds(4868, "superman")
	require.NoError(t, err)
	require.Equal(t, 2, len(builds))
	require.Equal(t, 358526331, builds[0].Id)
	require.Equal(t, 1157, builds[0].Duration)
	require.Equal(t, "failed", builds[0].State)
	require.Equal(t, 987654321, builds[1].Id)
	require.Equal(t, 1542, builds[1].Duration)
	require.Equal(t, "passed", builds[1].State)
}
