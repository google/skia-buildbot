// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bootstrap

import (
	"errors"
	"testing"

	"github.com/luci/luci-go/client/environ"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamclient"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type sentinelClient struct{}

func (*sentinelClient) NewStream(f streamproto.Flags) (streamclient.Stream, error) { return nil, nil }

func TestBootstrap(t *testing.T) {
	Convey(`A test Environment`, t, func() {
		reg := &streamclient.Registry{}
		regSpec := ""
		regErr := error(nil)
		reg.Register("test", func(spec string) (streamclient.Client, error) {
			regSpec = spec
			return &sentinelClient{}, regErr
		})

		env := environ.Environment{
			"IRRELEVANT": "VALUE",
		}

		Convey(`With no Butler values will return ErrNotBootstrapped.`, func() {
			_, err := getFromEnv(env, reg)
			So(err, ShouldEqual, ErrNotBootstrapped)
		})

		Convey(`With a Butler project and prefix`, func() {
			env[EnvStreamProject] = "test-project"
			env[EnvStreamPrefix] = "butler/prefix"

			Convey(`Yields a Bootstrap with a Project, Prefix, and no Client.`, func() {
				bs, err := getFromEnv(env, reg)
				So(err, ShouldBeNil)

				So(bs, ShouldResemble, &Bootstrap{
					Project: "test-project",
					Prefix:  "butler/prefix",
				})
			})

			Convey(`And a stream server path`, func() {
				env[EnvStreamServerPath] = "test:client:params"

				Convey(`Yields a Bootstrap with a Prefix and Client.`, func() {
					bs, err := getFromEnv(env, reg)
					So(err, ShouldBeNil)

					So(bs, ShouldResemble, &Bootstrap{
						Project: "test-project",
						Prefix:  "butler/prefix",
						Client:  &sentinelClient{},
					})
					So(regSpec, ShouldEqual, "client:params")
				})

				Convey(`If Client creation fails, will fail.`, func() {
					regErr = errors.New("testing error")
					_, err := getFromEnv(env, reg)
					So(err, ShouldErrLike, "failed to create stream client")
				})
			})

			Convey(`With an invalid Butler prefix, will fail.`, func() {
				env[EnvStreamPrefix] = "_notavaildprefix"
				_, err := getFromEnv(env, reg)
				So(err, ShouldErrLike, "failed to validate prefix")
			})

			Convey(`With an missing Butler project, will fail.`, func() {
				delete(env, EnvStreamProject)
				_, err := getFromEnv(env, reg)
				So(err, ShouldErrLike, "failed to validate project")
			})

			Convey(`With an invalid Butler project, will fail.`, func() {
				env[EnvStreamProject] = "_notavaildproject"
				_, err := getFromEnv(env, reg)
				So(err, ShouldErrLike, "failed to validate project")
			})
		})
	})
}
