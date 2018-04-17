package iap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	GROUP_URI_TEMPLATE = "https://chrome-infra-auth.appspot.com/auth/api/v1/groups/%s"

	REFRESH_PERION = 15 * time.Minute
)

// Group is used in Response.
type Group struct {
	Members []string `json:"members"`
}

// Response represents the format returned from GROUP_URI_TEMPLATE.
type Response struct {
	Group Group `json:"group"`
}

// AllowedFromInfraAuth implements Allow by reading the list of emails and domains
// from the API endpoint.
//
// It implements Allow.
type AllowedFromInfraAuth struct {
	url     string
	client  *http.Client
	mutex   sync.Mutex
	allowed *AllowedFromList
}

// NewAllowedFromInfraAuth creates an AllowedFromInfraAuth.
//
// client - Must be authenticated and whitelisted to access GROUP_URI_TEMPLATE.
// group - The name of the group we want to restrict access to.
func NewAllowedFromInfraAuth(client *http.Client, group string) (*AllowedFromInfraAuth, error) {
	ret := &AllowedFromInfraAuth{
		url:    fmt.Sprintf(GROUP_URI_TEMPLATE, group),
		client: client,
	}
	if err := ret.reload(); err != nil {
		return nil, fmt.Errorf("Failed to initially load allowed list for group %q: %s", group, err)
	}
	go func() {
		for _ = range time.Tick(REFRESH_PERION) {
			if err := ret.reload(); err != nil {
				sklog.Errorf("Failed to reload allowed list for group %q: %s", group, err)
			}
		}
	}()
	return ret, nil
}

func infraAuthToAllowFromList(infra []string) []string {
	ret := []string{}
	for _, name := range infra {
		if name == "anonymous:anonymous" {
			continue
		} else if strings.HasPrefix(name, "user:") {
			name = name[5:]
			if strings.HasPrefix(name, "*@") {
				name = name[2:]
			}
			ret = append(ret, name)
		}
	}
	sklog.Infof("Got: %v", ret)
	return ret
}

func (a *AllowedFromInfraAuth) reload() error {
	resp, err := a.client.Get(a.url)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Non-OK status: %s", resp.Status)
	}
	defer util.Close(resp.Body)
	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Convert infra auth format to AllowFromList format.
	a.allowed = NewAllowedFromList(infraAuthToAllowFromList(r.Group.Members))
	return nil
}

func (a *AllowedFromInfraAuth) Member(email string) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.allowed.Member(email)
}
