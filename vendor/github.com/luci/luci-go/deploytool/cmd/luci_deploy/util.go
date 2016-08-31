// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

func unmarshalTextProtobuf(path string, msg proto.Message) error {
	data, err := ioutil.ReadFile(path)
	switch {
	case err == nil:
		if err := proto.UnmarshalText(string(data), msg); err != nil {
			return errors.Annotate(err).Reason("failed to unmarshal %(type)T from [%(path)s]").
				D("type", msg).D("path", path).Err()
		}
		return nil

	case isNotExist(err):
		// Forward this so it can be tested.
		return err

	default:
		return errors.Annotate(err).Reason("failed to read data from [%(path)s]").D("path", path).Err()
	}
}

func unmarshalTextProtobufDir(base string, fis []os.FileInfo, msg proto.Message, cb func(name string) error) error {
	for _, fi := range fis {
		name := fi.Name()
		if isHidden(name) {
			continue
		}

		if err := unmarshalTextProtobuf(filepath.Join(base, name), msg); err != nil {
			return errors.Annotate(err).Reason("failed to unmarshal file [%(name)s]").D("name", name).Err()
		}
		if err := cb(name); err != nil {
			return errors.Annotate(err).Reason("failed to process file [%(name)s]").D("name", name).Err()
		}
	}
	return nil
}

func logError(c context.Context, err error, f string, args ...interface{}) {
	log.WithError(err).Errorf(c, f, args...)
	if log.IsLogging(c, log.Debug) {
		log.Debugf(c, "Error stack:\n%s", strings.Join(errors.RenderStack(err).ToLines(), "\n"))
	}
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Unwrap(err))
}

func splitTitlePath(s string) (title, string) {
	switch v := strings.SplitN(s, "/", 2); len(v) {
	case 1:
		return title(v[0]), ""
	default:
		return title(v[0]), v[1]
	}
}

func joinPath(titles ...title) string {
	comps := make([]string, len(titles))
	for i, t := range titles {
		comps[i] = string(t)
	}
	return strings.Join(comps, "/")
}

func splitSourcePath(v string) (group, source title) {
	var tail string
	group, tail = splitTitlePath(v)
	source = title(tail)
	return
}

func splitComponentPath(v string) (deployment, component title) {
	var tail string
	deployment, tail = splitTitlePath(v)
	if tail != "" {
		component = title(tail)
	}
	return
}

func splitGoPackage(pkg string) []string {
	// Check intermediate paths to make sure there isn't a deployment
	// conflict.
	var (
		parts   []string
		lastIdx = 0
	)
	for {
		idx := strings.IndexRune(pkg[lastIdx:], '/')
		if idx < 0 {
			// Last component, don't check/register.
			return append(parts, pkg)
		}
		parts = append(parts, pkg[:lastIdx+idx])
		lastIdx += idx + len("/")
	}
}

func findGoPackage(pkg string, goPath []string) string {
	bctx := build.Default
	bctx.GOPATH = strings.Join(goPath, string(os.PathListSeparator))
	p, err := bctx.Import(pkg, "", build.FindOnly)
	if err != nil {
		return ""
	}
	return p.Dir
}
