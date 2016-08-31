// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/config/impl/remote"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type testServicesClient struct {
	logdog.ServicesClient
	cfg logdog.GetConfigResponse
	err error
}

func (tsc *testServicesClient) GetConfig(context.Context, *google.Empty, ...grpc.CallOption) (
	*logdog.GetConfigResponse, error) {
	if tsc.err != nil {
		return nil, tsc.err
	}
	return &tsc.cfg, nil
}

func TestFlag(t *testing.T) {
	Convey(`A set of config Flags`, t, func() {
		c := context.Background()

		f := Flags{}

		Convey(`Can parse command-line options.`, func() {
			fs := flag.NewFlagSet("test", flag.PanicOnError)
			f.AddToFlagSet(fs)

			So(fs.Parse([]string{
				"-config-file-path", "test.cfg",
				"-config-kill-interval", "3m",
			}), ShouldBeNil)
			So(f.ConfigFilePath, ShouldEqual, "test.cfg")
			So(f.KillCheckInterval, ShouldEqual, 3*time.Minute)
		})

		Convey(`Using a Coordinator client stub`, func() {
			tsc := testServicesClient{}
			tsc.cfg.ConfigServiceUrl = "http://example.com"
			tsc.cfg.ConfigSet = "services/testservice"
			tsc.cfg.ServiceConfigPath = "configpath.cfg"

			Convey(`Will load configuration from luci-config by default.`, func() {
				o, err := f.CoordinatorOptions(c, &tsc)
				So(err, ShouldBeNil)
				So(o.Config, ShouldHaveSameTypeAs, remote.New("https://example.com", nil))
			})

			Convey(`Will fail to create Options if no config service is specified.`, func() {
				tsc.cfg.ConfigServiceUrl = ""

				_, err := f.CoordinatorOptions(c, &tsc)
				So(err, ShouldErrLike, "coordinator does not specify a config service")
			})

			Convey(`Will fail to create Options if no config set is specified.`, func() {
				tsc.cfg.ConfigSet = ""

				_, err := f.CoordinatorOptions(c, &tsc)
				So(err, ShouldErrLike, "coordinator does not specify a config set")
			})

			Convey(`Will fail to create Options if no config path is specified.`, func() {
				tsc.cfg.ServiceConfigPath = ""

				_, err := f.CoordinatorOptions(c, &tsc)
				So(err, ShouldErrLike, "coordinator does not specify a config path")
			})

			Convey(`When loading from a testing file using a Coordinator stub.`, func() {
				tdir, err := ioutil.TempDir("", "configTest")
				So(err, ShouldBeNil)
				defer os.RemoveAll(tdir)

				f.ConfigFilePath = filepath.Join(tdir, "config")
				writeConfig := func(cfg *svcconfig.Config) error {
					servicePath := filepath.Join(f.ConfigFilePath, "services", "testservice")
					os.MkdirAll(servicePath, 0755)

					fd, err := os.Create(filepath.Join(servicePath, "configpath.cfg"))
					if err != nil {
						return err
					}
					defer fd.Close()

					return proto.MarshalText(fd, cfg)
				}

				cfg := &svcconfig.Config{
					Transport: &svcconfig.Transport{
						Type: &svcconfig.Transport_Pubsub{
							Pubsub: &svcconfig.Transport_PubSub{
								Project:      "foo",
								Topic:        "bar",
								Subscription: "baz",
							},
						},
					},
				}
				So(writeConfig(cfg), ShouldBeNil)

				Convey(`Will fail to load Options if the Coordinator config could not be fetched.`, func() {
					tsc := testServicesClient{err: errors.New("test error")}
					_, err := f.CoordinatorOptions(c, &tsc)
					So(err, ShouldErrLike, "test error")
				})

				Convey(`Will load options from the configuration file.`, func() {
					o, err := f.CoordinatorOptions(c, &tsc)
					So(err, ShouldBeNil)
					So(o, ShouldNotBeNil)

					Convey(`Can load configuration from the file.`, func() {
						m, err := NewManager(c, *o)
						So(err, ShouldBeNil)
						So(m.Config(), ShouldResemble, cfg)
					})
				})
			})
		})
	})
}
