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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	ErrCookiesMissing = errors.New("Cannot make authenticated post calls without a valid .gitcookies file")
)

const (
	TIME_FORMAT         = "2006-01-02 15:04:05.999999"
	GERRIT_CHROMIUM_URL = "https://chromium-review.googlesource.com"
	GERRIT_SKIA_URL     = "https://skia-review.googlesource.com"
	MAX_GERRIT_LIMIT    = 500

	CHANGE_STATUS_ABANDONED = "ABANDONED"
	CHANGE_STATUS_DRAFT     = "DRAFT"
	CHANGE_STATUS_MERGED    = "MERGED"
	CHANGE_STATUS_NEW       = "NEW"

	CODEREVIEW_LABEL  = "Code-Review"
	COMMITQUEUE_LABEL = "Commit-Queue"

	CODEREVIEW_LABEL_DISAPPROVE = -1
	CODEREVIEW_LABEL_NONE       = 0
	CODEREVIEW_LABEL_APPROVE    = 1

	COMMITQUEUE_LABEL_NONE    = 0
	COMMITQUEUE_LABEL_DRY_RUN = 1
	COMMITQUEUE_LABEL_SUBMIT  = 2
)

// ChangeInfo contains information about a Gerrit issue.
type ChangeInfo struct {
	Created         time.Time              `json:"-"`
	CreatedString   string                 `json:"created"`
	Updated         time.Time              `json:"-"`
	UpdatedString   string                 `json:"updated"`
	Submitted       time.Time              `json:"-"`
	SubmittedString string                 `json:"submitted"`
	Project         string                 `json:"project"`
	ChangeId        string                 `json:"change_id"`
	Subject         string                 `json:"subject"`
	Branch          string                 `json:"branch"`
	Committed       bool                   `json:"committed"`
	Revisions       map[string]*Revision   `json:"revisions"`
	Patchsets       []*Revision            `json:"-"`
	MoreChanges     bool                   `json:"_more_changes"`
	Issue           int64                  `json:"_number"`
	Labels          map[string]*LabelEntry `json:"labels"`
	Owner           *Owner                 `json:"owner"`
	Status          string                 `json:"status"`
}

// IsClosed returns true iff the issue corresponding to the ChangeInfo is
// abandoned or merged.
func (c ChangeInfo) IsClosed() bool {
	return (c.Status == CHANGE_STATUS_ABANDONED ||
		c.Status == CHANGE_STATUS_MERGED)
}

// Owner gathers the owner information of a ChangeInfo instance. Some fields ommitted.
type Owner struct {
	Email string `json:"email"`
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

// Revision is the information associated with a patchset in Gerrit.
type Revision struct {
	ID            string    `json:"-"`
	Number        int64     `json:"_number"`
	CreatedString string    `json:"created"`
	Created       time.Time `json:"-"`
}

// Gerrit is an object used for iteracting with the issue tracker.
type Gerrit struct {
	client               *http.Client
	buildbucketClient    *buildbucket.Client
	url                  string
	cookies              map[string]string
	useAuthenticatedGets bool
}

// NewGerrit returns a new Gerrit instance. If gitCookiesPath is empty the
// instance will be in read-only mode and only return information available to
// anonymous users.
func NewGerrit(url, gitCookiesPath string, client *http.Client) (*Gerrit, error) {
	url = strings.TrimRight(url, "/")
	cookies, err := getCredentials(gitCookiesPath)
	if err != nil {
		return nil, err
	}

	if client == nil {
		client = httputils.NewTimeoutClient()
	}

	return &Gerrit{
		url:               url,
		client:            client,
		buildbucketClient: buildbucket.NewClient(client),
		cookies:           cookies,
	}, nil
}

// DefaultGitCookiesPath returns the default cookie file. The return value
// can be used as the input to NewGerrit. If it cannot be retrieved an
// error will be logged and the empty string is returned.
func DefaultGitCookiesPath() string {
	usr, err := user.Current()
	if err != nil {
		sklog.Errorf("Unable to retrieve default git cookies path")
		return ""
	}
	return filepath.Join(usr.HomeDir, ".gitcookies")
}

// getCredentials returns the parsed contents of .gitCookies.
// This logic has been borrowed from
// https://cs.chromium.org/chromium/tools/depot_tools/gerrit_util.py?l=143
func getCredentials(gitCookiesPath string) (map[string]string, error) {
	// Set empty cookies if no path was given and issue a warning.
	if gitCookiesPath == "" {
		sklog.Infof("Gerrit client initialized in read-only mode. ")
		return map[string]string{}, nil
	}

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
			gitCookies[domain] = value
		}
	}
	return gitCookies, nil
}

func parseTime(t string) time.Time {
	parsed, _ := time.Parse(TIME_FORMAT, t)
	return parsed
}

// TurnOnAuthenticatedGets makes all GET requests contain authentication
// cookies. By default only POST requests are automatically authenticated.
func (g *Gerrit) TurnOnAuthenticatedGets() {
	g.useAuthenticatedGets = true
}

// Url returns the url of the Gerrit issue identified by issueID or the
// base URL of the Gerrit instance if issueID is 0.
func (g *Gerrit) Url(issueID int64) string {
	if issueID == 0 {
		return g.url
	}
	return fmt.Sprintf("%s/c/%d", g.url, issueID)
}

// extractReg is the regular expression used by ExtractIssue.
var extractReg = regexp.MustCompile("^/c/([0-9]+)$")

// ExtractIssue returns the issue id as a string given the issue URL.
// The second return value is true if the issueURL matches the current Gerrit
// instance. If it is false the first return value should be ignored.
func (g *Gerrit) ExtractIssue(issueURL string) (string, bool) {
	if !strings.HasPrefix(issueURL, g.url) {
		return "", false
	}

	match := extractReg.FindStringSubmatch(strings.TrimRight(issueURL[len(g.url):], "/"))
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
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
	patchsets := make([]*Revision, 0, len(fullIssue.Revisions))
	for id, r := range fullIssue.Revisions {
		// Fill in the missing fields.
		r.ID = id
		r.Created = parseTime(r.CreatedString)
		patchsets = append(patchsets, r)
	}
	sort.Sort(revisionSlice(patchsets))
	fullIssue.Patchsets = patchsets

	return fullIssue, nil
}

// GetPatchsetIDs is a convenience function that returns the sorted list of patchset IDs.
func (c *ChangeInfo) GetPatchsetIDs() []int64 {
	ret := make([]int64, len(c.Patchsets))
	for idx, patchSet := range c.Patchsets {
		ret[idx] = patchSet.Number
	}
	return ret
}

// GetPatch returns the formatted patch for one revision. Documentation is here:
// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#get-patch
func (g *Gerrit) GetPatch(issue int64, revision string) (string, error) {
	url := fmt.Sprintf("%s/changes/%d/revisions/%s/patch", g.url, issue, revision)
	resp, err := g.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %s: %s", url, err)
	}
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("Not a valid Issue %s", url)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Error retrieving %s: %d %s", url, resp.StatusCode, resp.Status)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Could not read response body: %s", err)
	}

	data, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return "", fmt.Errorf("Could not base64 decode response body: %s", err)
	}
	// Extract out only the patch.
	tokens := strings.SplitN(string(data), "---", 2)
	if len(tokens) != 2 {
		return "", fmt.Errorf("Gerrit patch response was invalid: %s", string(data))
	}
	patch := tokens[1]
	return patch, nil
}

// setReview calls the Set Review endpoint of the Gerrit API to add messages and/or set labels for
// the latest patchset.
// API documentation: https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-review
func (g *Gerrit) SetReview(issue *ChangeInfo, message string, labels map[string]interface{}) error {
	postData := map[string]interface{}{
		"message": message,
		"labels":  labels,
	}
	latestPatchset := issue.Patchsets[len(issue.Patchsets)-1]
	return g.post(fmt.Sprintf("/a/changes/%s/revisions/%s/review", issue.ChangeId, latestPatchset.ID), postData)
}

// AddComment adds a message to the issue.
func (g *Gerrit) AddComment(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{})
}

// Utility methods for interacting with the COMMITQUEUE_LABEL.

func (g *Gerrit) SendToDryRun(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN})
}

func (g *Gerrit) SendToCQ(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT})
}

func (g *Gerrit) RemoveFromCQ(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_NONE})
}

// Utility methods for interacting with the CODEREVIEW_LABEL.

func (g *Gerrit) Approve(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_APPROVE})
}

func (g *Gerrit) NoScore(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_NONE})
}

func (g *Gerrit) DisApprove(issue *ChangeInfo, message string) error {
	return g.SetReview(issue, message, map[string]interface{}{CODEREVIEW_LABEL: CODEREVIEW_LABEL_DISAPPROVE})
}

// Abandon abandons the issue with the given message.
func (g *Gerrit) Abandon(issue *ChangeInfo, message string) error {
	postData := map[string]interface{}{
		"message": message,
	}
	return g.post(fmt.Sprintf("/a/changes/%s/abandon", issue.ChangeId), postData)
}

func (g *Gerrit) addAuthenticationCookie(req *http.Request) error {
	u, err := url.Parse(g.url)
	if err != nil {
		return err
	}

	auth := ""
	for d, a := range g.cookies {
		if util.CookieDomainMatch(u.Host, d) {
			auth = a
			cookie := http.Cookie{Name: "o", Value: a}
			req.AddCookie(&cookie)
			break
		}
	}
	if auth == "" {
		return ErrCookiesMissing
	}
	return nil
}

func (g *Gerrit) get(suburl string, rv interface{}) error {
	getURL := g.url + suburl
	if g.useAuthenticatedGets {
		getURL = g.url + "/a" + suburl
	}
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}

	if g.useAuthenticatedGets {
		if err := g.addAuthenticationCookie(req); err != nil {
			return err
		}
	}

	resp, err := g.client.Do(req)
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

	if err := g.addAuthenticationCookie(req); err != nil {
		return err
	}
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

type revisionSlice []*Revision

func (r revisionSlice) Len() int           { return len(r) }
func (r revisionSlice) Less(i, j int) bool { return r[i].Created.Before(r[j].Created) }
func (r revisionSlice) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }

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

func SearchCommit(commit string) *SearchTerm {
	return &SearchTerm{
		Key:   "commit",
		Value: commit,
	}
}

func SearchStatus(status string) *SearchTerm {
	return &SearchTerm{
		Key:   "status",
		Value: status,
	}
}

func SearchProject(project string) *SearchTerm {
	return &SearchTerm{
		Key:   "project",
		Value: project,
	}
}

func SearchLabel(label, value string) *SearchTerm {
	return &SearchTerm{
		Key:   "label",
		Value: fmt.Sprintf("%s=%s", label, value),
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
	return g.buildbucketClient.GetTrybotsForCL(issueID, patchsetID, "gerrit", g.url)
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
	sklog.Infof("\nAdding %d", key)
	c.cache.Add(key, value)
}

// Retrieve an issue from the cache.
func (c *CodeReviewCache) Get(key int64) (*ChangeInfo, bool) {
	sklog.Infof("\nGetting: %d", key)
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
		sklog.Errorf("Error polling Gerrit: %s", err)
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, issue := range issues {
		sklog.Infof("\nRemoving: %d", issue.Issue)
		c.cache.Remove(issue.Issue)
	}
}

// ContainsAny returns true if the provided ChangeInfo slice contains any
// change with the same issueID as id.
func ContainsAny(id int64, changes []*ChangeInfo) bool {
	for _, c := range changes {
		if id == c.Issue {
			return true
		}
	}
	return false
}
