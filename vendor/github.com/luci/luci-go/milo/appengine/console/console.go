// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package console

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/backend/git"
	"github.com/luci/luci-go/milo/appengine/buildbot"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/milo/common/config"
	"github.com/luci/luci-go/server/router"
	"golang.org/x/net/context"
)

// Returns results of build[commit_index][builder_index]
func GetConsoleBuilds(
	c context.Context, module string,
	builders []resp.BuilderRef, commits []string) (
	[][]*resp.ConsoleBuild, error) {

	switch module {
	case "buildbot":
		return buildbot.GetConsoleBuilds(c, builders, commits)
	// The case for buildbucket and dm goes here.
	default:
		panic(fmt.Errorf("Unrecognized module %s", module))
	}
}

// getConsoleDef finds the console definition as defined by any project.
// If the user is not a reader of the project, this will return a 404.
// TODO(hinoka): If the user is not a reader of any of of the builders returned,
// that builder will be removed from list of results.
func getConsoleDef(c context.Context, project, name string) (*config.Console, error) {
	cs, err := settings.GetConsole(c, project, name)
	if err != nil {
		return nil, err
	}
	// TODO(hinoka): Remove builders that the user does not have access to.
	return cs, nil
}

// Main is a redirect handler that redirects the user to the main console for a
// particular project.
func Main(ctx *router.Context) {
	w, r, p := ctx.Writer, ctx.Request, ctx.Params
	proj := p.ByName("project")
	http.Redirect(w, r, fmt.Sprintf("/console/%s/main", proj), http.StatusMovedPermanently)
	return
}

func console(c context.Context, project, name string) (*resp.Console, error) {
	tStart := clock.Now(c)
	def, err := getConsoleDef(c, project, name)
	if err != nil {
		return nil, err
	}
	commits, err := git.GetCommits(c, def.RepoURL, def.Branch, 25)
	if err != nil {
		return nil, err
	}
	tGitiles := clock.Now(c)
	log.Debugf(c, "Loading commits took %s.", tGitiles.Sub(tStart))
	commitNames := make([]string, len(commits))
	for i, commit := range commits {
		commitNames[i] = commit.Revision
	}

	// HACK(hinoka): This only supports buildbot....
	builders := make([]resp.BuilderRef, len(def.Builders))
	for i, b := range def.Builders {
		builders[i] = resp.BuilderRef{
			b.Module, b.Name, strings.Split(b.Category, "|"), b.ShortName,
		}
	}
	cb, err := GetConsoleBuilds(c, "buildbot", builders, commitNames)
	tConsole := clock.Now(c)
	log.Debugf(c, "Loading the console took a total of %s.", tConsole.Sub(tGitiles))
	if err != nil {
		return nil, err
	}
	ccb := make([]resp.CommitBuild, len(commits))
	for i, commit := range commitNames {
		// TODO(hinoka): Not like this
		ccb[i].Commit = resp.Commit{Revision: commit}
		ccb[i].Build = cb[i]
	}

	cs := &resp.Console{
		Name:       def.Name,
		Commit:     ccb,
		BuilderRef: builders,
	}

	return cs, nil
}
