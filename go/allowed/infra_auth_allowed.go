package allowed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// GROUP_URL_TEMPLATE is the URL to retrieve the group membership from Chrome Infra Auth server.
	GROUP_URL_TEMPLATE = "https://chrome-infra-auth.appspot.com/auth/api/v1/groups/%s"

	// REFRESH_PERIOD How often to refresh the group membership.
	REFRESH_PERIOD = 15 * time.Minute
)

// Group is used in Response.
type Group struct {
	Members []string `json:"members"`
	Nested  []string `json:"nested"`
	Globs   []string `json:"globs"`
}

// Response represents the format returned from GROUP_URL_TEMPLATE.
type Response struct {
	Group Group `json:"group"`
}

// AllowedFromChromeInfraAuth implements Allow by reading the list of emails and domains
// from the Chrome Infra Auth API endpoint.
//
// It implements Allow.
type AllowedFromChromeInfraAuth struct {
	group   string
	client  *http.Client
	mutex   sync.RWMutex
	allowed *AllowedFromList
}

// NewAllowedFromChromeInfraAuth creates an AllowedFromChromeInfraAuth.
//
// client - Must be authenticated and allowed to access GROUP_URL_TEMPLATE.
// group - The name of the group we want to restrict access to.
//
// The presumption is that an AllowedFromChromeInfraAuth will be created
// at startup and if creation fails then the application will
// not start.
func NewAllowedFromChromeInfraAuth(client *http.Client, group string) (*AllowedFromChromeInfraAuth, error) {
	ret := &AllowedFromChromeInfraAuth{
		group:  group,
		client: client,
	}
	if err := ret.reload(); err != nil {
		return nil, fmt.Errorf("Failed to initially load allowed list for group %q: %s", group, err)
	}
	go func() {
		failedMetric := metrics2.GetCounter("cria_refresh_failed")
		for range time.Tick(REFRESH_PERIOD) {
			if err := ret.reload(); err != nil {
				failedMetric.Inc(1)
				sklog.Errorf("Failed to reload allowed list for group %q: %s", group, err)
			} else {
				failedMetric.Reset()
			}
		}
	}()
	return ret, nil
}

// infraAuthToAllowFromList converts from Chrome Infra Auth format
// to the format the AllowFromList expects.
//
// Note that iap doesn't support 'anonymous:anonymous' access, so
// that gets ignored.
func infraAuthToAllowFromList(infra []string) []string {
	ret := []string{}
	for _, name := range infra {
		if name == "anonymous:anonymous" {
			continue
		}
		if strings.HasPrefix(name, "bot:") || strings.HasPrefix(name, "service:") {
			continue
		}
		name = strings.TrimPrefix(name, "user:")
		name = strings.TrimPrefix(name, "*@")
		if name == "" {
			continue
		}
		ret = append(ret, name)
	}
	sklog.Infof("Allowed list contains %d entries.", len(ret))
	return ret
}

func (a *AllowedFromChromeInfraAuth) getMembers(group string) ([]string, error) {
	url := fmt.Sprintf(GROUP_URL_TEMPLATE, group)
	sklog.Debugf("CrIA: Fetching group membership for group %q from endpoint %s", group, url)
	resp, err := a.client.Get(url)
	if err != nil {
		sklog.Errorf("CrIA: Error fetching group %q: %v", group, err)
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Non-OK status from CrIA endpoint for group %q: %s", group, resp.Status)
	}
	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	members := r.Group.Members
	sklog.Debugf("CrIA: Group %q fetched: %d direct members, %d nested groups %v, %d globs", group, len(members), len(r.Group.Nested), r.Group.Nested, len(r.Group.Globs))
	// Get all members from nested groups.
	for _, nestedGroup := range r.Group.Nested {
		indirectMembers, err := a.getMembers(nestedGroup)
		if err != nil {
			return nil, err
		}
		members = append(members, indirectMembers...)
	}
	// Get all globs.
	for _, glob := range r.Group.Globs {
		members = append(members, glob)
	}
	return members, nil
}

func (a *AllowedFromChromeInfraAuth) reload() error {
	sklog.Debugf("CrIA: Reloading allowed list for group %q...", a.group)
	members, err := a.getMembers(a.group)
	if err != nil {
		return err
	}
	sort.Strings(members)
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Convert infra auth format to AllowFromList format.
	allowedList := infraAuthToAllowFromList(members)
	a.allowed = NewAllowedFromList(allowedList)
	sklog.Infof("CrIA: Reload complete for group %q with %d final member entries.", a.group, len(allowedList))
	return nil
}

func (a *AllowedFromChromeInfraAuth) Member(email string) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	isMember := a.allowed.Member(email)
	sklog.Debugf("CrIA: Evaluating group %q for email %q -> member=%t", a.group, email, isMember)
	return isMember
}

func (a *AllowedFromChromeInfraAuth) Emails() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.allowed.Emails()
}
