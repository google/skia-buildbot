package catapult

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"golang.org/x/oauth2/google"
)

const (
	catapultBisectPostUrl  = "https://pinpoint-dot-chromeperf.appspot.com/api/job"
	catapultStagingPostUrl = "https://pinpoint-dot-chromeperf-stage.uc.r.appspot.com/api/job"
	contentType            = "application/protobuf"
)

type DatastoreResponse struct {
	Kind string `json:"kind"`

	// Pinpoint IDs are usually int, but now that we execute workflows with
	// a UUID, Pinpoint stores the IDs as string and handles them.
	ID string `json:"id"`
}

// CatapultClient contains an httpClient for writing to catpault
type CatapultClient struct {
	httpClient *http.Client
	url        string
}

// NewCatapultClient creates a new CatapultClient
func NewCatapultClient(ctx context.Context, prod bool) (*CatapultClient, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create catapult client.")
	}

	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).Client()
	if prod {
		return &CatapultClient{
			httpClient: client,
			url:        catapultBisectPostUrl,
		}, nil
	}
	return &CatapultClient{
		httpClient: client,
		url:        catapultStagingPostUrl,
	}, nil
}

func (cc *CatapultClient) WriteBisectToCatapult(ctx context.Context, content *pinpoint_proto.LegacyJobResponse) (*DatastoreResponse, error) {
	var buff bytes.Buffer
	err := json.NewEncoder(&buff).Encode(content)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to marshal content")
	}

	httpResponse, err := httputils.PostWithContext(ctx, cc.httpClient, cc.url, contentType, bytes.NewReader(buff.Bytes()))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get Pinpoint response")
	}
	if httpResponse.Body == nil {
		return nil, skerr.Wrap(err)
	}

	if httpResponse.StatusCode != http.StatusOK {
		return nil, skerr.Fmt("The catapult post request failed with status code %d", httpResponse.StatusCode)
	}

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read body from datastore response.")
	}

	var dsResp DatastoreResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		return nil, skerr.Wrapf(err, "Could not unmarshal response")
	}

	return &dsResp, nil
}
