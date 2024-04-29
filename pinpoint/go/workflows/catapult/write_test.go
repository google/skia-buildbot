package catapult

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	mockJobId         = "179a34b2be0000"
	mockDatastoreResp = `{"kind":"Job","id":5743261448667136}`
)

var mockPinpointLegacyJobResp = &pinpoint_proto.LegacyJobResponse{
	JobId: mockJobId,
}

func unmarshalMockDatastoreResp(data string) (*DatastoreResponse, error) {
	var resp DatastoreResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, skerr.Wrapf(err, "could not generate expected mock response")
	}
	return &resp, nil
}

func TestNewCatapultClient_GivenDefaults_ReturnsClient(t *testing.T) {
	cc, err := NewCatapultClient(context.Background(), false)
	assert.NoError(t, err)
	assert.NotNil(t, cc)
	assert.Equal(t, cc.url, catapultBisectPostUrl)
}

func TestNewCatapultClient_GivenStaging_ReturnsStagingClient(t *testing.T) {
	cc, err := NewCatapultClient(context.Background(), true)
	assert.NoError(t, err)
	assert.NotNil(t, cc)
	assert.Equal(t, cc.url, catapultStagingPostUrl)
}

func TestWriteBisectToCatapault_GivenValidInput_ReturnsResponse(t *testing.T) {
	b, err := json.Marshal(mockPinpointLegacyJobResp)
	require.NoError(t, err)

	expected, err := unmarshalMockDatastoreResp(mockDatastoreResp)
	require.NoError(t, err)

	ctx := context.Background()
	m := mockhttpclient.NewURLMock()
	mockPost := mockhttpclient.MockPostDialogue(contentType, b, []byte(mockDatastoreResp))
	m.MockOnce(catapultStagingPostUrl, mockPost)

	cc, err := NewCatapultClient(ctx, true)
	require.NoError(t, err)
	cc.httpClient = m.Client()
	resp, err := cc.WriteBisectToCatapult(ctx, mockPinpointLegacyJobResp)
	require.NoError(t, err)
	assert.Equal(t, expected, resp)
}

func TestWriteBisectToCatapault_GivenBadStatusCode_ReturnsError(t *testing.T) {
	b, err := json.Marshal(mockPinpointLegacyJobResp)
	require.NoError(t, err)

	ctx := context.Background()
	m := mockhttpclient.NewURLMock()
	mockPost := mockhttpclient.MockPostDialogueWithResponseCode(contentType, b, []byte(""), 400)
	m.MockOnce(catapultStagingPostUrl, mockPost)

	cc, err := NewCatapultClient(ctx, true)
	require.NoError(t, err)
	cc.httpClient = m.Client()
	resp, err := cc.WriteBisectToCatapult(ctx, mockPinpointLegacyJobResp)
	require.Error(t, err)
	assert.Nil(t, resp)
}
