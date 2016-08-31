// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/common/types"
)

type coordinatorLinkGenerator struct {
	host    string
	project config.ProjectName
	prefix  types.StreamName
}

func (g *coordinatorLinkGenerator) canGenerateLinks() bool {
	return (g.host != "" && g.prefix != "")
}

func (g *coordinatorLinkGenerator) GetLink(names ...types.StreamName) string {
	links := make([]string, len(names))
	for i, n := range names {
		streamName := string(g.prefix.Join(n))
		proj := g.project
		if proj == "" {
			proj = "_"
		}
		links[i] = fmt.Sprintf("s=%s", url.QueryEscape(fmt.Sprintf("%s/%s", proj, streamName)))
	}
	return fmt.Sprintf("https://%s/v/?%s", g.host, strings.Join(links, "&"))
}
