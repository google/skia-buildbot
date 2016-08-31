// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"errors"
	"net"
	"net/http"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/secrets"

	"github.com/luci/luci-go/server/auth/authdb"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/service/protocol"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAuthenticate(t *testing.T) {
	Convey("IsAllowedOAuthClientID on default DB", t, func() {
		c := context.Background()
		auth := Authenticator{fakeOAuthMethod{clientID: "some_client_id"}}
		_, err := auth.Authenticate(c, makeRequest())
		So(err, ShouldErrLike, "the library is not properly configured")
	})

	Convey("IsAllowedOAuthClientID with valid client_id", t, func() {
		c := injectTestDB(context.Background(), &fakeDB{
			allowedClientID: "some_client_id",
		})
		auth := Authenticator{fakeOAuthMethod{clientID: "some_client_id"}}
		_, err := auth.Authenticate(c, makeRequest())
		So(err, ShouldBeNil)
	})

	Convey("IsAllowedOAuthClientID with invalid client_id", t, func() {
		c := injectTestDB(context.Background(), &fakeDB{
			allowedClientID: "some_client_id",
		})
		auth := Authenticator{fakeOAuthMethod{clientID: "another_client_id"}}
		_, err := auth.Authenticate(c, makeRequest())
		So(err, ShouldEqual, ErrBadClientID)
	})

	Convey("IP whitelist restriction works", t, func() {
		db, err := authdb.NewSnapshotDB(&protocol.AuthDB{
			IpWhitelistAssignments: []*protocol.AuthIPWhitelistAssignment{
				{
					Identity:    strPtr("user:abc@example.com"),
					IpWhitelist: strPtr("whitelist"),
				},
			},
			IpWhitelists: []*protocol.AuthIPWhitelist{
				{
					Name: strPtr("whitelist"),
					Subnets: []string{
						"1.2.3.4/32",
					},
				},
			},
		}, "http://auth-service", 1234)
		So(err, ShouldBeNil)

		c := injectTestDB(context.Background(), db)

		Convey("User is using IP whitelist and IP is in the whitelist.", func() {
			auth := Authenticator{fakeOAuthMethod{email: "abc@example.com"}}
			req := makeRequest()
			req.RemoteAddr = "1.2.3.4"
			c, err := auth.Authenticate(c, req)
			So(err, ShouldBeNil)
			So(CurrentIdentity(c), ShouldEqual, identity.Identity("user:abc@example.com"))
		})

		Convey("User is using IP whitelist and IP is NOT in the whitelist.", func() {
			auth := Authenticator{fakeOAuthMethod{email: "abc@example.com"}}
			req := makeRequest()
			req.RemoteAddr = "1.2.3.5"
			_, err := auth.Authenticate(c, req)
			So(err, ShouldEqual, ErrIPNotWhitelisted)
		})

		Convey("User is not using IP whitelist.", func() {
			auth := Authenticator{fakeOAuthMethod{email: "def@example.com"}}
			req := makeRequest()
			req.RemoteAddr = "1.2.3.5"
			c, err := auth.Authenticate(c, req)
			So(err, ShouldBeNil)
			So(CurrentIdentity(c), ShouldEqual, identity.Identity("user:def@example.com"))
		})
	})
}

///

func makeRequest() *http.Request {
	req, _ := http.NewRequest("GET", "http://some-url", nil)
	return req
}

///

// fakeOAuthMethod implements Method.
type fakeOAuthMethod struct {
	clientID string
	email    string
}

func (m fakeOAuthMethod) Authenticate(context.Context, *http.Request) (*User, error) {
	email := m.email
	if email == "" {
		email = "abc@example.com"
	}
	return &User{
		Identity: identity.Identity("user:" + email),
		Email:    email,
		ClientID: m.clientID,
	}, nil
}

func injectTestDB(c context.Context, d authdb.DB) context.Context {
	return SetConfig(c, Config{
		DBProvider: func(c context.Context) (authdb.DB, error) {
			return d, nil
		},
	})
}

func strPtr(s string) *string { return &s }

///

// fakeDB implements DB.
type fakeDB struct {
	allowedClientID string
	authServiceURL  string
}

func (db *fakeDB) IsAllowedOAuthClientID(c context.Context, email, clientID string) (bool, error) {
	return clientID == db.allowedClientID, nil
}

func (db *fakeDB) IsMember(c context.Context, id identity.Identity, group string) (bool, error) {
	return true, nil
}

func (db *fakeDB) SharedSecrets(c context.Context) (secrets.Store, error) {
	return nil, errors.New("fakeDB: SharedSecrets is not implemented")
}

func (db *fakeDB) GetWhitelistForIdentity(c context.Context, ident identity.Identity) (string, error) {
	return "", nil
}

func (db *fakeDB) IsInWhitelist(c context.Context, ip net.IP, whitelist string) (bool, error) {
	return whitelist == "bots" && ip.String() == "1.2.3.4", nil
}

func (db *fakeDB) GetAuthServiceURL(c context.Context) (string, error) {
	if db.authServiceURL == "" {
		return "", errors.New("fakeDB: GetAuthServiceURL is not configured")
	}
	return db.authServiceURL, nil
}
