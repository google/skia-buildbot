package gcr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestTags(t *testing.T) {
	unittest.SmallTest(t)
	url := fmt.Sprintf("https://%s/v2/skia-public/docserver/tags/list", SERVER)
	m := mockhttpclient.NewURLMock()
	m.Mock(url, mockhttpclient.MockGetDialogue([]byte(`{"tags": ["foo", "bar"]}`)))
	c := &Client{
		client:    m.Client(),
		projectId: "skia-public",
		imageName: "docserver",
	}
	tags, err := c.Tags()
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, tags)

	c.imageName = "unknown"
	tags, err = c.Tags()
	assert.Error(t, err)
}

func TestGcrTokenSource(t *testing.T) {
	unittest.SmallTest(t)
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
