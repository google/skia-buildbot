package travisci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

//func TestHasOpenDependency(t *testing.T) {
//	travisClient, err := NewTravisCI(context.Background(), "", "flutter", "engine")
//	assert.NoError(t, err)
//	build, err2 := travisClient.GetLatestBuildDetails("rmistry", "4868")
//	assert.NoError(t, err2)
//	assert.Equal(t, build.State, "failed")
//}

func TestGetLatestBuildDetails(t *testing.T) {
	testutils.SmallTest(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `
{
  "@type": "builds",
  "builds": [
    {
	  "@type": "build",
	  "id": 358526331,
	  "number": "111594",
	  "duration": 1157,
	  "state": "failed"
	}
  ]
}
`)
	}))

	defer ts.Close()

	travisClient, err := NewTravisCI(context.Background(), "", "kryptonians", "krypton")
	travisClient.apiURL = ts.URL
	assert.NoError(t, err)
	build, err := travisClient.GetLatestBuildDetails("superman", "4868")
	assert.NoError(t, err)
	assert.Equal(t, 358526331, build.Id)
	assert.Equal(t, "111594", build.Number)
	assert.Equal(t, 1157, build.Duration)
	assert.Equal(t, "failed", build.State)
}
