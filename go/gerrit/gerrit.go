package gerrit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

const (
	TIME_FORMAT      = "2006-01-02 15:04:05.999999"
	GERRIT_SKIA_URL  = "https://skia-review.googlesource.com"
	MAX_GERRIT_LIMIT = 500

	CODEREVIEW_LABEL  = "Code-Review"
	COMMITQUEUE_LABEL = "Commit-Queue"
)

// ChangeInfo contains information about a Gerrit issue.
type ChangeInfo struct {
	Created         time.Time
	CreatedString   string `json:"created"`
	Updated         time.Time
	UpdatedString   string `json:"updated"`
	Submitted       time.Time
	SubmittedString string `json:"submitted"`
	Project         string
	ChangeId        string `json:"change_id"`
	Subject         string
	Branch          string
	Committed       bool
	Revisions       map[string]Revision
	Patchsets       []*PatchSet
	MoreChanges     bool  `json:"_more_changes"`
	Issue           int64 `json:"_number"`
	Labels          map[string]*LabelEntry
}

type LabelEntry struct {
	All          []*LabelDetail
	Values       map[string]string
	DefaultValue int
}

type LabelDetail struct {
	Name  string
	Email string
	Date  string
	Value int
}

type Revision struct {
	CreatedString string `json:"created"`
}

type PatchSet struct {
	Created    time.Time
	RevisionId string
}

// Gerrit is an object used for iteracting with the issue tracker.
type Gerrit struct {
	client  *http.Client
	url     string
	cookies map[string]string
}

// NewGerrit returns a new Gerrit instance. If gitCookiesPath is empty then the
// .gitcookies in the users's home directory will be used for authenticated
// post calls like modifying issues.
func NewGerrit(url, gitCookiesPath string, client *http.Client) (*Gerrit, error) {
	url = strings.TrimRight(url, "/")
	if gitCookiesPath == "" {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		gitCookiesPath = filepath.Join(usr.HomeDir, ".gitcookies")
	}
	cookies, err := getCredentials(gitCookiesPath)
	if err != nil {
		return nil, err
	}

	if client == nil {
		client = httputils.NewTimeoutClient()
	}

	return &Gerrit{
		url:     url,
		client:  client,
		cookies: cookies,
	}, nil
}

// getCredentials returns the parsed contents of .gitCookies.
// This logic has been borrowed from
// https://cs.chromium.org/chromium/tools/depot_tools/gerrit_util.py?l=143
func getCredentials(gitCookiesPath string) (map[string]string, error) {
	gitCookies := map[string]string{}

	dat, err := ioutil.ReadFile(gitCookiesPath)
	if err != nil {
		return nil, err
	}
	contents := string(dat)
	for _, line := range strings.Split(contents, "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		tokens := strings.Split(line, "\t")
		domain, xpath, key, value := tokens[0], tokens[2], tokens[5], tokens[6]
		if xpath == "/" && key == "o" {
			auth := strings.SplitN(value, "=", 2)
			gitCookies[domain] = fmt.Sprintf("%s:%s", auth[0], auth[1])
		}
	}
	return gitCookies, nil
}

func parseTime(t string) time.Time {
	parsed, _ := time.Parse(TIME_FORMAT, t)
	return parsed
}

// GetIssueProperties returns a fully filled-in ChangeInfo object, as opposed to
// the partial data returned by Gerrit's search endpoint.
func (g *Gerrit) GetIssueProperties(issue int64) (*ChangeInfo, error) {
	url := fmt.Sprintf("/changes/%d/detail?o=ALL_REVISIONS", issue)
	fullIssue := &ChangeInfo{}
	if err := g.get(url, fullIssue); err != nil {
		return nil, fmt.Errorf("Failed to load details for issue %d: %v", issue, err)
	}

	// Set created, updated and submitted timestamps. Also set the committed flag.
	fullIssue.Created = parseTime(fullIssue.CreatedString)
	fullIssue.Updated = parseTime(fullIssue.UpdatedString)
	if fullIssue.SubmittedString != "" {
		fullIssue.Submitted = parseTime(fullIssue.SubmittedString)
		fullIssue.Committed = true
	}
	// Make patchset objects with the revision IDs and created timestamps.
	var patchsets patchsetSortable
	for id, r := range fullIssue.Revisions {
		patchsets = append(patchsets, &PatchSet{RevisionId: id, Created: parseTime(r.CreatedString)})
	}
	sort.Sort(patchsets)
	fullIssue.Patchsets = patchsets

	return fullIssue, nil
}

// setReview calls the Set Review endpoint of the Gerrit API to add messages and/or set labels for
// the latest patchset.
// API documentation: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
func (g *Gerrit) setReview(issue *ChangeInfo, message string, labels map[string]interface{}) error {
	postData := map[string]interface{}{
		"message": message,
		"labels":  labels,
	}
	latestPatchset := issue.Patchsets[len(issue.Patchsets)-1]
	return g.post(fmt.Sprintf("/a/changes/%s/revisions/%s/review", issue.ChangeId, latestPatchset.RevisionId), postData)
}

// AddComment adds a message to the issue.
func (g *Gerrit) AddComment(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{})
}

// Utility methods for interacting with the COMMITQUEUE_LABEL.

func (g *Gerrit) SendToDryRun(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: 1})
}

func (g *Gerrit) SendToCQ(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: 2})
}

func (g *Gerrit) RemoveFromCQ(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: 0})
}

// Utility methods for interacting with the CODEREVIEW_LABEL.

func (g *Gerrit) Approve(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: 1})
}

func (g *Gerrit) NoScore(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: 0})
}

func (g *Gerrit) DisApprove(issue *ChangeInfo, message string) error {
	return g.setReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: -1})
}

// Abandon abandons the issue with the given message.
func (g *Gerrit) Abandon(issue *ChangeInfo, message string) error {
	postData := map[string]interface{}{
		"message": message,
	}
	return g.post(fmt.Sprintf("/a/changes/%s/abandon", issue.ChangeId), postData)
}

func (g *Gerrit) get(suburl string, rv interface{}) error {
	resp, err := g.client.Get(g.url + suburl)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", g.url+suburl, err)
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("Not a valid Issue %s", g.url+suburl)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error retrieving %s: %d %s", g.url+suburl, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Could not read response body: %s", err)
	}

	// Strip off the XSS protection chars.
	parts := strings.SplitN(string(body), "\n", 2)

	if len(parts) != 2 {
		return fmt.Errorf("Reponse invalid format.")
	}
	if err := json.Unmarshal([]byte(parts[1]), &rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return nil
}

func (g *Gerrit) post(suburl string, postData interface{}) error {
	b, err := json.Marshal(postData)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", g.url+suburl, bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	u, err := url.Parse(g.url)
	if err != nil {
		return err
	}
	auth := ""
	for d, a := range g.cookies {
		if util.CookieDomainMatch(u.Host, d) {
			auth = a
			break
		}
	}
	if auth == "" {
		return errors.New("Cannot make authenticated post calls without a valid .gitcookies file")
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Got status %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

type changeListSortable []*ChangeInfo

func (p changeListSortable) Len() int           { return len(p) }
func (p changeListSortable) Less(i, j int) bool { return p[i].Created.Before(p[j].Created) }
func (p changeListSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type patchsetSortable []*PatchSet

func (p patchsetSortable) Len() int           { return len(p) }
func (p patchsetSortable) Less(i, j int) bool { return p[i].Created.Before(p[j].Created) }
func (p patchsetSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// SearchTerm is a wrapper for search terms to pass into the Search method.
type SearchTerm struct {
	Key   string
	Value string
}

// SearchOwner is a SearchTerm used for filtering by issue owner.
// API documentation is here: https://review.openstack.org/Documentation/user-search.html
func SearchOwner(name string) *SearchTerm {
	return &SearchTerm{
		Key:   "owner",
		Value: name,
	}
}

// SearchModifiedAfter is a SearchTerm used for finding issues modified after
// a particular time.Time.
// API documentation is here: https://review.openstack.org/Documentation/user-search.html
func SearchModifiedAfter(after time.Time) *SearchTerm {
	return &SearchTerm{
		Key:   "after",
		Value: "\"" + strings.Trim(strings.Split(after.UTC().String(), "+")[0], " ") + "\"",
	}
}

// queryString encodes query parameters in the key:val[+key:val...] format specified here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
func queryString(terms []*SearchTerm) string {
	q := []string{}
	for _, t := range terms {
		q = append(q, fmt.Sprintf("%s:%s", t.Key, t.Value))
	}
	return strings.Join(q, " ")
}

// Search returns a slice of Issues which fit the given criteria.
func (g *Gerrit) Search(limit int, terms ...*SearchTerm) ([]*ChangeInfo, error) {
	var issues changeListSortable
	for {
		data := make([]*ChangeInfo, 0)
		queryLimit := util.MinInt(limit-len(issues), MAX_GERRIT_LIMIT)
		skip := len(issues)

		q := url.Values{}
		q.Add("q", queryString(terms))
		q.Add("n", strconv.Itoa(queryLimit))
		q.Add("S", strconv.Itoa(skip))
		searchUrl := "/changes/?" + q.Encode()
		err := g.get(searchUrl, &data)
		if err != nil {
			return nil, fmt.Errorf("Gerrit search failed: %v", err)
		}
		var moreChanges bool

		for _, issue := range data {
			// See if there are more changes available.
			moreChanges = issue.MoreChanges
			// Save Created as a timestamp for sorting.
			issue.Created = parseTime(issue.CreatedString)
			issues = append(issues, issue)
		}
		if len(issues) >= limit || !moreChanges {
			break
		}
	}

	sort.Sort(issues)
	return issues, nil
}

func (g *Gerrit) GetTrybotResults(issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return buildbucket.NewClient(g.client).GetTrybotsForCL(issueID, patchsetID, "gerrit", g.url)
}

// CodeReviewCache is an LRU cache for Gerrit Issues that polls in the background to determine if
// issues have been updated. If so it expells them from the cache to force a reload.
type CodeReviewCache struct {
	cache     *lru.Cache
	gerritAPI *Gerrit
	timeDelta time.Duration
	mutex     sync.Mutex
}

// NewCodeReviewCache returns a new cache for the given API instance, poll interval and maximum cache size.
func NewCodeReviewCache(gerritAPI *Gerrit, pollInterval time.Duration, cacheSize int) *CodeReviewCache {
	ret := &CodeReviewCache{
		cache:     lru.New(cacheSize),
		gerritAPI: gerritAPI,
		timeDelta: pollInterval * 2,
	}

	// Start the poller.
	go util.Repeat(pollInterval, nil, ret.poll)
	return ret
}

// Add an issue to the cache.
func (c *CodeReviewCache) Add(key int64, value *ChangeInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	glog.Infof("\nAdding %d", key)
	c.cache.Add(key, value)
}

// Retrieve an issue from the cache.
func (c *CodeReviewCache) Get(key int64) (*ChangeInfo, bool) {
	glog.Infof("\nGetting: %d", key)
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if val, ok := c.cache.Get(key); ok {
		return val.(*ChangeInfo), true
	}
	return nil, false
}

// Poll Gerrit for all issues that have changed in the recent past.
func (c *CodeReviewCache) poll() {
	// Search for all keys that have changed in the last timeDelta duration.
	issues, err := c.gerritAPI.Search(10000, SearchModifiedAfter(time.Now().Add(-c.timeDelta)))
	if err != nil {
		glog.Errorf("Error polling Gerrit: %s", err)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, issue := range issues {
		glog.Infof("\nRemoving: %d", issue.Issue)
		c.cache.Remove(issue.Issue)
	}
}
