// Package codesearch wraps up the codesearch JSON API.
//
// Notes about the codesearch REST API:
//
// The query needs to be bracketed by search_request=[b|e] query parameters.
//
// For example:
//    https://cs.chromium.org/codesearch/json/search_request:1
//       ?search_request=b
//       &query=file%3A.md+file%3A%5Esrc%2Fthird_party%2Fskia%2F+package%3A%5Echromium%24
//       &max_num_results=20
//       &results_offset=0
//       &search_request=e
//
// The URL for a file.name of "src/third_party/skia/site/roles.md" is
//
//    https://cs.chromium.org/chromium/src/third_party/skia/site/roles.md
//
// To jump to a specific line:
//
//    https://cs.chromium.org/chromium/src/third_party/skia/site/roles.md?l=2
//
package codesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	// origin is the site hosting codesearch.
	origin = "https://cs.chromium.org"

	// The location of the JSON codesearch API.
	jsonQueryPath = "/codesearch/json/search_request"

	// SkiaAllCode is the text to add to a search to restrict it to the Skia
	// codebase.
	SkiaAllCode = "file:^src/third_party/skia/ package:^chromium$"

	// SkiaInfraBaseQuery is the text to add to a search to restrict it to the
	// Skia Infra codebase.
	SkiaInfraBaseQuery = "file:^skia/buildbot/ package:^chromium$"

	// SkiaAllMarkdown is the text to add to a search to restrict it to Markdown
	// files in either the buildbot or skia proper repo.
	SkiaAllMarkdown = "lang:^markdown$ AND (file:^skia/buildbot/ OR file:^src/third_party/skia/) AND package:^chromium$ "
)

var (
	defaultQueryParams = url.Values{
		"search_request": []string{""},
	}
)

// CodeSearch searches code.
type CodeSearch struct {
	c      *http.Client
	origin string
}

// New creates a new CodeSearch instance.
func New(client *http.Client) *CodeSearch {
	return &CodeSearch{
		c:      client,
		origin: origin,
	}
}

// Origin allows over-riding the default origin.
func (cs *CodeSearch) Origin(origin string) {
	cs.origin = origin
}

// File is a file in a search result.
type File struct {
	Name        string `json:"name"`
	PackageName string `json:"package_name"`
}

// TopFile is the best matching file in a search result.
type TopFile struct {
	File File `json:"file"`
	Size int  `json:"size"`
}

// Text is the text of a snippet.
type Text struct {
	Text string `json:"text"`
}

// Snippet is a matching snippet of code in a search result.
type Snippet struct {
	Text            Text `json:"text"`
	FirstLineNumber int  `json:"first_line_number"`
}

// SearchResult is a single result from a search.
type SearchResult struct {
	TopFile                TopFile   `json:"top_file"`
	NumDuplicates          int       `json:"num_duplicates"`
	NumMatches             int       `json:"num_matches"`
	Language               string    `json:"language"`
	BestMatchingLineNumber int       `json:"best_matching_line_number"`
	Snippet                []Snippet `json:"snippet"`
}

// SearchResponse is the response from CodeSearch.Query.
type SearchResponse struct {
	Status                        int            `json:"status"`
	StatusMessage                 string         `json:"status_message"`
	EstimatedTotalNumberOfResults int            `json:"estimated_total_number_of_results"`
	ResultsOffset                 int            `json:"results_offset"`
	NextPageToken                 string         `json:"next_page_token"`
	SearchResult                  []SearchResult `json:"search_result"`
}

// CompoundSearchResponse represents multiple search responses.
type CompoundSearchResponse struct {
	Response []SearchResponse `json:"search_response"`
}

func (cs *CodeSearch) urlForQuery(q string, params url.Values) string {
	if params == nil {
		params = url.Values{}
	}
	params["query"] = []string{q}
	encodedQuery := params.Encode()
	return fmt.Sprintf("%s%s?search_request=b&%s&search_request=e", cs.origin, jsonQueryPath, encodedQuery)
}

// Query runs a search against the given search service.
//
// The query string should conform to any query you would use on
// https://cs.chromium.org/.
func (cs CodeSearch) Query(ctx context.Context, q string, params url.Values) (SearchResponse, error) {
	resp, err := httputils.GetWithContext(ctx, cs.c, cs.urlForQuery(q, params))
	if err != nil {
		return SearchResponse{}, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return SearchResponse{}, skerr.Fmt("Bad status code: %d", resp.StatusCode)
	}
	var csr CompoundSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&csr); err != nil {
		return SearchResponse{}, err
	}
	if len(csr.Response) < 1 {
		return SearchResponse{}, skerr.Fmt("No results body found")
	}
	return csr.Response[0], nil
}

// URL returns the link to display the TopFile in the SearchResult.
func (cs CodeSearch) URL(r SearchResult) string {
	return fmt.Sprintf("%s/%s/%s", cs.origin, r.TopFile.File.PackageName, r.TopFile.File.Name)
}
