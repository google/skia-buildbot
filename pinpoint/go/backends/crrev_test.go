package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
)

// from https://cr-rev.appspot.com/_ah/api/crrev/v1/redirect/1291010
const (
	mockCommitHash     = "f05a3520e20212c3da51f4005f6f80e64f7f9ccb"
	mockCommitPosition = "1291010"
	mockCrrevResp      = `{"git_sha":"f05a3520e20212c3da51f4005f6f80e64f7f9ccb","project":"chromium","repo":"chromium/src","redirectUrl":"https://chromium.googlesource.com/chromium/src/+/f05a3520e20212c3da51f4005f6f80e64f7f9ccb"}`
)

func unmarshalMockCrrevResp(data string) (*CrrevResponse, error) {
	var resp CrrevResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, skerr.Wrapf(err, "could not generate expected mock response")
	}
	return &resp, nil
}

func TestNewCrrevClient_GivenDefaults_ReturnsClient(t *testing.T) {
	cc, err := NewCrrevClient(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, cc)
}

func TestGetCommitInfo_GivenCommitPosition_ReturnsHash(t *testing.T) {
	expected, err := unmarshalMockCrrevResp(mockCrrevResp)
	require.NoError(t, err)

	ctx := context.Background()
	m := mockhttpclient.NewURLMock()
	m.MockOnce(fmt.Sprintf("%s/%s", crrevRedirectUrl, mockCommitPosition),
		mockhttpclient.MockGetDialogue([]byte(mockCrrevResp)))

	cr, err := NewCrrevClient(ctx)
	require.NoError(t, err)
	cr.Client = m.Client()
	actual, err := cr.GetCommitInfo(ctx, mockCommitPosition)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetCommitInfo_GivenCommitHash_ReturnsHash(t *testing.T) {
	expected, err := unmarshalMockCrrevResp(mockCrrevResp)
	require.NoError(t, err)

	ctx := context.Background()
	m := mockhttpclient.NewURLMock()
	m.MockOnce(fmt.Sprintf("%s/%s", crrevRedirectUrl, mockCommitHash),
		mockhttpclient.MockGetDialogue([]byte(mockCrrevResp)))

	cr, err := NewCrrevClient(ctx)
	require.NoError(t, err)
	cr.Client = m.Client()
	actual, err := cr.GetCommitInfo(ctx, mockCommitHash)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
