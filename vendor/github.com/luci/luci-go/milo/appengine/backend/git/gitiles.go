// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package git

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"
)

// Repo defines a git repository.
type Repo struct {
	// Server is the full path to a git repository.  Server must start with https://
	// and should not end with .git.
	Server string
	// Branch specifies a treeish of a git repository.  This is generally a branch.
	Branch string
}

// Author is the author returned from a gitiles log request.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Time  string `json:"time"`
}

// Committer is the committer information returned from a gitiles log request.
type Commiter struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Time  string `json:"time"`
}

// Log is the Log information of a commit returned from a gitiles log request.
type Log struct {
	Commit    string   `json:"commit"`
	Tree      string   `json:"tree"`
	Parents   []string `json:"parents"`
	Author    Author   `json:"author"`
	Committer Commiter `json:"committer"`
	Message   string   `json:"message"`
}

// Commit is the JSON response from querying gitiles for a log request.
type Commit struct {
	Log  []Log  `json:"log"`
	Next string `json:"next"`
}

// fixURL validates and normalizes a repoURL and treeish, and returns the
// log JSON gitiles URL.
func fixURL(repoURL, treeish string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("%s should start with https://", repoURL)
	}
	if !strings.HasSuffix(u.Host, "googlesource.com") {
		return "", fmt.Errorf("Only googlesource.com repos supported")
	}
	// Use the authenticated URL
	u.Path = "a/" + u.Path
	URL := fmt.Sprintf("%s/+log/%s?format=JSON", u.String(), treeish)
	return URL, nil
}

// GetCommits returns a list of commits based on a repo and treeish (usually
// a branch).  This should be equivilent of a "git log <treeish>" call in
// that repository.
func GetCommits(c context.Context, repoURL, treeish string, limit int) ([]resp.Commit, error) {
	// TODO(hinoka): Respect the limit.
	URL, err := fixURL(repoURL, treeish)
	if err != nil {
		return nil, err
	}
	t, err := auth.GetRPCTransport(c, auth.NoAuth)
	if err != nil {
		return nil, err
	}
	client := http.Client{Transport: t}
	r, err := client.Get(URL)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch %s, status code %d", URL, r.StatusCode)
	}
	defer r.Body.Close()
	// Strip out the jsonp header, which is ")]}'"
	trash := make([]byte, 4)
	r.Body.Read(trash) // Read the jsonp header
	commits := Commit{}
	if err := json.NewDecoder(r.Body).Decode(&commits); err != nil {
		return nil, err
	}
	// TODO(hinoka): If there is a page and we have gotten less than the limit,
	// keep making requests for the next page until we have enough commits.

	// Move things into our own datastructure.
	result := make([]resp.Commit, len(commits.Log))
	for i, log := range commits.Log {
		result[i] = resp.Commit{
			AuthorName:  log.Author.Name,
			AuthorEmail: log.Author.Email,
			Repo:        repoURL,
			Revision:    log.Commit,
			Description: log.Message,
			Title:       strings.SplitN(log.Message, "\n", 2)[0],
			// TODO(hinoka): Fill in the rest of resp.Commit and add those details
			// in the html.
		}
	}
	return result, nil
}
