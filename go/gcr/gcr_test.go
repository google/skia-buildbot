package gcr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestTags(t *testing.T) {
	testutils.SmallTest(t)
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count == 0 {
			w.Header().Add("Link", "<?n=10&last=20>; rel=\"next\"")
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"tags": []string{"bar", "baz"},
			})
			assert.NoError(t, err)
			count += 1
		} else {
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"tags": []string{"quux"},
			})
			assert.NoError(t, err)
		}
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	assert.NoError(t, err)
	SCHEME = "http"
	SERVER = u.Host

	c := &Client{
		client:    httputils.NewTimeoutClient(),
		projectId: "skia-public",
		imageName: "docserver",
	}
	tags, err := c.Tags()
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar", "baz", "quux"}, tags)
}

func TestGcrTokenSource(t *testing.T) {
	testutils.SmallTest(t)
	m := mockhttpclient.NewURLMock()
	url := fmt.Sprintf("https://%s/v2/token?scope=repository:skia-public/docserver:pull", SERVER)
	m.Mock(url, mockhttpclient.MockGetDialogue([]byte(`{"token": "foo", "expires_in": 3600}`)))

	ts := &gcrTokenSource{
		client:    m.Client(),
		projectId: "skia-public",
		imageName: "docserver",
	}
	token, err := ts.Token()
	assert.NoError(t, err)
	assert.Equal(t, "foo", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)

	ts.imageName = "unknown"
	token, err = ts.Token()
	assert.Error(t, err)
}
