package codesearch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
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
	unittest.SmallTest(t)

	var resp CompoundSearchResponse
	err := json.Unmarshal([]byte(searchResponseBody), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 174, resp.Response[0].EstimatedTotalNumberOfResults)
	assert.Equal(t, "src/third_party/skia/site/roles.md", resp.Response[0].SearchResult[0].TopFile.File.Name)
	assert.Equal(t, "CJv...wE=", resp.Response[0].NextPageToken)
}
