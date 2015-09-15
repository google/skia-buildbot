package search

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/golden/go/storage"
)

// TrybotIssue is the output structure for a Rietveld issue that has
// trybot runs assoicated with it.
type TrybotIssue struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
	Owner   string `json:"owner"`
	Updated int64  `json:"updated"`
	URL     string `json:"url"`
}

// ListTrybotIssues returns the most recently updated trybot issues.
func ListTrybotIssues(storages *storage.Storage) ([]*TrybotIssue, error) {
	issueIds, err := storages.TrybotResults.List(30)
	if err != nil {
		return nil, err
	}

	if len(issueIds) == 0 {
		return []*TrybotIssue{}, nil
	}

	ch := make(chan interface{}, len(issueIds))
	var wg sync.WaitGroup

	for _, issueId := range issueIds {
		wg.Add(1)
		go func(issueId string) {
			defer wg.Done()

			intIssueId, err := strconv.Atoi(issueId)
			if err != nil {
				ch <- fmt.Errorf("Unable to parse issue id %s. Got error: %s", issueId, err)
				return
			}
			issue, err := storages.RietveldAPI.GetIssueProperties(intIssueId, false)
			if err != nil {
				ch <- fmt.Errorf("Error retrieving rietveld informaton for issue: %s: %s", issueId, err)
				return
			}
			ch <- issue
		}(issueId)
	}
	wg.Wait()
	close(ch)

	ret := make([]*TrybotIssue, 0, len(issueIds))
	for result := range ch {
		switch result := result.(type) {
		case *rietveld.Issue:
			glog.Infof("MODIFIED: %v", result.Modified)
			ret = append(ret, &TrybotIssue{
				ID:      result.Issue,
				Owner:   result.Owner,
				Subject: result.Subject,
				Updated: result.Modified.Unix(),
				URL:     strings.TrimSuffix(storages.RietveldAPI.Url, "/") + "/" + strconv.Itoa(result.Issue),
			})
		case error:
			return nil, err
		}
	}

	return ret, nil
}
