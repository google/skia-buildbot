package codesearch

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

const searchResponseBody = `{
  "search_response": [
	  {
		"status": 0,
		"estimated_total_number_of_results": 174,
		"maybe_skipped_documents": false,
		"results_offset": 0,
		"hit_max_results": false,
		"hit_max_to_score": false,
		"status_message": "",
		"percent_shards_skipped": 0,
		"called_local_augmentation": false,
		"next_page_token": "CJv...wE=",
		"search_result": [
		  {
			"top_file": {
			  "file": {
				"name": "src/third_party/skia/site/roles.md",
				"package_name": "chromium"
			  }
			},
			"num_duplicates": 0,
			"num_matches": 0,
			"language": "markdown",
			"docid": "svr-40oKURY",
			"has_unshown_matches": false,
			"is_augmented": false,
			"match_reason": {},
			"full_history_search": false
		  },
		  {
			"top_file": {
			  "file": {
				"name": "src/third_party/skia/site/index.md",
				"package_name": "chromium"
			  }
			},
			"num_duplicates": 0,
			"num_matches": 0,
			"language": "markdown",
			"docid": "vAHsr5oQ12k",
			"has_unshown_matches": false,
			"is_augmented": false,
			"match_reason": {},
			"full_history_search": false
		  }
		]
     }
  ]
}`

func TestParse(t *testing.T) {

	var resp CompoundSearchResponse
	err := json.Unmarshal([]byte(searchResponseBody), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 174, resp.Response[0].EstimatedTotalNumberOfResults)
	assert.Equal(t, "src/third_party/skia/site/roles.md", resp.Response[0].SearchResult[0].TopFile.File.Name)
	assert.Equal(t, "CJv...wE=", resp.Response[0].NextPageToken)
}

func TestCodeSearch_urlForQuery(t *testing.T) {

	tests := []struct {
		name   string
		q      string
		params url.Values
		want   string
	}{
		{
			name:   "nil params",
			q:      "file:.md",
			params: nil,
			want:   "https://cs.chromium.org/codesearch/json/search_request?search_request=b&query=file%3A.md&search_request=e",
		},
		{
			name:   "extra params",
			q:      "file:.md",
			params: url.Values{"foo": []string{"bar"}},
			want:   "https://cs.chromium.org/codesearch/json/search_request?search_request=b&foo=bar&query=file%3A.md&search_request=e",
		},
	}
	cs := New(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cs.urlForQuery(tt.q, tt.params)
			assert.Equal(t, tt.want, got, tt.name)
		})
	}
}
