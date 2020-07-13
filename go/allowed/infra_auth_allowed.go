package allowed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

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
	url     string
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
		url:    fmt.Sprintf(GROUP_URL_TEMPLATE, group),
		client: client,
	}
	if err := ret.reload(); err != nil {
		return nil, fmt.Errorf("Failed to initially load allowed list for group %q: %s", group, err)
	}
	go func() {
		for range time.Tick(REFRESH_PERIOD) {
			if err := ret.reload(); err != nil {
				sklog.Errorf("Failed to reload allowed list for group %q: %s", group, err)
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
		} else if strings.HasPrefix(name, "user:") {
			name = name[5:]
			name = strings.TrimPrefix(name, "*@")
			if name == "" {
				continue
			}
			ret = append(ret, name)
		}
	}
	sklog.Infof("Allowed list contains %d entries.", len(ret))
	return ret
}

func (a *AllowedFromChromeInfraAuth) reload() error {
	resp, err := a.client.Get(a.url)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("Non-OK status: %s", resp.Status)
	}
	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	sort.Strings(r.Group.Members)

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Convert infra auth format to AllowFromList format.
	a.allowed = NewAllowedFromList(infraAuthToAllowFromList(r.Group.Members))
	return nil
}

func (a *AllowedFromChromeInfraAuth) Member(email string) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.allowed.Member(email)
}

func (a *AllowedFromChromeInfraAuth) Emails() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.allowed.Emails()
}
