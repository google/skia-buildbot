package gcr

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestTags(t *testing.T) {
	ctx := context.Background()
	url := fmt.Sprintf("https://%s/v2/skia-public/docserver/tags/list?n=100", Server)
	m := mockhttpclient.NewURLMock()
	m.Mock(url, mockhttpclient.MockGetDialogue([]byte(`{
		"name": "docserver",
		"manifest": {"sha256:abc123": {
			"imageSizeBytes": "32",
			"layerId": "0",
			"tag": ["foo", "bar"],
			"timeCreatedMs": "1655387750000",
			"timeUploadedMs": "1655387810000"
		}},
		"tags": ["foo", "bar"]
	}`)))
	c := &Client{
		client:    m.Client(),
		projectId: "skia-public",
		imageName: "docserver",
	}
	tagsResp, err := c.Tags(ctx)
	assert.NoError(t, err)
	assert.Equal(t, &TagsResponse{
		Name: "docserver",
		Manifest: map[string]struct {
			ImageSizeBytes string   `json:"imageSizeBytes"`
			LayerID        string   `json:"layerId"`
			Tags           []string `json:"tag"`
			TimeCreatedMs  string   `json:"timeCreatedMs"`
			TimeUploadedMs string   `json:"timeUploadedMs"`
		}{
			"sha256:abc123": {
				ImageSizeBytes: "32",
				LayerID:        "0",
				Tags:           []string{"foo", "bar"},
				TimeCreatedMs:  "1655387750000",
				TimeUploadedMs: "1655387810000",
			},
		},
		Tags: []string{"foo", "bar"},
	}, tagsResp)

	c.imageName = "unknown"
	tagsResp, err = c.Tags(ctx)
	assert.Error(t, err)
}

func TestTags_Pagination(t *testing.T) {

	ctx := context.Background()
	url := fmt.Sprintf("https://%s/v2/skia-public/docserver/tags/list?n=100", Server)
	m := mockhttpclient.NewURLMock()
	nextUrl := fmt.Sprintf("https://%s/v2/skia-public/docserver/tags/list?n=100&last=bar", Server)
	m.Mock(url, mockhttpclient.MockGetDialogueWithResponseHeaders(
		[]byte(`{
			"name": "docserver",
			"manifest": {"sha256:abc123": {
				"imageSizeBytes": "32",
				"layerId": "0",
				"tag": ["foo", "bar"],
				"timeCreatedMs": "1655387750000",
				"timeUploadedMs": "1655387810000"
			}},
			"tags": ["foo", "bar"]
		}`),
		map[string][]string{
			"Link": {nextUrl + "; rel=\"next\""},
		},
	))
	m.Mock(nextUrl, mockhttpclient.MockGetDialogue([]byte(`{
		"name": "docserver",
		"manifest": {"sha256:def456": {
			"imageSizeBytes": "64",
			"layerId": "0",
			"tag": ["baz"],
			"timeCreatedMs": "1655387750000",
			"timeUploadedMs": "1655387810000"
		}},
		"tags": ["baz"]
	}`)))
	c := &Client{
		client:    m.Client(),
		projectId: "skia-public",
		imageName: "docserver",
	}
	tagsResp, err := c.Tags(ctx)
	assert.NoError(t, err)
	assert.Equal(t, &TagsResponse{
		Name: "docserver",
		Manifest: map[string]struct {
			ImageSizeBytes string   `json:"imageSizeBytes"`
			LayerID        string   `json:"layerId"`
			Tags           []string `json:"tag"`
			TimeCreatedMs  string   `json:"timeCreatedMs"`
			TimeUploadedMs string   `json:"timeUploadedMs"`
		}{
			"sha256:abc123": {
				ImageSizeBytes: "32",
				LayerID:        "0",
				Tags:           []string{"foo", "bar"},
				TimeCreatedMs:  "1655387750000",
				TimeUploadedMs: "1655387810000",
			},
			"sha256:def456": {
				ImageSizeBytes: "64",
				LayerID:        "0",
				Tags:           []string{"baz"},
				TimeCreatedMs:  "1655387750000",
				TimeUploadedMs: "1655387810000",
			},
		},
		Tags: []string{"foo", "bar", "baz"},
	}, tagsResp)

	c.imageName = "unknown"
	tagsResp, err = c.Tags(ctx)
	assert.Error(t, err)
}

func TestGcrTokenSource(t *testing.T) {
	m := mockhttpclient.NewURLMock()
	url := fmt.Sprintf("https://%s/v2/token?scope=repository:skia-public/docserver:pull", Server)
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
