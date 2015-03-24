package issues

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

const (
	API_QUERY_TEMPLATE = "%s.&fields=items/id,items/state&key=%s"
	API_URL_TEMPLATE   = "https://www.googleapis.com/projecthosting/v2/projects/skia/issues?q=%s"
)

// IssueTracker is a genric interface to an issue tracker that allows us
// to connect issues with items (identified by an id).
type IssueTracker interface {
	// GetIssues returns the ids of the issues associated with itemID.
	GetIssues(itemID int64) ([]int64, error)
}

type CodesiteIssueTracker struct {
	apiKey      string
	urlTemplate string
}

func NewIssueTracker(apiKey, urlTemplate string) IssueTracker {
	return &CodesiteIssueTracker{
		apiKey:      apiKey,
		urlTemplate: urlTemplate,
	}
}

// Issue is an individual issue returned from the project hosting response.
//
// It is used in IssueResponse.
type Issue struct {
	ID int64 `json:"id"`
}

// IssueResponse is used to decode JSON responses from the project hosting API.
type IssueResponse struct {
	Items []*Issue `json:"items"`
}

// GetIssues is part of the IssueTracker interface. See documentation there.
func (c *CodesiteIssueTracker) GetIssues(itemID int64) ([]int64, error) {
	// The link to the issues is established by a URL embedded in the
	// description of the issue. That url will match the template in the
	// urlTemplate field combined with the itemID.

	// Search through the project hosting API for all issues that match that URI.
	searchURL := fmt.Sprintf(c.urlTemplate, itemID)
	qStr := fmt.Sprintf(API_QUERY_TEMPLATE, searchURL, c.apiKey)
	url := fmt.Sprintf(API_URL_TEMPLATE, url.QueryEscape(qStr))

	//  This will return a JSON response of the form:
	//
	//  {
	//   "items": [
	//    {
	//     "id": 2874,
	//     "state": "open"
	//    }
	//   ]
	//  }
	//
	// We don't currently use "state".

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)

	issueResponse := &IssueResponse{
		Items: []*Issue{},
	}
	if err := json.NewDecoder(resp.Body).Decode(&issueResponse); err != nil {
		return nil, err
	}

	glog.Infof("For %d Got %#v", itemID, issueResponse)
	bugs := make([]int64, 0, len(issueResponse.Items))
	for _, issue := range issueResponse.Items {
		bugs = append(bugs, issue.ID)
	}

	return bugs, nil
}
