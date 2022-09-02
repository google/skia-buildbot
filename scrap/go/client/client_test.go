// Package client is a client for the Scrap Exchange REST API.
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/scrap/go/api"
	"go.skia.org/infra/scrap/go/scrap"
	"go.skia.org/infra/scrap/go/scrap/mocks"
)

var (
	errMyMockError = fmt.Errorf("My mock error")
)

const (
	hash       = scrap.SHA256("f7b0bac33b5f5b3ac86bec9f33c2d1c3ef025a9e4282ca7a8b9bc01e40d40556")
	scrapName  = "@smiley"
	scrapName2 = "@frowny"
)

func TestCheckResponseForError_BadStatusCode_ReturnsError(t *testing.T) {

	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
	}

	require.Error(t, checkResponseForError(resp, nil))
}

func TestNew_InvalidURL_ReturnsError(t *testing.T) {
	_, err := New("not-a-valid-url\n")
	require.Error(t, err)
}

func setupForTest(t *testing.T) (*mocks.ScrapExchange, *Client) {
	scrapExchangeMock := &mocks.ScrapExchange{}
	a := api.New(scrapExchangeMock)
	router := mux.NewRouter()
	a.AddHandlers(router, api.AddProtectedEndpoints)

	// Create a test server that mocks out the scrap exchange.
	testServer := httptest.NewServer(router)
	t.Cleanup(testServer.Close)

	// Configure main to point at the test server.
	client, err := New(testServer.URL)
	require.NoError(t, err)
	return scrapExchangeMock, client
}

func TestExpand_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("Expand", testutils.AnyContext, scrap.SVG, scrapName, scrap.CPP, mock.Anything).Run(func(args mock.Arguments) {
		_, err := args.Get(4).(io.Writer).Write([]byte("<svg></svg>"))
		require.NoError(t, err)
	}).Return(nil)

	var b bytes.Buffer
	err := client.Expand(context.Background(), scrap.SVG, scrapName, scrap.CPP, &b)
	require.NoError(t, err)
	require.Equal(t, `<svg></svg>`, b.String())
	scrapExchangeMock.AssertExpectations(t)
}

func TestExpand_HappyPathButContextTimesOut_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	// Create and cancel a Context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var b bytes.Buffer
	err := client.Expand(ctx, scrap.SVG, scrapName, scrap.CPP, &b)
	require.Contains(t, err.Error(), "context canceled")
	scrapExchangeMock.AssertExpectations(t)
}

func TestExpand_ExpandError_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("Expand", testutils.AnyContext, scrap.SVG, scrapName, scrap.CPP, mock.Anything).Return(errMyMockError)

	var b bytes.Buffer
	err := client.Expand(context.Background(), scrap.SVG, scrapName, scrap.CPP, &b)
	require.Error(t, err)
}

func TestLoadScrap_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchangeMock.On("LoadScrap", testutils.AnyContext, scrap.SVG, scrapName).Return(scrapBody, nil)

	returnedBody, err := client.LoadScrap(context.Background(), scrap.SVG, scrapName)
	require.NoError(t, err)
	require.Equal(t, scrapBody, returnedBody)
	scrapExchangeMock.AssertExpectations(t)
}

func TestLoadScrap_LoadScrapError_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchangeMock.On("LoadScrap", testutils.AnyContext, scrap.SVG, scrapName).Return(scrapBody, errMyMockError)

	_, err := client.LoadScrap(context.Background(), scrap.SVG, scrapName)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestCreateScrap_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapID := scrap.ScrapID{
		Hash: hash,
	}
	scrapExchangeMock.On("CreateScrap", testutils.AnyContext, scrapBody).Return(scrapID, nil)

	returnedScrapID, err := client.CreateScrap(context.Background(), scrapBody)
	require.NoError(t, err)
	require.Equal(t, scrapID, returnedScrapID)
	scrapExchangeMock.AssertExpectations(t)
}

func TestCreateScrap_CreateReturnsError_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{}
	scrapID := scrap.ScrapID{}
	scrapExchangeMock.On("CreateScrap", testutils.AnyContext, scrapBody).Return(scrapID, errMyMockError)

	_, err := client.CreateScrap(context.Background(), scrapBody)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestDeleteScrap_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("DeleteScrap", testutils.AnyContext, scrap.SVG, scrapName).Return(nil)

	err := client.DeleteScrap(context.Background(), scrap.SVG, scrapName)
	require.NoError(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestDeleteScrap_DeleteErrors_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("DeleteScrap", testutils.AnyContext, scrap.SVG, scrapName).Return(errMyMockError)

	err := client.DeleteScrap(context.Background(), scrap.SVG, scrapName)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestPutName_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	name := scrap.Name{
		Hash: hash,
	}
	scrapExchangeMock.On("PutName", testutils.AnyContext, scrap.SVG, scrapName, name).Return(nil)

	err := client.PutName(context.Background(), scrap.SVG, scrapName, name)
	require.NoError(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestPutName_PutErrors_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	name := scrap.Name{
		Hash: hash,
	}
	scrapExchangeMock.On("PutName", testutils.AnyContext, scrap.SVG, scrapName, name).Return(errMyMockError)

	err := client.PutName(context.Background(), scrap.SVG, scrapName, name)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestGetName_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	name := scrap.Name{
		Hash: hash,
	}
	scrapExchangeMock.On("GetName", testutils.AnyContext, scrap.SVG, scrapName).Return(name, nil)

	returnedBody, err := client.GetName(context.Background(), scrap.SVG, scrapName)
	require.NoError(t, err)
	require.Equal(t, name, returnedBody)
	scrapExchangeMock.AssertExpectations(t)
}

func TestGetName_GetErrors_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	name := scrap.Name{
		Hash: hash,
	}
	scrapExchangeMock.On("GetName", testutils.AnyContext, scrap.SVG, scrapName).Return(name, errMyMockError)

	_, err := client.GetName(context.Background(), scrap.SVG, scrapName)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestDeleteName_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("DeleteName", testutils.AnyContext, scrap.SVG, scrapName).Return(nil)

	err := client.DeleteName(context.Background(), scrap.SVG, scrapName)
	require.NoError(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestDeleteName_DeleteErrors_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("DeleteName", testutils.AnyContext, scrap.SVG, scrapName).Return(errMyMockError)

	err := client.DeleteName(context.Background(), scrap.SVG, scrapName)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}

func TestListNames_HappyPath_Success(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	body := []string{scrapName, scrapName2}
	scrapExchangeMock.On("ListNames", testutils.AnyContext, scrap.SVG).Return(body, nil)

	returnedBody, err := client.ListNames(context.Background(), scrap.SVG)
	require.NoError(t, err)
	require.Equal(t, body, returnedBody)
	scrapExchangeMock.AssertExpectations(t)
}

func TestListNames_ListErrors_ReturnsError(t *testing.T) {
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("ListNames", testutils.AnyContext, scrap.SVG).Return(nil, errMyMockError)

	_, err := client.ListNames(context.Background(), scrap.SVG)
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}
