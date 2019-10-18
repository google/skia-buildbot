package codesearch

import (
	"net/http"
	"net/url"
)

const (
	// Origin is the site hosting codesearch.
	Origin = "https://cs.chromium.org"

	// The location of the JSON codesearch API.
	jsonQueryPath = "/codesearch/json/search_request"

	// SkiaBaseQuery is the text to add to a search to restrict it to the Skia
	// codebase.
	SkiaBaseQuery = "file:^src/third_party/skia/ package:^chromium$"

	// SkiaInfraBaseQuery is the text to add to a search to restrict it to the
	// Skia Infra codebase.
	SkiaInfraBaseQuery = "file:^skia/buildbot/ package:^chromium$"
)

var (
	defaultQueryParams = url.Values{
		"search_request": []string{""},
	}
)

// CodeSearch searches code.
type CodeSearch struct {
	c *http.Client
}

// New creates a new CodeSearch instance.
func New(client *http.Client) CodeSearch {
	return CodeSearch{
		c: client,
	}
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

// Query runs a search against the given search service.
//
// The query string should conform to
func (cs CodeSearch) Query(q string, params map[string]string) SearchResponse {
	return SearchResponse{}
}

/*
BUILDBOT:

file:.md file:^skia/buildbot/ package:^chromium$

  becomes

https://cs.chromium.org/codesearch/json/search_request:1
  ?search_request=b
  &query=file%3A.md+file%3A%5Eskia%2Fbuildbot%2F+package%3A%5Echromium%24
  &max_num_results=11
  &results_offset=0
  &exhaustive=false
  &return_snippets=true
  &return_all_snippets=false
  &return_line_matches=false
  &sort_results=false
  &lines_context=1
  &file_sizes=true
  &return_directories=true
  &return_local_augmented_results=true
  &return_decorated_snippets=true
  &full_history_search=false
  &internal_options=b
  &debug_level=0
  &consolidated_query=true
  &internal_options=e
  &search_request=e
  &sid=1571411516527
  &msid=1571411516527


SKIA:

file:.md file:^src/third_party/skia/ package:^chromium$

  becomes

https://cs.chromium.org/codesearch/json/search_request:1
  ?search_request=b
  &query=file%3A.md+file%3A%5Esrc%2Fthird_party%2Fskia%2F+package%3A%5Echromium%24
  &max_num_results=11
  &results_offset=0
  &exhaustive=false
  &return_snippets=true
  &return_all_snippets=false
  &return_line_matches=false
  &sort_results=false
  &lines_context=1
  &file_sizes=true
  &return_directories=true
  &return_local_augmented_results=true
  &return_decorated_snippets=true
  &full_history_search=false
  &internal_options=b
  &debug_level=0
  &consolidated_query=true
  &internal_options=e
  &search_request=e
  &sid=1571411514399
  &msid=1571411514399

Minimal viable search:

https://cs.chromium.org/codesearch/json/search_request:1?search_request=b&query=file%3A.md+file%3A%5Esrc%2Fthird_party%2Fskia%2F+package%3A%5Echromium%24&max_num_results=20&results_offset=0&search_request=e

Note that the query needs to be bracketed by search_request=[b|e] query parameters.

  Results look like:

{
  "search_response": [
    {
      "status": 0,
      "estimated_total_number_of_results": 176,
      "maybe_skipped_documents": false,
      "results_offset": 0,
      "hit_max_results": false,
      "hit_max_to_score": false,
      "status_message": "",
      "percent_shards_skipped": 0,
      "called_local_augmentation": false,
      "stats": {
        "retries": 0,
        "filter_ratio": 0,
        "first_phase_latency_micros": "24411"
      },
      "next_page_token": "CJvxp9CBvaDzEwiloO3Sv4a2xUYI++DSkdH2l9BZCOCo9sejysb1Zwi/7YvL2cLci4UBCO+7w7X7rN/HpAEIlqKp0LT8v/2yAQjprsPQ+ZX7gLwBCPPC0YjMqZjKwgEIwb6f8PfO5uP5AQj79eazroD0k/wB",
      "search_result": [
        {
          "top_file": {
            "file": {
              "name": "src/third_party/skia/site/roles.md",
              "package_name": "chromium",
              "revision": "0",
              "muppet_params": "CgYI/v///wc="
            },
            "size": "1586"
          },
          "num_duplicates": 0,
          "num_matches": 0,
          "language": "markdown",
          "docid": "svr-40oKURY",
          "has_unshown_matches": true,
          "is_augmented": false,
          "best_matching_line_number": 1,
          "match_reason": {},
          "full_history_search": false,
          "snippet": [
            {
              "text": {
                "text": "Project Roles\n=============\n\n"
              },
              "first_line_number": 1,
              "match_reason": {}
            }
          ]
        },
        {
          "top_file": {
            "file": {
              "name": "src/third_party/skia/site/index.md",
              "package_name": "chromium",
              "revision": "0",
              "muppet_params": "CgYI/v///wc="
            },
            "size": "3006"
          },
          "num_duplicates": 0,
          "num_matches": 0,
          "language": "markdown",
          "docid": "vAHsr5oQ12k",
          "has_unshown_matches": true,
          "is_augmented": false,
          "best_matching_line_number": 1,
          "match_reason": {},
          "full_history_search": false,
          "snippet": [
            {
              "text": {
                "text": "Skia Graphics Library\n=====================\n\n"
              },
              "first_line_number": 1,
              "match_reason": {}
            }
          ]
        },

The URL for a file.name of "src/third_party/skia/site/roles.md" is

  https://cs.chromium.org/chromium/src/third_party/skia/site/roles.md

To jump to a specific line:

  https://cs.chromium.org/chromium/src/third_party/skia/site/roles.md?l=2

*/
