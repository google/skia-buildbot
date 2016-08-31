// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package frontend

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/milo/appengine/buildbot"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/appengine/swarming"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
	luciSettings "github.com/luci/luci-go/server/settings"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

var (
	allHandlers = []settings.TestableHandler{
		settings.TestableSettings{},
		buildbot.TestableBuild{},
		buildbot.TestableBuilder{},
		swarming.TestableBuild{},
		swarming.TestableLog{},
		testableFrontpage{},
	}
)

var generate = flag.Bool(
	"test.generate", false, "Generate expectations instead of running tests.")

func expectFileName(name string) string {
	name = strings.Replace(name, " ", "_", -1)
	name = strings.Replace(name, "/", "_", -1)
	name = strings.Replace(name, ":", "-", -1)
	return filepath.Join("expectations", name)
}

func load(name string) ([]byte, error) {
	filename := expectFileName(name)
	return ioutil.ReadFile(filename)
}

// mustWrite Writes a buffer into an expectation file.  Should always work or
// panic.  This is fine because this only runs when -generate is passed in,
// not during tests.
func mustWrite(name string, buf []byte) {
	filename := expectFileName(name)
	err := ioutil.WriteFile(filename, buf, 0644)
	if err != nil {
		panic(err)
	}
}

// fakeOAuthMethod implements Method.
type fakeOAuthMethod struct {
	clientID string
}

func (m fakeOAuthMethod) Authenticate(context.Context, *http.Request) (*auth.User, error) {
	return &auth.User{
		Identity: identity.Identity("user:abc@example.com"),
		Email:    "abc@example.com",
		ClientID: m.clientID,
	}, nil
}

func (m fakeOAuthMethod) LoginURL(context.Context, string) (string, error) {
	return "https://login.url/", nil
}

func (m fakeOAuthMethod) LogoutURL(context.Context, string) (string, error) {
	return "https://logout.url/", nil
}

type analyticsSettings struct {
	AnalyticsID string `json:"analytics_id"`
}

func TestPages(t *testing.T) {
	Convey("Testing basic rendering.", t, func() {
		// Load all the bundles.
		c := context.Background()
		c = memory.Use(c)
		c, _ = testclock.UseTime(c, testclock.TestTimeUTC)
		a := auth.Authenticator{fakeOAuthMethod{"some_client_id"}}
		c = auth.SetAuthenticator(c, a)
		c = luciSettings.Use(c, luciSettings.New(&luciSettings.MemoryStorage{Expiration: time.Second}))
		err := luciSettings.Set(c, "analytics", &analyticsSettings{"UA-12345-01"}, "", "")
		So(err, ShouldBeNil)
		for _, nb := range settings.GetTemplateBundles() {
			Convey(fmt.Sprintf("Testing theme %q", nb.Name), func() {
				err := nb.Bundle.EnsureLoaded(c)
				So(err, ShouldBeNil)
				for _, h := range allHandlers {
					hName := reflect.TypeOf(h).String()
					Convey(fmt.Sprintf("Testing handler %q", hName), func() {
						for _, b := range h.TestData() {
							Convey(fmt.Sprintf("Testing: %q", b.Description), func() {
								args := b.Data
								// This is not a path, but a file key, should always be "/".
								tmplName := fmt.Sprintf(
									"pages/%s", h.GetTemplateName(*nb.Theme))
								buf, err := nb.Bundle.Render(c, tmplName, args)
								So(err, ShouldBeNil)
								fname := fmt.Sprintf(
									"%s-%s-%s.html", nb.Name, hName, b.Description)
								if *generate {
									mustWrite(fname, buf)
								} else {
									localBuf, err := load(fname)
									So(err, ShouldBeNil)
									So(string(buf), ShouldEqual, string(localBuf))
								}
							})
						}
					})
				}
			})
		}
	})
}
