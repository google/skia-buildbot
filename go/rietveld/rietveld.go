package rietveld

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

import (
	"github.com/golang/glog"
)

var (
	committedIssueRegexp []string = []string{
		"Committed patchset #[0-9]+ \\((id:)?[0-9]+\\) as [0-9a-f]{2,40}",
		"Change committed as [0-9]+",
	}
)

// Issue contains information about a Rietveld issue.
type Issue struct {
	CC             []string
	Closed         bool
	Committed      bool
	Created        time.Time
	CreatedString  string `json:"created"`
	Description    string
	Issue          int
	Messages       []IssueMessage
	Modified       time.Time
	ModifiedString string `json:"modified"`
	Owner          string
	Project        string
	Reviewers      []string
	Subject        string
}

// IssueMessage contains information about a comment on an issue.
type IssueMessage struct {
	Date       time.Time
	DateString string `json:"date"`
	Sender     string
	Text       string
}

type issueListSortable []*Issue

func (p issueListSortable) Len() int           { return len(p) }
func (p issueListSortable) Less(i, j int) bool { return p[i].Created.Before(p[j].Created) }
func (p issueListSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type rietveldResults struct {
	Results []*Issue
	Cursor  string
}

func parseTime(t string) time.Time {
	parsed, _ := time.Parse("2006-01-02 15:04:05.999999", t)
	return parsed
}

// Rietveld is an object used for interacting with the issue tracker.
type Rietveld struct {
	Url string
}

func (r Rietveld) get(suburl string, rv interface{}) error {
	resp, err := http.Get(r.Url + suburl)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %v", r.Url+suburl, err)
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	return nil
}

// SearchTerm is a wrapper for search terms to pass into the Search method.
type SearchTerm struct {
	Key   string
	Value string
}

func SearchOwner(name string) *SearchTerm {
	return &SearchTerm{
		Key:   "owner",
		Value: name,
	}
}

func SearchModifiedAfter(after time.Time) *SearchTerm {
	return &SearchTerm{
		Key:   "modified_after",
		Value: url.QueryEscape(strings.Trim(strings.Split(after.UTC().String(), "+")[0], " ")),
	}
}

func SearchOpen(open bool) *SearchTerm {
	value := "2"
	if open {
		value = "3"
	}
	return &SearchTerm{
		Key:   "closed",
		Value: value,
	}
}

func SearchLimit(limit int) *SearchTerm {
	return &SearchTerm{
		Key:   "limit",
		Value: fmt.Sprintf("%d", limit),
	}
}

// Search returns a slice of Issues which fit the given criteria.
func (r Rietveld) Search(terms ...*SearchTerm) ([]*Issue, error) {
	searchUrl := "/search?format=json"
	for _, term := range terms {
		searchUrl += fmt.Sprintf("&%s=%s", term.Key, term.Value)
	}

	var issues issueListSortable
	cursor := ""
	for {
		var data rietveldResults
		err := r.get(searchUrl+cursor, &data)
		if err != nil {
			return nil, fmt.Errorf("Rietveld search failed: %v", err)
		}
		if len(data.Results) == 0 {
			break
		}
		for _, issue := range data.Results {
			fullIssue, err := r.getIssueProperties(issue.Issue, true)
			if err != nil {
				glog.Error(err)
			} else {
				fullIssue.Created = parseTime(fullIssue.CreatedString)
				fullIssue.Modified = parseTime(fullIssue.ModifiedString)
				for _, msg := range fullIssue.Messages {
					committed := false
					for _, r := range committedIssueRegexp {
						committed, err = regexp.MatchString(r, msg.Text)
						if committed {
							break
						}
					}
					msg.Date = parseTime(msg.DateString)
					if err != nil {
						glog.Error(err)
						continue
					}
					if committed {
						fullIssue.Committed = true
					}
				}
				issues = append(issues, &fullIssue)
			}
		}
		cursor = "&cursor=" + data.Cursor
	}
	sort.Sort(issues)
	return issues, nil
}

// getIssueProperties returns a fully filled-in Issue object, as opposed to
// the partial data returned by Rietveld's search endpoint.
func (r Rietveld) getIssueProperties(issue int, messages bool) (Issue, error) {
	url := fmt.Sprintf("/api/%v", issue)
	if messages {
		url += "?messages=true"
	}
	var res Issue
	err := r.get(url, &res)
	if err != nil {
		return Issue{}, fmt.Errorf("Failed to load details for issue %d: %v", issue, err)
	}
	return res, nil
}

// New returns a new Rietveld instance.
func New(url string) Rietveld {
	url = strings.TrimRight(url, "/")
	return Rietveld{url}
}
