package versionhistory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	vhPlatforms    = "platforms"
	vhChannels     = "channels"
	vhVersions     = "versions"
	vhReleases     = "releases"
	vhAll          = "all"
	queryNoEndtime = "filter=endtime=none"
	queryStarttime = "filter=starttime%%3E%s,starttime%%3C%s"
	queryOrderBy   = "order_by=starttime%20desc"
)

// VersionHistoryClient is a client for interacting with an HTTPS API.
type VersionHistoryClient struct {
	// The HTTP client used to interact with the API.
	c *http.Client
	// The URL object for creating query Version History URLs.
	u *url.URL
}

// NewVersionHistoryClient creates a new VersionHistoryClient.
func NewVersionHistoryClient() *VersionHistoryClient {
	return &VersionHistoryClient{
		c: &http.Client{
			Timeout: 30 * time.Second,
		},
		u: &url.URL{
			Scheme: "https",
			Host:   "versionhistory.googleapis.com",
			Path:   "v1/chrome",
		},
	}
}

// JSONToMarkdown converts a JSON string into a Markdown formatted string.
// It uses nested bullet points and appropriate indentation for all structures.
func JSONToMarkdown(jsonStr string) (string, error) {
	var data interface{}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	var sb strings.Builder
	// Start the recursive conversion from the root of the JSON data.
	// The depth starts at 0, and there's no parent key for the root element.
	convertValueToMarkdown(&sb, data, 0, "")

	return sb.String(), nil
}

// convertValueToMarkdown is a recursive helper function to build the Markdown string.
// sb: The strings.Builder to append Markdown to.
// val: The current JSON value (interface{} to handle any type).
// depth: Current nesting depth (0 for root, 1 for first level children, etc.).
// parentKey: The key that led to the current 'val' (only applicable if 'val' is a value in a map).
//
//	It's used for labelling. An empty string "" indicates it's an element of an array
//	or the root element.
func convertValueToMarkdown(sb *strings.Builder, val interface{}, depth int, parentKey string) {
	// Calculate indentation for list items and deeper content
	indent := strings.Repeat("  ", depth)

	switch v := val.(type) {
	case map[string]interface{}:
		// Handle JSON objects (Go maps)
		if parentKey != "" {
			// If this object is a value of a key (e.g., "address" in "user_profile")
			sb.WriteString(fmt.Sprintf("%s- %s:\n", indent, parentKey))
		} else if depth > 0 {
			// If this object is an element of an array (e.g., an item in "products" list)
			// It needs its own bullet point
			sb.WriteString(fmt.Sprintf("%s-\n", indent))
		}

		// Collect and sort keys for consistent Markdown output order
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Recursively process each key-value pair in the object
		for _, k := range keys {
			childVal := v[k]
			// Children of an object are always one level deeper
			convertValueToMarkdown(sb, childVal, depth+1, k)
		}

	case []interface{}:
		// Handle JSON arrays (Go slices)
		if parentKey != "" {
			// If this array is a value of a key (e.g., "hobbies" in "user_profile")
			sb.WriteString(fmt.Sprintf("%s- %s:\n", indent, parentKey))
		} else if depth > 0 {
			// If this array is an element of an array (less common, but possible,
			// e.g., [[1,2],[3,4]])
			sb.WriteString(fmt.Sprintf("%s- \n", indent))
		}

		// Elements of an array are always one level deeper than the array's own bullet/label
		for _, elem := range v {
			// Recursively process each element in the array.
			// ParentKey is empty for array elements, as they don't have a key themselves.
			convertValueToMarkdown(sb, elem, depth+1, "")
		}
		sb.WriteString("\n") // Add a newline after each list block for separation

	default:
		// Handle scalar values (strings, numbers, booleans, null)
		if parentKey != "" {
			// If a parent key exists, format as a key-value pair in a list-like style
			sb.WriteString(fmt.Sprintf("%s- %s: %s\n", indent, parentKey, formatScalar(val)))
		} else {
			// If no parent key (e.g., the root of the JSON is a scalar, or it's an item in a list),
			// just print the scalar value with appropriate indentation as a bullet point.
			sb.WriteString(fmt.Sprintf("%s- %s\n", indent, formatScalar(val)))
		}
	}
}

// formatScalar formats a scalar value (string, number, bool, null) for Markdown output.
func formatScalar(val interface{}) string {
	switch v := val.(type) {
	case string:
		// Do not quote strings
		return fmt.Sprintf("%v", v)
	case float64:
		// JSON numbers are unmarshaled as float64. Format as integer if it has no fractional part.
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null" // Explicitly represent null values
	default:
		// Fallback for any other unexpected scalar types
		return fmt.Sprintf("%v", v)
	}
}

// Get sends a GET request to the specified URL and returns the response body.
func (vhc *VersionHistoryClient) queryAPI(ctx context.Context, url string) ([]byte, error) {
	sklog.Infof("Version History API query: %s", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create request for %s", url)
	}

	resp, err := vhc.c.Do(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to GET %s", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, skerr.Wrapf(readErr, "Bad status code from %s: %d, and failed to read response body", url, resp.StatusCode)
		}
		return nil, skerr.Fmt("Bad status code from %s: %d. Body: %s", url, resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read response body from %s", url)
	}

	return body, nil
}

func (vhc *VersionHistoryClient) ListChromePlatformsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url := vhc.u.JoinPath(vhPlatforms)
	body, err := vhc.queryAPI(ctx, url.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "Query Version History API failed")
	}

	md, err := JSONToMarkdown(string(body))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert JSON to Markdown")
	}
	return mcp.NewToolResultText(fmt.Sprintf("Supported platforms:\n%s", md)), nil
}

func (vhc *VersionHistoryClient) ListChromeChannelsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	platform := request.GetString(argPlatform, vhAll)
	url := vhc.u.JoinPath(vhPlatforms, platform, vhChannels)
	body, err := vhc.queryAPI(ctx, url.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "Query Version History API failed")
	}

	md, err := JSONToMarkdown(string(body))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert JSON to Markdown")
	}
	return mcp.NewToolResultText(fmt.Sprintf("Available channels:\n%s", md)), nil
}

func (vhc *VersionHistoryClient) ListActiveReleasesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url := vhc.u.JoinPath(vhPlatforms, vhAll, vhChannels, vhAll, vhVersions, vhAll, vhReleases)
	url.RawQuery = queryNoEndtime
	body, err := vhc.queryAPI(ctx, url.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "Query Version History API failed")
	}

	md, err := JSONToMarkdown(string(body))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert JSON to Markdown")
	}
	return mcp.NewToolResultText(fmt.Sprintf("Active releases:\n%s", md)), nil
}

func (vhc *VersionHistoryClient) ListReleaseInfoHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	st, err := request.RequireString(argStarttime)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	ed, err := request.RequireString(argEndtime)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	platform := request.GetString(argPlatform, vhAll)
	channel := request.GetString(argChannel, vhAll)
	version := request.GetString(argVersion, vhAll)
	url := vhc.u.JoinPath(vhPlatforms, platform, vhChannels, channel, vhVersions, version, vhReleases)
	url.RawQuery = strings.Join([]string{fmt.Sprintf(queryStarttime, st, ed), queryOrderBy}, "&")

	body, err := vhc.queryAPI(ctx, url.String())
	if err != nil {
		return nil, skerr.Wrapf(err, "Query Version History API failed")
	}

	md, err := JSONToMarkdown(string(body))
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to convert JSON to Markdown")
	}
	return mcp.NewToolResultText(fmt.Sprintf("Chrome releases found:\n%s", md)), nil
}
