// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
)

// ServiceURL is URL of a service to talk to by default.
const ServiceURL string = "https://chrome-infra-auth.appspot.com"

// IdentityKind is enum like type with possible kinds of identities.
type IdentityKind string

const (
	// IdentityKindUnknown is used when server return identity kind not recognized
	// by the client.
	IdentityKindUnknown IdentityKind = ""
	// IdentityKindAnonymous is used to represent anonymous callers.
	IdentityKindAnonymous IdentityKind = "anonymous"
	// IdentityKindBot is used to represent bots. Identity name is bot's IP.
	IdentityKindBot IdentityKind = "bot"
	// IdentityKindService is used to represent AppEngine apps when they use
	// X-Appengine-Inbound-AppId header to authenticate. Identity name is app ID.
	IdentityKindService IdentityKind = "service"
	// IdentityKindUser is used to represent end users or OAuth service accounts.
	// Identity name is an associated email (user email or service account email).
	IdentityKindUser IdentityKind = "user"
)

var (
	// ErrAccessDenied is returned by GroupsService methods if caller doesn't have
	// enough permissions to execute an action. Corresponds to 403 HTTP status.
	ErrAccessDenied = errors.New("access denied (HTTP 403)")

	// ErrNoSuchItem is returned by GroupsService methods if requested item
	// (e.g. a group) doesn't exist. Corresponds to 404 HTTP status.
	ErrNoSuchItem = errors.New("no such item (HTTP 404)")
)

// Identity represents some caller that can make requests. It generalizes
// accounts of real people, bot accounts and service-to-service accounts.
type Identity struct {
	// Kind describes what sort of identity this struct represents.
	Kind IdentityKind
	// Name defines concrete instance of identity, its meaning depends on kind.
	Name string
}

// Group is a named list of identities, included subgroups and glob-like
// patterns (e.g. "user:*@example.com).
type Group struct {
	// Name is a name of the group.
	Name string
	// Description is a human readable description of the group.
	Description string
	// Members is a list of identities included in the group explicitly.
	Members []Identity
	// Globs is a list of glob-like patterns for identities.
	Globs []string
	// Nested is a list of group names included into this group.
	Nested []string
}

// String returns human readable representation of Identity.
func (i Identity) String() string {
	switch i.Kind {
	case IdentityKindUnknown:
		return "unknown"
	case IdentityKindAnonymous:
		return "anonymous"
	case IdentityKindUser:
		return i.Name
	default:
		return fmt.Sprintf("%s:%s", i.Kind, i.Name)
	}
}

// GroupsService knows how to talk to Groups API backend.
//
// Server side code is in https://github.com/luci/luci-py repository.
type GroupsService struct {
	client     *http.Client
	serviceURL string

	// doRequest is mocked in tests. Default is ctxhttp.Do.
	doRequest func(context.Context, *http.Client, *http.Request) (*http.Response, error)
}

// NewGroupsService constructs new instance of GroupsService.
//
// It that talks to given service URL via the given http.Client. If url is empty
// string, the default backend will be used. If httpClient is nil,
// http.DefaultClient will be used.
func NewGroupsService(url string, client *http.Client) *GroupsService {
	if url == "" {
		url = ServiceURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &GroupsService{
		client:     client,
		serviceURL: url,
		doRequest:  ctxhttp.Do,
	}
}

// ServiceURL returns a string with root URL of a Groups backend.
func (s *GroupsService) ServiceURL() string {
	return s.serviceURL
}

// FetchCallerIdentity returns caller's own Identity as seen by the server.
func (s *GroupsService) FetchCallerIdentity(ctx context.Context) (Identity, error) {
	var response struct {
		Identity string `json:"identity"`
	}
	err := s.doGet(ctx, "/auth/api/v1/accounts/self", &response)
	if err != nil {
		return Identity{}, err
	}
	return parseIdentity(response.Identity)
}

// FetchGroup returns a group definition.
//
// It does not fetch nested groups. Returns ErrNoSuchItem error if no such
// group.
func (s *GroupsService) FetchGroup(ctx context.Context, name string) (Group, error) {
	var response struct {
		Group struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Members     []string `json:"members"`
			Globs       []string `json:"globs"`
			Nested      []string `json:"nested"`
		} `json:"group"`
	}
	err := s.doGet(ctx, "/auth/api/v1/groups/"+name, &response)
	if err != nil {
		return Group{}, err
	}
	if response.Group.Name != name {
		return Group{}, fmt.Errorf(
			"unexpected group name in server response: %q, expecting %q",
			response.Group.Name, name)
	}
	g := Group{
		Name:        response.Group.Name,
		Description: response.Group.Description,
		Members:     make([]Identity, 0, len(response.Group.Members)),
		Globs:       response.Group.Globs,
		Nested:      response.Group.Nested,
	}
	for _, str := range response.Group.Members {
		ident, err := parseIdentity(str)
		if err != nil {
			logging.Warningf(ctx, "auth: failed to parse an identity in a group, ignoring it - %s", err)
		} else {
			g.Members = append(g.Members, ident)
		}
	}
	return g, nil
}

func (s *GroupsService) retryIterator() retry.Iterator {
	return &retry.ExponentialBackoff{
		Limited: retry.Limited{
			Delay:    10 * time.Millisecond,
			Retries:  50,
			MaxTotal: 10 * time.Second,
		},
		Multiplier: 1.5,
	}
}

func (s *GroupsService) doGet(ctx context.Context, path string, response interface{}) error {
	if len(path) == 0 || path[0] != '/' {
		return fmt.Errorf("path should start with '/': %s", path)
	}
	url := s.serviceURL + path

	var responseBody []byte

	err := retry.Retry(ctx, retry.TransientOnly(s.retryIterator), func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		resp, err := s.doRequest(ctx, s.client, req)
		if err != nil {
			return errors.WrapTransient(err) // connection error, transient
		}
		defer resp.Body.Close()

		// Success?
		if resp.StatusCode < 300 {
			responseBody, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return errors.WrapTransient(err) // connection aborted midway
			}
			return nil
		}

		// Fatal error?
		if resp.StatusCode >= 300 && resp.StatusCode < 500 {
			switch resp.StatusCode {
			case http.StatusForbidden:
				return ErrAccessDenied
			case http.StatusNotFound:
				return ErrNoSuchItem
			default:
				body, _ := ioutil.ReadAll(resp.Body)
				return fmt.Errorf("unexpected reply (HTTP %d):\n%s", resp.StatusCode, string(body))
			}
		}

		// Transient error on the backend.
		return errors.WrapTransient(
			fmt.Errorf("transient error on the backend (HTTP %d)", resp.StatusCode))
	}, nil)
	if err != nil {
		return err
	}

	return json.Unmarshal(responseBody, response)
}

// parseIdentity takes a string of form "<kind>:<name>" and returns Identity
// struct.
func parseIdentity(str string) (Identity, error) {
	chunks := strings.Split(str, ":")
	if len(chunks) != 2 {
		return Identity{}, fmt.Errorf("invalid identity string: %q", str)
	}
	kind := IdentityKind(chunks[0])
	name := chunks[1]
	switch kind {
	case IdentityKindAnonymous:
		if name != "anonymous" {
			return Identity{}, fmt.Errorf("invalid anonymous identity: %q", str)
		}
		return Identity{
			Kind: IdentityKindAnonymous,
			Name: "anonymous",
		}, nil
	case IdentityKindBot, IdentityKindService, IdentityKindUser:
		return Identity{
			Kind: kind,
			Name: name,
		}, nil
	default:
		return Identity{}, fmt.Errorf("unrecognized identity kind: %q", str)
	}
}
