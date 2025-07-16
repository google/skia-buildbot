package versionhistory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONToMarkdown(t *testing.T) {
	testCases := []struct {
		name     string
		jsonStr  string
		expected string
		hasErr   bool
	}{
		{
			name: "Valid JSON",
			jsonStr: `{
				"name": "John Doe",
				"age": 30,
				"isStudent": false,
				"courses": [
					{"title": "History", "credits": 3},
					{"title": "Math", "credits": 4}
				],
				"address": {
					"street": "123 Main St",
					"city": "Anytown"
				}
			}`,
			expected: `  - address:
    - city: Anytown
    - street: 123 Main St
  - age: 30
  - courses:
    -
      - credits: 3
      - title: History
    -
      - credits: 4
      - title: Math

  - isStudent: false
  - name: John Doe
`,
			hasErr: false,
		},
		{
			name:     "Invalid JSON",
			jsonStr:  `{"name": "John Doe", "age": 30,`,
			expected: "",
			hasErr:   true,
		},
		{
			name:     "Empty JSON",
			jsonStr:  `{}`,
			expected: "",
			hasErr:   false,
		},
		{
			name:     "JSON with array",
			jsonStr:  `[1, "two", true]`,
			expected: "  - 1\n  - two\n  - true\n\n",
			hasErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			markdown, err := JSONToMarkdown(tc.jsonStr)
			if tc.hasErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, markdown)
			}
		})
	}
}

func TestNewVersionHistoryClient(t *testing.T) {
	client := NewVersionHistoryClient()
	assert.NotNil(t, client)
	assert.NotNil(t, client.c)
	assert.Equal(t, "https", client.u.Scheme)
	assert.Equal(t, "versionhistory.googleapis.com", client.u.Host)
}

func TestListChromePlatformsHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chrome/platforms", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "platforms": [
    {
      "name": "chrome/platforms/win64",
      "platformType": "WIN64"
    },
    {
      "name": "chrome/platforms/ios",
      "platformType": "IOS"
    }
  ]
}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewVersionHistoryClient()
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client.u.Scheme = serverURL.Scheme
	client.u.Host = serverURL.Host

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name: "list_chrome_platforms",
		},
	}

	actual, err := client.ListChromePlatformsHandler(context.Background(), req)
	require.NoError(t, err)

	expected := mcp.NewToolResultText(`Supported platforms:
  - platforms:
    -
      - name: chrome/platforms/win64
      - platformType: WIN64
    -
      - name: chrome/platforms/ios
      - platformType: IOS

`)
	assert.Equal(t, expected, actual)
}

func TestListChromeChannelsHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chrome/platforms/all/channels", r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "channels": [
    {
      "name": "chrome/platforms/win/channels/stable",
      "channelType": "STABLE"
    },
    {
      "name": "chrome/platforms/win/channels/beta",
      "channelType": "BETA"
    }
  ]
}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewVersionHistoryClient()
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client.u.Scheme = serverURL.Scheme
	client.u.Host = serverURL.Host

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name: "list_chrome_channels",
		},
	}

	actual, err := client.ListChromeChannelsHandler(context.Background(), req)
	require.NoError(t, err)

	expected := mcp.NewToolResultText(`Available channels:
  - channels:
    -
      - channelType: STABLE
      - name: chrome/platforms/win/channels/stable
    -
      - channelType: BETA
      - name: chrome/platforms/win/channels/beta

`)
	assert.Equal(t, expected, actual)
}

func TestListActiveReleasesHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chrome/platforms/all/channels/all/versions/all/releases", r.URL.Path)
		assert.Equal(t, "filter=endtime=none", r.URL.RawQuery)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "releases": [
    {
      "name": "chrome/platforms/win/channels/stable/versions/126.0.6478.63/releases/1234567890",
      "serving": {
				"startTime": "2024-06-15T16:10:38.525185Z"
			},
			"fraction": 0.25,
			"version": "126.0.6478.63",
			"fractionGroup": "61",
			"pinnable": true
    }
  ]
}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewVersionHistoryClient()
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client.u.Scheme = serverURL.Scheme
	client.u.Host = serverURL.Host

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name: "list_active_releases",
		},
	}

	actual, err := client.ListActiveReleasesHandler(context.Background(), req)
	require.NoError(t, err)

	expected := mcp.NewToolResultText(`Active releases:
  - releases:
    -
      - fraction: 0.25
      - fractionGroup: 61
      - name: chrome/platforms/win/channels/stable/versions/126.0.6478.63/releases/1234567890
      - pinnable: true
      - serving:
        - startTime: 2024-06-15T16:10:38.525185Z
      - version: 126.0.6478.63

`)
	assert.Equal(t, expected, actual)
}

func TestListReleaseInfoHandler(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chrome/platforms/win/channels/all/versions/all/releases", r.URL.Path)
		assert.Equal(t, "filter=starttime%3E2024-06-18T00:00:00Z,starttime%3C2024-06-19T00:00:00Z&order_by=starttime%20desc", r.URL.RawQuery)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
  "releases": [
    {
      "name": "chrome/platforms/win/channels/stable/versions/126.0.6478.63/releases/1234567890",
      "serving": {
				"startTime": "2024-06-18T16:10:38.525185Z"
			},
			"fraction": 0.25,
			"version": "126.0.6478.63",
			"fractionGroup": "61",
			"pinnable": true
    }
  ]
}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := NewVersionHistoryClient()
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	client.u.Scheme = serverURL.Scheme
	client.u.Host = serverURL.Host

	req := mcp.CallToolRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: mcp.CallToolParams{
			Name: "list_release_info",
			Arguments: map[string]interface{}{
				"filter_end_time":   "2024-06-19T00:00:00Z",
				"filter_start_time": "2024-06-18T00:00:00Z",
				"platform":          "win",
			},
		},
	}

	actual, err := client.ListReleaseInfoHandler(context.Background(), req)
	require.NoError(t, err)

	expected := mcp.NewToolResultText(`Chrome releases found:
  - releases:
    -
      - fraction: 0.25
      - fractionGroup: 61
      - name: chrome/platforms/win/channels/stable/versions/126.0.6478.63/releases/1234567890
      - pinnable: true
      - serving:
        - startTime: 2024-06-18T16:10:38.525185Z
      - version: 126.0.6478.63

`)
	assert.Equal(t, expected, actual)
}
