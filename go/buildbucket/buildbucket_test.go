package buildbucket

import (
	"encoding/json"
	"testing"

	"github.com/gorilla/mux"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestGetTrybotsForCL(t *testing.T) {
	testutils.MediumTest(t)

	client := NewClient(httputils.NewTimeoutClient())
	tries, err := client.GetTrybotsForCL(2347, 7, "gerrit", "https://skia-review.googlesource.com")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tries))
}

func TestSerialize(t *testing.T) {
	testutils.SmallTest(t)
	tc := []struct {
		in  Properties
		out string
	}{
		{
			in:  Properties{},
			out: "{}", // Leave out all empty fields.
		},
		{
			in: Properties{
				Reason: "Triggered by SkiaScheduler",
			},
			out: "{\"reason\":\"Triggered by SkiaScheduler\"}",
		},
	}
	for _, c := range tc {
		b, err := json.Marshal(c.in)
		assert.NoError(t, err)
		assert.Equal(t, []byte(c.out), b)
	}
}

func TestRequestBuild(t *testing.T) {
	testutils.SmallTest(t)
	respBody := []byte(testutils.MarshalJSON(t, &buildBucketResponse{
		Build: &Build{},
		Error: nil,
		Kind:  "",
		Etag:  "",
	}))
	reqType := "application/json; charset=utf-8"
	reqBody := []byte(`{"bucket":"master.my-master","parameters_json":"{\"builder_name\":\"my-builder\",\"changes\":[{\"author\":{\"email\":\"my-author\"},\"repo_url\":\"my-repo\",\"revision\":\"abc123\"}],\"properties\":{\"reason\":\"Triggered by SkiaScheduler\"}}"}`)
	r := mux.NewRouter()
	md := mockhttpclient.MockPutDialogue(reqType, reqBody, respBody)
	r.Schemes("https").Host("cr-buildbucket.appspot.com").Methods("PUT").Path("/api/buildbucket/v1/builds").Handler(md)
	httpClient := mockhttpclient.NewMuxClient(r)
	c := NewClient(httpClient)
	b, err := c.RequestBuild("my-builder", "my-master", "abc123", "my-repo", "my-author", "")
	assert.NoError(t, err)
	assert.NotNil(t, b)
}
