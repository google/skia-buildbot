package rietveld

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/util"
)

const (
	CLIENT_ID     = "446450136466-2hr92jrq8e6i4tnsa56b52vacp7t3936.apps.googleusercontent.com"
	CLIENT_SECRET = "uBfbay2KCy9t4QveJ-dOqHtp"

	COMMITTED_ISSUE_REGEXP = "(?m:^Committed: .+$)"
	CQ_STATUS_URL          = "https://chromium-cq-status.appspot.com/v2/patch-summary/codereview.chromium.org/%d/%d"

	TIME_FORMAT = "2006-01-02 15:04:05.999999"

	RIETVELD_SKIA_URL = "https://codereview.chromium.org"
)

var (
	OAUTH_SCOPES = []string{
		"https://www.googleapis.com/auth/userinfo.email",
	}
)

// Issue contains information about a Rietveld issue.
type Issue struct {
	CC                []string
	Closed            bool
	Committed         bool
	CommitQueue       bool `json:"commit"`
	CommitQueueDryRun bool `json:"cq_dry_run"`
	Created           time.Time
	CreatedString     string `json:"created"`
	Description       string
	Issue             int64
	Messages          []IssueMessage
	Modified          time.Time
	ModifiedString    string `json:"modified"`
	Owner             string
	Project           string
	Reviewers         []string
	Subject           string
	Patchsets         []int64
}

// IssueMessage contains information about a comment on an issue.
type IssueMessage struct {
	Date       time.Time
	DateString string `json:"date"`
	Sender     string
	Text       string
}

// Rietveld is an object used for interacting with the issue tracker.
type Rietveld struct {
	client        *http.Client
	url           string
	xsrfToken     string
	xsrfTokenTime time.Time
}

// New returns a new Rietveld instance. If client is nil, the default
// http.Client will be used for anonymous access. In this case, some
// functionality will be unavailable, eg. modifying issues.
func New(url string, client *http.Client) *Rietveld {
	url = strings.TrimRight(url, "/")
	if client == nil {
		client = http.DefaultClient
	}
	return &Rietveld{
		url:    url,
		client: client,
	}
}

// Url returns the URL of the server for this Rietveld instance.
func (r *Rietveld) Url() string {
	return r.url
}

// Patchset contains the information about one patchset. Currently we omit
// fields that we don't need.
type Patchset struct {
	Patchset      int64           `json:"patchset"`
	Issue         int64           `json:"issue"`
	Owner         string          `json:"owner"`
	OwnerEmail    string          `json:"owner_email"`
	Created       time.Time       `json:"-"`
	CreatedStr    string          `json:"created"`
	Modified      time.Time       `json:"-"`
	ModifiedStr   string          `json:"modified"`
	TryjobResults []*TryjobResult `json:"try_job_results"`
}

// TryjobResult contains the trybots that have been scheduled in Rietveld. We ommit
// fields we are currently not interested in.
type TryjobResult struct {
	Master      string `json:"master"`
	Builder     string `json:"builder"`
	BuildNumber int64  `json:"buildnumber"`
	Result      int64  `json:"result"`
}

func parseTime(t string) time.Time {
	parsed, _ := time.Parse(TIME_FORMAT, t)
	return parsed
}

// isCommitted returns true iff the issue has been committed.
func (r *Rietveld) isCommitted(i *Issue) (bool, error) {
	committed, err := regexp.MatchString(COMMITTED_ISSUE_REGEXP, i.Description)
	if err != nil {
		return false, err
	}
	if committed {
		return true, nil
	}

	// The description sometimes doesn't get updated in time. Check the
	// commit queue status for its result.
	url := fmt.Sprintf(CQ_STATUS_URL, i.Issue, i.Patchsets[len(i.Patchsets)-1])
	resp, err := r.client.Get(url)
	if err != nil {
		return false, fmt.Errorf("Failed to GET %s: %s", url, err)
	}
	defer util.Close(resp.Body)
	dec := json.NewDecoder(resp.Body)
	var rv struct {
		Success bool `json:"success"`
	}
	if err := dec.Decode(&rv); err != nil {
		return false, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return rv.Success, nil
}

// getIssueProperties returns a fully filled-in Issue object, as opposed to
// the partial data returned by Rietveld's search endpoint.
func (r *Rietveld) GetIssueProperties(issue int64, messages bool) (*Issue, error) {
	url := fmt.Sprintf("/api/%v", issue)
	if messages {
		url += "?messages=true"
	}
	fullIssue := &Issue{}
	if err := r.get(url, fullIssue); err != nil {
		return nil, fmt.Errorf("Failed to load details for issue %d: %v", issue, err)
	}

	committed, err := r.isCommitted(fullIssue)
	if err != nil {
		return nil, err
	}
	fullIssue.Committed = committed

	fullIssue.Created = parseTime(fullIssue.CreatedString)
	fullIssue.Modified = parseTime(fullIssue.ModifiedString)
	if messages {
		for _, msg := range fullIssue.Messages {
			msg.Date = parseTime(msg.DateString)
		}
	}
	return fullIssue, nil
}

// AddComment adds a comment to the given CL.
func (r *Rietveld) AddComment(issue int64, message string) error {
	data := url.Values{}
	data.Add("message", message)
	data.Add("message_only", "True")
	data.Add("add_as_reviewer", "False")
	data.Add("send_mail", "True")
	data.Add("no_redirect", "True")
	return r.post(fmt.Sprintf("/%d/publish", issue), data)
}

// SetProperties sets the given properties on the issue with the given value.
func (r *Rietveld) SetProperties(issue, lastPatchset int64, props map[string]string) error {
	data := url.Values{}
	for k, v := range props {
		data.Add(k, v)
	}
	data.Add("last_patchset", fmt.Sprintf("%d", lastPatchset))
	return r.post(fmt.Sprintf("/%d/edit_flags", issue), data)
}

// Close closes the issue with the given message.
func (r *Rietveld) Close(issue int64, message string) error {
	if err := r.AddComment(issue, message); err != nil {
		return err
	}
	return r.post(fmt.Sprintf("/%d/close", issue), nil)
}

func (r *Rietveld) refreshXSRFToken() error {
	req, err := http.NewRequest("GET", r.url+"/xsrf_token", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Requesting-XSRF-Token", "1")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	r.xsrfToken = string(data)
	r.xsrfTokenTime = time.Now()
	return nil
}

func (r *Rietveld) refreshXSRFTokenIfNeeded() error {
	if time.Now().Sub(r.xsrfTokenTime) > 30*time.Minute {
		return r.refreshXSRFToken()
	}
	return nil
}

func (r *Rietveld) get(suburl string, rv interface{}) error {
	resp, err := r.client.Get(r.url + suburl)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %s", r.url+suburl, err)
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("Not a valid Issue %s: %s", r.url+suburl, err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Error retrieving %s: %d %s", r.url+suburl, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return nil
}

func (r *Rietveld) post(suburl string, data url.Values) error {
	if err := r.refreshXSRFTokenIfNeeded(); err != nil {
		return err
	}
	if data == nil {
		data = url.Values{}
	}
	data.Add("xsrf_token", r.xsrfToken)
	resp, err := r.client.PostForm(r.url+suburl, data)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Got status %s (%d)", resp.Status, resp.StatusCode)
	}
	return nil
}

type issueListSortable []*Issue

func (p issueListSortable) Len() int           { return len(p) }
func (p issueListSortable) Less(i, j int) bool { return p[i].Created.Before(p[j].Created) }
func (p issueListSortable) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// SearchTerm is a wrapper for search terms to pass into the Search method.
type SearchTerm struct {
	Key   string
	Value string
}

// SearchOwner is a SearchTerm used for filtering by issue owner.
func SearchOwner(name string) *SearchTerm {
	return &SearchTerm{
		Key:   "owner",
		Value: name,
	}
}

// SearchModifiedAfter is a SearchTerm used for finding issues modified after
// a particular time.Time.
func SearchModifiedAfter(after time.Time) *SearchTerm {
	return &SearchTerm{
		Key:   "modified_after",
		Value: url.QueryEscape(strings.Trim(strings.Split(after.UTC().String(), "+")[0], " ")),
	}
}

// SearchOpen is a SearchTerm used for filtering issues by open/closed status.
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

// Search returns a slice of Issues which fit the given criteria.
func (r *Rietveld) Search(limit int, terms ...*SearchTerm) ([]*Issue, error) {
	searchUrl := fmt.Sprintf("/search?format=json&limit=%d", limit)
	for _, term := range terms {
		searchUrl += fmt.Sprintf("&%s=%s", term.Key, term.Value)
	}

	var issues issueListSortable
	cursor := ""
	for {
		var data struct {
			Results []*Issue
			Cursor  string
		}
		err := r.get(searchUrl+cursor, &data)
		if err != nil {
			return nil, fmt.Errorf("Rietveld search failed: %v", err)
		}
		if len(data.Results) == 0 {
			break
		}
		for _, issue := range data.Results {
			fullIssue, err := r.GetIssueProperties(issue.Issue, false)
			if err != nil {
				return nil, err
			} else {
				issues = append(issues, fullIssue)
			}
		}
		if len(issues) >= limit {
			break
		}
		cursor = "&cursor=" + data.Cursor
	}
	sort.Sort(issues)
	return issues, nil
}

// SearchKeys returns the issue ids that meet the given search terms.
func (r *Rietveld) SearchKeys(limit int, terms ...*SearchTerm) ([]int64, error) {
	// 1000 is the maximum number Rietveld will accept. If we want more than that,
	// we will do multiple requests with the maximum query limit.
	queryLimit := util.MinInt(limit, 1000)
	searchUrl := fmt.Sprintf("/search?format=json&keys_only=true&limit=%d", queryLimit)
	for _, term := range terms {
		searchUrl += fmt.Sprintf("&%s=%s", term.Key, term.Value)
	}

	cursor := ""
	ret := []int64{}
	for {
		var data struct {
			Results []int64
			Cursor  string
		}
		err := r.get(searchUrl+cursor, &data)
		if err != nil {
			return nil, fmt.Errorf("Rietveld search failed: %v", err)
		}
		ret = append(ret, data.Results...)
		if (len(data.Results) < queryLimit) || (len(ret) >= limit) {
			break
		}
		cursor = "&cursor=" + data.Cursor
	}

	// There is a very small change we have more than we asked for.
	if len(ret) > limit {
		ret = ret[0:limit]
	}

	return ret, nil
}

// GetPatchset returns information about the given patchset.
func (r *Rietveld) GetPatchset(issueID int64, patchsetID int64) (*Patchset, error) {
	url := fmt.Sprintf("/api/%d/%d", issueID, patchsetID)
	patchset := &Patchset{}
	if err := r.get(url, patchset); err != nil {
		return nil, err
	}

	patchset.Created = parseTime(patchset.CreatedStr)
	patchset.Modified = parseTime(patchset.ModifiedStr)
	return patchset, nil
}

// GetTrybotResults returns trybot results for the given Issue and Patchset.
func (r *Rietveld) GetTrybotResults(issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return buildbucket.NewClient(r.client).GetTrybotsForCL(issueID, patchsetID)
}

// CodeReviewCache is an LRU cache for Rietveld Issues that polls in the background to determine if
// issues have been updated. If so it expells them from the cache to force a reload.
type CodeReviewCache struct {
	cache       *lru.Cache
	rietveldAPI *Rietveld
	timeDelta   time.Duration
	mutex       sync.Mutex
}

// NewCodeReviewCache returns a new chache for the given API instance, poll interval and maximum cache size.
func NewCodeReviewCache(rietveldAPI *Rietveld, pollInterval time.Duration, cacheSize int) *CodeReviewCache {
	ret := &CodeReviewCache{
		cache:       lru.New(cacheSize),
		rietveldAPI: rietveldAPI,
		timeDelta:   pollInterval * 2,
	}

	// Start the poller.
	go util.Repeat(pollInterval, nil, ret.poll)
	return ret
}

// Add an issue to the cache.
func (c *CodeReviewCache) Add(key int64, value *Issue) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache.Add(key, value)
}

// Retrieve an issue from the cache.
func (c *CodeReviewCache) Get(key int64) (*Issue, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if val, ok := c.cache.Get(key); ok {
		return val.(*Issue), true
	}
	return nil, false
}

// Poll rietveld for all issues that have changed in the recent past.
func (c *CodeReviewCache) poll() {
	// Search for all keys that ahve changed in the last
	keys, err := c.rietveldAPI.SearchKeys(10000, SearchModifiedAfter(time.Now().Add(-c.timeDelta)))
	if err != nil {
		glog.Errorf("Error polling Rietveld: %s", err)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, key := range keys {
		c.cache.Remove(key)
	}
}
