package pushgateway

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestPush(t *testing.T) {
	unittest.SmallTest(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics/job/test_job" {
			body, err := ioutil.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, "test_metric_name test_metric_value\n", string(body))
		} else {
			require.Fail(t, fmt.Sprintf("Unexpected path: %s", r.URL.Path))
		}
	}))
	defer ts.Close()

	p := New(httputils.NewTimeoutClient(), "test_job", ts.URL)
	err := p.Push(context.Background(), "test_metric_name", "test_metric_value")
	require.NoError(t, err)
}
