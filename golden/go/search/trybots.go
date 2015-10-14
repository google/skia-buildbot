package search

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/trybot"
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

// ListTrybotIssues returns the most recently updated trybot issues in reverse
// chronological order. offset and size determine the page and size of the
// returned list. The second return value is the total number of items
// available to facilitate pagination.
func ListTrybotIssues(storages *storage.Storage, offset, size int) ([]*TrybotIssue, int, error) {
	issueInfos, total, err := storages.TrybotResults.List(offset, size)
	if err != nil {
		return nil, 0, err
	}

	if len(issueInfos) == 0 {
		return []*TrybotIssue{}, 0, nil
	}

	ch := make(chan error, len(issueInfos))
	var wg sync.WaitGroup
	retMap := map[string]*TrybotIssue{}
	var mutex sync.Mutex

	for _, issueInfo := range issueInfos {
		wg.Add(1)
		go func(issueInfo *trybot.IssueListItem) {
			defer wg.Done()

			intIssueId, err := strconv.Atoi(issueInfo.Issue)
			if err != nil {
				ch <- fmt.Errorf("Unable to parse issue id %s. Got error: %s", issueInfo.Issue, err)
				return
			}
			result, err := storages.RietveldAPI.GetIssueProperties(intIssueId, false)
			if err != nil {
				ch <- fmt.Errorf("Error retrieving rietveld informaton for issue: %s: %s", issueInfo.Issue, err)
				return
			}

			ret := &TrybotIssue{
				ID:      result.Issue,
				Owner:   result.Owner,
				Subject: result.Subject,
				Updated: result.Modified.Unix(),
				URL:     strings.TrimSuffix(storages.RietveldAPI.Url(), "/") + "/" + strconv.Itoa(result.Issue),
			}

			mutex.Lock()
			retMap[issueInfo.Issue] = ret
			mutex.Unlock()

		}(issueInfo)
	}
	wg.Wait()
	close(ch)

	for err := range ch {
		return nil, 0, err
	}

	ret := make([]*TrybotIssue, 0, len(retMap))
	for _, issueInfo := range issueInfos {
		ret = append(ret, retMap[issueInfo.Issue])
	}

	return ret, total, nil
}
