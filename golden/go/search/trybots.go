package search

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

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

// ListTrybotIssues returns the most recently updated trybot issues in reverse
// chronological order. offset and size determine the page and size of the
// returned list. The second return value is the total number of items
// available to facilitate pagination.
func ListTrybotIssues(storages *storage.Storage, offset, size int) ([]*TrybotIssue, int, error) {
	issueIds, total, err := storages.TrybotResults.List(offset, size)
	if err != nil {
		return nil, 0, err
	}

	if len(issueIds) == 0 {
		return []*TrybotIssue{}, 0, nil
	}

	ch := make(chan error, len(issueIds))
	var wg sync.WaitGroup
	retMap := map[string]*TrybotIssue{}
	var mutex sync.Mutex

	for _, issueId := range issueIds {
		wg.Add(1)
		go func(issueId string) {
			defer wg.Done()

			intIssueId, err := strconv.Atoi(issueId)
			if err != nil {
				ch <- fmt.Errorf("Unable to parse issue id %s. Got error: %s", issueId, err)
				return
			}
			result, err := storages.RietveldAPI.GetIssueProperties(intIssueId, false)
			if err != nil {
				ch <- fmt.Errorf("Error retrieving rietveld informaton for issue: %s: %s", issueId, err)
				return
			}

			ret := &TrybotIssue{
				ID:      result.Issue,
				Owner:   result.Owner,
				Subject: result.Subject,
				Updated: result.Modified.Unix(),
				URL:     strings.TrimSuffix(storages.RietveldAPI.Url, "/") + "/" + strconv.Itoa(result.Issue),
			}

			mutex.Lock()
			retMap[issueId] = ret
			mutex.Unlock()

		}(issueId)
	}
	wg.Wait()
	close(ch)

	for err := range ch {
		return nil, 0, err
	}

	ret := make([]*TrybotIssue, 0, len(retMap))
	for _, issueId := range issueIds {
		ret = append(ret, retMap[issueId])
	}

	return ret, total, nil
}
