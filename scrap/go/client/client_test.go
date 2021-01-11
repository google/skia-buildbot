// Package client is a client for the Scrap Exchange REST API.
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/scrap/go/api"
	"go.skia.org/infra/scrap/go/scrap"
	"go.skia.org/infra/scrap/go/scrap/mocks"
)

var (
	errMyMockError = fmt.Errorf("My mock error")
)

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
	unittest.SmallTest(t)
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("Expand", testutils.AnyContext, scrap.SVG, "@smiley", scrap.CPP, mock.Anything).Run(func(args mock.Arguments) {
		_, err := args[4].(io.Writer).Write([]byte("<svg></svg>"))
		require.NoError(t, err)
	}).Return(nil)

	var b bytes.Buffer
	err := client.Expand(context.Background(), scrap.SVG, "@smiley", scrap.CPP, &b)
	require.NoError(t, err)
	require.Equal(t, `<svg></svg>`, b.String())
	scrapExchangeMock.AssertExpectations(t)
}

func TestExpand_ExpandError_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	scrapExchangeMock, client := setupForTest(t)

	scrapExchangeMock.On("Expand", testutils.AnyContext, scrap.SVG, "@smiley", scrap.CPP, mock.Anything).Return(errMyMockError)

	var b bytes.Buffer
	err := client.Expand(context.Background(), scrap.SVG, "@smiley", scrap.CPP, &b)
	require.Error(t, err)
}

func TestLoadScrap_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchangeMock.On("LoadScrap", testutils.AnyContext, scrap.SVG, "@smiley").Return(scrapBody, nil)

	returnedBody, err := client.LoadScrap(context.Background(), scrap.SVG, "@smiley")
	require.NoError(t, err)
	require.Equal(t, scrapBody, returnedBody)
	scrapExchangeMock.AssertExpectations(t)
}

func TestLoadScrap_LoadScrapError_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	scrapExchangeMock, client := setupForTest(t)

	scrapBody := scrap.ScrapBody{
		Type: scrap.SVG,
		Body: "<svg></svg>",
	}
	scrapExchangeMock.On("LoadScrap", testutils.AnyContext, scrap.SVG, "@smiley").Return(scrapBody, errMyMockError)

	_, err := client.LoadScrap(context.Background(), scrap.SVG, "@smiley")
	require.Error(t, err)
	scrapExchangeMock.AssertExpectations(t)
}
