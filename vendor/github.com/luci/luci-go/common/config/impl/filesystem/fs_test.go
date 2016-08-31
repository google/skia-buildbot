// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package filesystem

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
)

func withFolder(files map[string]string, cb func(folder string)) {
	folder, err := ioutil.TempDir("", "fs_test_")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(folder)

	for fpath, content := range files {
		if content == "" {
			content = fpath
		}
		fpath = filepath.Join(folder, filepath.FromSlash(fpath))
		if err := os.MkdirAll(filepath.Dir(fpath), 0777); err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(fpath, []byte(content), 0666); err != nil {
			panic(err)
		}
	}

	cb(folder)
}

func TestFSImpl(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	withFolder(map[string]string{
		"projects/doodly/refs/otherref/file.cfg": "",
		"projects/doodly/refs/someref/file.cfg":  "",
		"projects/doodly/something/file.cfg":     "",
		"projects/foobar/refs/someref/file.cfg":  "",
		"projects/foobar/refs/bad.cfg":           "",
		"projects/foobar/something/file.cfg":     "",
		"services/foosrv/something.cfg":          "",
		"projects/foobar.json": `{
			"Name": "A cool project",
			"Url": "https://something.example.com"
		}`,
	}, func(folder string) {
		Convey("basic Test Filesystem config client", t, func() {
			client, err := New(folder)
			So(err, ShouldBeNil)

			Convey("GetConfig", func() {
				expect := &config.Config{
					ConfigSet:   "projects/foobar",
					Path:        "something/file.cfg",
					Content:     "projects/foobar/something/file.cfg",
					ContentHash: "v1:e42874cc28bbba410f56790c24bb6f33e73ab784",
					Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
				}

				Convey("All content", func() {
					cfg, err := client.GetConfig(ctx, "projects/foobar", "something/file.cfg", false)
					So(err, ShouldBeNil)
					So(cfg, ShouldResemble, expect)
				})

				Convey("services", func() {
					cfg, err := client.GetConfig(ctx, "services/foosrv", "something.cfg", false)
					So(err, ShouldBeNil)
					So(cfg, ShouldResemble, &config.Config{
						ConfigSet:   "services/foosrv",
						Path:        "something.cfg",
						Content:     "services/foosrv/something.cfg",
						ContentHash: "v1:71ecbefbed9d895b71205724d3e693bc2ec12246",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					})
				})

				Convey("refs", func() {
					cfg, err := client.GetConfig(ctx, "projects/foobar/refs/someref", "file.cfg", false)
					So(err, ShouldBeNil)
					So(cfg, ShouldResemble, &config.Config{
						ConfigSet:   "projects/foobar/refs/someref",
						Path:        "file.cfg",
						Content:     "projects/foobar/refs/someref/file.cfg",
						ContentHash: "v1:82b0518dd04288c285023ff0534658e5b0df93d4",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					})
				})

				Convey("just hash", func() {
					cfg, err := client.GetConfig(ctx, "projects/foobar", "something/file.cfg", true)
					So(err, ShouldBeNil)
					So(cfg.ContentHash, ShouldEqual, "v1:e42874cc28bbba410f56790c24bb6f33e73ab784")

					Convey("make sure it doesn't poison the cache", func() {
						cfg, err := client.GetConfig(ctx, "projects/foobar", "something/file.cfg", false)
						So(err, ShouldBeNil)
						So(cfg, ShouldResemble, expect)
					})
				})
			})

			Convey("GetConfigByHash", func() {
				cont, err := client.GetConfigByHash(ctx, "v1:e42874cc28bbba410f56790c24bb6f33e73ab784")
				So(err, ShouldBeNil)
				So(cont, ShouldEqual, "projects/foobar/something/file.cfg")
			})

			Convey("GetConfigSetLocation", func() {
				csurl, err := client.GetConfigSetLocation(ctx, "projects/foobar")
				So(err, ShouldBeNil)
				So(csurl, ShouldResemble, &url.URL{
					Scheme: "file",
					Path:   filepath.ToSlash(folder) + "/projects/foobar",
				})
			})

			Convey("GetProjectConfigs", func() {
				cfgs, err := client.GetProjectConfigs(ctx, "something/file.cfg", false)
				So(err, ShouldBeNil)
				So(cfgs, ShouldResemble, []config.Config{
					{
						ConfigSet:   "projects/doodly",
						Path:        "something/file.cfg",
						Content:     "projects/doodly/something/file.cfg",
						ContentHash: "v1:a3b4b34e5c8dd1dd8dff3e643504ce28f9335e6f",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					},
					{
						ConfigSet:   "projects/foobar",
						Path:        "something/file.cfg",
						Content:     "projects/foobar/something/file.cfg",
						ContentHash: "v1:e42874cc28bbba410f56790c24bb6f33e73ab784",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					},
				})
			})

			Convey("GetProjects", func() {
				projs, err := client.GetProjects(ctx)
				So(err, ShouldBeNil)
				So(projs, ShouldResemble, []config.Project{
					{
						ID:       "doodly",
						Name:     "doodly",
						RepoType: "FILESYSTEM",
					},
					{
						ID:       "foobar",
						Name:     "A cool project",
						RepoType: "FILESYSTEM",
						RepoURL:  &url.URL{Scheme: "https", Host: "something.example.com"},
					},
				})
			})

			Convey("GetRefConfigs", func() {
				cfgs, err := client.GetRefConfigs(ctx, "file.cfg", false)
				So(err, ShouldBeNil)
				So(cfgs, ShouldResemble, []config.Config{
					{
						ConfigSet:   "projects/doodly/refs/otherref",
						Path:        "file.cfg",
						Content:     "projects/doodly/refs/otherref/file.cfg",
						ContentHash: "v1:0de822c33630b5be0aa78497c0918e0dd773c7cb",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					}, {
						ConfigSet:   "projects/doodly/refs/someref",
						Path:        "file.cfg",
						Content:     "projects/doodly/refs/someref/file.cfg",
						ContentHash: "v1:5e9963aa1551a9e9db8e7bebe6164c3b5d8aee97",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					}, {
						ConfigSet:   "projects/foobar/refs/someref",
						Path:        "file.cfg",
						Content:     "projects/foobar/refs/someref/file.cfg",
						ContentHash: "v1:82b0518dd04288c285023ff0534658e5b0df93d4",
						Revision:    "dc6481ef835f1c7625a8aa64cdfc33e6a975f626",
					},
				})
			})

			Convey("GetRefs", func() {
				refs, err := client.GetRefs(ctx, "foobar")
				So(err, ShouldBeNil)
				So(refs, ShouldResemble, []string{"refs/someref"})

				refs, err = client.GetRefs(ctx, "doodly")
				So(err, ShouldBeNil)
				So(refs, ShouldResemble, []string{"refs/otherref", "refs/someref"})
			})

		})
	})

	withFolder(map[string]string{
		"projects/doodly/refs/otherref/file.cfg": "",
		"projects/doodly/refs/someref/file.cfg":  "",
	}, func(folder string) {
		Convey("rereads configs in sloppy mode", t, func() {
			client, err := New(folder)
			So(err, ShouldBeNil)

			cfgs, err := client.GetRefConfigs(ctx, "file.cfg", false)
			So(err, ShouldBeNil)
			So(cfgs, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/doodly/refs/otherref",
					Path:        "file.cfg",
					Content:     "projects/doodly/refs/otherref/file.cfg",
					ContentHash: "v1:0de822c33630b5be0aa78497c0918e0dd773c7cb",
					Revision:    "37c845ce6697d135cfb03392c9589ed79bcb8b6c",
				}, {
					ConfigSet:   "projects/doodly/refs/someref",
					Path:        "file.cfg",
					Content:     "projects/doodly/refs/someref/file.cfg",
					ContentHash: "v1:5e9963aa1551a9e9db8e7bebe6164c3b5d8aee97",
					Revision:    "37c845ce6697d135cfb03392c9589ed79bcb8b6c",
				},
			})

			err = ioutil.WriteFile(
				filepath.Join(folder, filepath.FromSlash("projects/doodly/refs/otherref/file.cfg")),
				[]byte("blarg"),
				0666)
			So(err, ShouldBeNil)

			cfgs, err = client.GetRefConfigs(ctx, "file.cfg", false)
			So(err, ShouldBeNil)
			So(cfgs, ShouldResemble, []config.Config{
				{
					ConfigSet:   "projects/doodly/refs/otherref",
					Path:        "file.cfg",
					Content:     "blarg",
					ContentHash: "v1:4ccb603a6ce7eb3d310e4a7aab1022f5ff57fc0b",
					Revision:    "4eb3077a22e66ba9ea38dcab2e80b59dffe26de4",
				}, {
					ConfigSet:   "projects/doodly/refs/someref",
					Path:        "file.cfg",
					Content:     "projects/doodly/refs/someref/file.cfg",
					ContentHash: "v1:5e9963aa1551a9e9db8e7bebe6164c3b5d8aee97",
					Revision:    "4eb3077a22e66ba9ea38dcab2e80b59dffe26de4",
				},
			})
		})
	})

	versioned := map[string]string{
		"v1/projects/foobar/something/file.cfg": "",
		"v2/projects/foobar/something/file.cfg": "",
	}

	withFolder(versioned, func(folder string) {
		symlink := filepath.Join(folder, "link")

		Convey("Test versioned Filesystem", t, func() {
			So(errors.FilterFunc(os.Remove(symlink), os.IsNotExist), ShouldBeNil)
			So(os.Symlink(filepath.Join(folder, "v1"), symlink), ShouldBeNil)
			client, err := New(symlink)
			So(err, ShouldBeNil)

			Convey("v1", func() {
				cfg, err := client.GetConfig(ctx, "projects/foobar", "something/file.cfg", false)
				So(err, ShouldBeNil)
				So(cfg.Content, ShouldEqual, "v1/projects/foobar/something/file.cfg")

				Convey("v2", func() {
					So(errors.Filter(os.Remove(symlink), os.ErrNotExist), ShouldBeNil)
					So(os.Symlink(filepath.Join(folder, "v2"), symlink), ShouldBeNil)

					cfg, err := client.GetConfig(ctx, "projects/foobar", "something/file.cfg", false)
					So(err, ShouldBeNil)
					So(cfg.Content, ShouldEqual, "v2/projects/foobar/something/file.cfg")
				})
			})

		})
	})

}
