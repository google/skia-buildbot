// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package memory

import (
	"testing"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMemoryImpl(t *testing.T) {
	Convey("with memory implementation", t, func() {
		ctx := context.Background()
		impl := New(map[string]ConfigSet{
			"services/abc": {
				"file": "body",
			},
			"projects/proj1": {
				"file": "project1 file",
			},
			"projects/proj2": {
				"file":         "project2 file",
				"another/file": "project2 another file",
			},
			"projects/proj1/refs/heads/master": {
				"file": "project1 master ref",
			},
			"projects/proj1/refs/heads/other": {
				"file": "project1 other ref",
			},
			"projects/proj2/refs/heads/master": {
				"file": "project2 master ref",
			},
			"projects/proj3/refs/heads/blah": {
				"filezzz": "project2 blah ref",
			},
		})

		Convey("GetConfig works", func() {
			cfg, err := impl.GetConfig(ctx, "services/abc", "file", false)
			So(err, ShouldBeNil)
			So(cfg, ShouldResemble, &config.Config{
				ConfigSet:   "services/abc",
				Path:        "file",
				Content:     "body",
				ContentHash: "v1:fb4c35e739d53994aba7d3e0416a1082f11bfbba",
				Revision:    "a9ae6f9d4d7ee130e6d77b5bf6cc94c681318a47",
			})
		})

		Convey("GetConfig hashOnly works", func() {
			cfg, err := impl.GetConfig(ctx, "services/abc", "file", true)
			So(err, ShouldBeNil)
			So(cfg, ShouldResemble, &config.Config{
				ConfigSet:   "services/abc",
				Path:        "file",
				ContentHash: "v1:fb4c35e739d53994aba7d3e0416a1082f11bfbba",
				Revision:    "a9ae6f9d4d7ee130e6d77b5bf6cc94c681318a47",
			})
		})

		Convey("GetConfig missing set", func() {
			cfg, err := impl.GetConfig(ctx, "missing/set", "path", false)
			So(cfg, ShouldBeNil)
			So(err, ShouldEqual, config.ErrNoConfig)
		})

		Convey("GetConfig missing path", func() {
			cfg, err := impl.GetConfig(ctx, "services/abc", "missing file", false)
			So(cfg, ShouldBeNil)
			So(err, ShouldEqual, config.ErrNoConfig)
		})

		Convey("GetConfigByHash works", func() {
			body, err := impl.GetConfigByHash(ctx, "v1:fb4c35e739d53994aba7d3e0416a1082f11bfbba")
			So(err, ShouldBeNil)
			So(body, ShouldEqual, "body")
		})

		Convey("GetConfigByHash missing hash", func() {
			body, err := impl.GetConfigByHash(ctx, "v1:blarg")
			So(err, ShouldEqual, config.ErrNoConfig)
			So(body, ShouldEqual, "")
		})

		Convey("GetConfigSetLocation works", func() {
			loc, err := impl.GetConfigSetLocation(ctx, "services/abc")
			So(err, ShouldBeNil)
			So(loc, ShouldNotBeNil)
		})

		Convey("GetProjectConfigs works", func() {
			cfgs, err := impl.GetProjectConfigs(ctx, "file", false)
			So(err, ShouldBeNil)
			So(cfgs, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/proj1",
					Path:        "file",
					Content:     "project1 file",
					ContentHash: "v1:4eb9d5ca35782bed53bbaae001306251b9471ff8",
					Revision:    "c57ee9f7b1ce4d1f145f76c7a3d908c800a923c8",
				},
				{
					ConfigSet:   "projects/proj2",
					Path:        "file",
					Content:     "project2 file",
					ContentHash: "v1:1d1ac7078c40817f0bb2c41be3c3a6ee47d99b54",
					Revision:    "bc2557da36bfa9db25ee678e773c2607bcb6068c",
				},
			})
		})

		Convey("GetProjectConfigs hashesOnly works", func() {
			cfgs, err := impl.GetProjectConfigs(ctx, "file", true)
			So(err, ShouldBeNil)
			So(cfgs, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/proj1",
					Path:        "file",
					ContentHash: "v1:4eb9d5ca35782bed53bbaae001306251b9471ff8",
					Revision:    "c57ee9f7b1ce4d1f145f76c7a3d908c800a923c8",
				},
				{
					ConfigSet:   "projects/proj2",
					Path:        "file",
					ContentHash: "v1:1d1ac7078c40817f0bb2c41be3c3a6ee47d99b54",
					Revision:    "bc2557da36bfa9db25ee678e773c2607bcb6068c",
				},
			})
		})

		Convey("GetProjectConfigs unknown file", func() {
			cfgs, err := impl.GetProjectConfigs(ctx, "unknown file", false)
			So(err, ShouldBeNil)
			So(len(cfgs), ShouldEqual, 0)
		})

		Convey("GetProjects works", func() {
			proj, err := impl.GetProjects(ctx)
			So(err, ShouldBeNil)
			So(proj, ShouldResemble, []config.Project{
				{
					ID:       "proj1",
					Name:     "Proj1",
					RepoType: config.GitilesRepo,
				},
				{
					ID:       "proj2",
					Name:     "Proj2",
					RepoType: config.GitilesRepo,
				},
				{
					ID:       "proj3",
					Name:     "Proj3",
					RepoType: config.GitilesRepo,
				},
			})
		})

		Convey("GetRefConfigs works", func() {
			cfg, err := impl.GetRefConfigs(ctx, "file", false)
			So(err, ShouldBeNil)
			So(cfg, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/proj1/refs/heads/master",
					Path:        "file",
					Content:     "project1 master ref",
					ContentHash: "v1:ef997153c60bd293248d146aa7d8e73080ab4d03",
					Revision:    "cd5ecf349116150a828f076cc5faeb2cf9d0e8c2",
				},
				{
					ConfigSet:   "projects/proj1/refs/heads/other",
					Path:        "file",
					Content:     "project1 other ref",
					ContentHash: "v1:1cfd1169b62b807e8dc10725f171bb0d8246dcd4",
					Revision:    "22760df658f5124ea212f7dac5ff36d511950582",
				},
				{
					ConfigSet:   "projects/proj2/refs/heads/master",
					Path:        "file",
					Content:     "project2 master ref",
					ContentHash: "v1:1fdb77cd2ce14bc5cadbb012692a65ef4a0e3a55",
					Revision:    "841da20f3e01271c6b9f7fec6244d352272f8aee",
				},
			})
		})

		Convey("GetRefConfigs hashesOnly works", func() {
			cfg, err := impl.GetRefConfigs(ctx, "file", true)
			So(err, ShouldBeNil)
			So(cfg, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/proj1/refs/heads/master",
					Path:        "file",
					ContentHash: "v1:ef997153c60bd293248d146aa7d8e73080ab4d03",
					Revision:    "cd5ecf349116150a828f076cc5faeb2cf9d0e8c2",
				},
				{
					ConfigSet:   "projects/proj1/refs/heads/other",
					Path:        "file",
					ContentHash: "v1:1cfd1169b62b807e8dc10725f171bb0d8246dcd4",
					Revision:    "22760df658f5124ea212f7dac5ff36d511950582",
				},
				{
					ConfigSet:   "projects/proj2/refs/heads/master",
					Path:        "file",
					ContentHash: "v1:1fdb77cd2ce14bc5cadbb012692a65ef4a0e3a55",
					Revision:    "841da20f3e01271c6b9f7fec6244d352272f8aee",
				},
			})
		})

		Convey("GetRefConfigs no configs", func() {
			cfg, err := impl.GetRefConfigs(ctx, "unknown file", false)
			So(err, ShouldBeNil)
			So(len(cfg), ShouldEqual, 0)
		})

		Convey("GetRefs works", func() {
			refs, err := impl.GetRefs(ctx, "proj1")
			So(err, ShouldBeNil)
			So(refs, ShouldResemble, []string{"refs/heads/master", "refs/heads/other"})

			refs, err = impl.GetRefs(ctx, "proj2")
			So(err, ShouldBeNil)
			So(refs, ShouldResemble, []string{"refs/heads/master"})
		})

		Convey("GetRefs unknown project", func() {
			refs, err := impl.GetRefs(ctx, "unknown project")
			So(err, ShouldBeNil)
			So(len(refs), ShouldEqual, 0)
		})
	})
}
