// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authdbimpl

import (
	"fmt"
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/appengine/gaetesting"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth/service"
	"github.com/luci/luci-go/server/auth/service/protocol"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConfigureAuthService(t *testing.T) {
	Convey("Initial config", t, func() {
		srv := &fakeAuthService{LatestRev: 123}
		c := setAuthService(gaetesting.TestingContext(), srv)

		So(ConfigureAuthService(c, "http://base_url", "http://auth-service"), ShouldBeNil)
		So(srv.Calls, ShouldResemble, []string{
			`EnsureSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service" ""`,
		})

		info, err := GetLatestSnapshotInfo(c)
		So(err, ShouldBeNil)
		So(info, ShouldResemble, &SnapshotInfo{
			AuthServiceURL: "http://auth-service",
			Rev:            123,
		})

		// Coverage for GetAuthDBSnapshot.
		_, err = GetAuthDBSnapshot(c, "missing")
		So(err, ShouldEqual, datastore.ErrNoSuchEntity)
		snap, err := GetAuthDBSnapshot(c, info.GetSnapshotID())
		So(err, ShouldBeNil)
		So(snap, ShouldResemble, &protocol.AuthDB{
			OauthClientId:     strPtr("client-id-for-rev-123"),
			OauthClientSecret: strPtr("secret"),
		})

		// Same config call again triggers resubsciption.
		srv.Calls = nil
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service"), ShouldBeNil)
		So(srv.Calls, ShouldResemble, []string{
			`EnsureSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service" ""`,
		})
	})

	Convey("Switching cfg", t, func() {
		srv := &fakeAuthService{LatestRev: 123}
		c := setAuthService(gaetesting.TestingContext(), srv)

		// Initial config.
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service-1"), ShouldBeNil)
		// Change URL of the service.
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service-2"), ShouldBeNil)

		info, err := GetLatestSnapshotInfo(c)
		So(err, ShouldBeNil)
		So(info, ShouldResemble, &SnapshotInfo{
			AuthServiceURL: "http://auth-service-2",
			Rev:            123,
		})

		So(srv.Calls, ShouldResemble, []string{
			`EnsureSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service-1" ""`,
			`EnsureSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service-2" ""`,
			`DeleteSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service-1"`,
		})
	})

	Convey("Removing cfg", t, func() {
		srv := &fakeAuthService{LatestRev: 123}
		c := setAuthService(gaetesting.TestingContext(), srv)

		// Initial config.
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service-1"), ShouldBeNil)
		// Remove.
		So(ConfigureAuthService(c, "http://base_url", ""), ShouldBeNil)

		info, err := GetLatestSnapshotInfo(c)
		So(err, ShouldBeNil)
		So(info, ShouldBeNil)

		So(srv.Calls, ShouldResemble, []string{
			`EnsureSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service-1" ""`,
			`DeleteSubscription "projects/app/subscriptions/dev-app-server-v1+auth-service-1"`,
		})
	})
}

func TestSyncAuthDB(t *testing.T) {
	Convey("No new changes", t, func() {
		srv := &fakeAuthService{LatestRev: 123}
		c := setAuthService(gaetesting.TestingContext(), srv)
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service"), ShouldBeNil)

		info, err := syncAuthDB(c)
		So(err, ShouldBeNil)
		So(info, ShouldResemble, &SnapshotInfo{
			AuthServiceURL: "http://auth-service",
			Rev:            123,
		})
	})

	Convey("Have update", t, func() {
		srv := &fakeAuthService{LatestRev: 123}
		c := setAuthService(gaetesting.TestingContext(), srv)
		So(ConfigureAuthService(c, "http://base_url", "http://auth-service"), ShouldBeNil)

		srv.LatestRev = 456

		info, err := syncAuthDB(c)
		So(err, ShouldBeNil)
		So(info, ShouldResemble, &SnapshotInfo{
			AuthServiceURL: "http://auth-service",
			Rev:            456,
		})
	})
}

///

func strPtr(s string) *string {
	return &s
}

type fakeAuthService struct {
	LatestRev    int64
	Calls        []string
	Notification *service.Notification
}

func (f *fakeAuthService) EnsureSubscription(c context.Context, subscription, pushURL string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("EnsureSubscription %q %q", subscription, pushURL))
	return nil
}

func (f *fakeAuthService) DeleteSubscription(c context.Context, subscription string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("DeleteSubscription %q", subscription))
	return nil
}

func (f *fakeAuthService) PullPubSub(c context.Context, subscription string) (*service.Notification, error) {
	f.Calls = append(f.Calls, fmt.Sprintf("PullPubSub %q", subscription))
	return f.Notification, nil
}

func (f *fakeAuthService) ProcessPubSubPush(c context.Context, body []byte) (*service.Notification, error) {
	f.Calls = append(f.Calls, "ProcessPubSubPush")
	return f.Notification, nil
}

func (f *fakeAuthService) GetLatestSnapshotRevision(c context.Context) (int64, error) {
	return f.LatestRev, nil
}

func (f *fakeAuthService) GetSnapshot(c context.Context, rev int64) (*service.Snapshot, error) {
	if rev != f.LatestRev {
		return nil, fmt.Errorf("fakeAuthService: no snapshot for rev %d", rev)
	}
	return &service.Snapshot{
		AuthDB: &protocol.AuthDB{
			OauthClientId:     strPtr(fmt.Sprintf("client-id-for-rev-%d", f.LatestRev)),
			OauthClientSecret: strPtr("secret"),
		},
		Rev:     f.LatestRev,
		Created: clock.Now(c),
	}, nil
}
