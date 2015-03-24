// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/*
	Utilities for interacting with the GoogleCode issue tracker.

	Example usage:
		issueTracker := issue_tracker.New()
*/
package issue_tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"go.skia.org/infra/go/util"
)

// BugPriorities are the possible values for "Priority-*" labels for issues.
var BugPriorities = []string{"Critical", "High", "Medium", "Low", "Never"}

var OAUTH_SCOPE = []string{
	"https://www.googleapis.com/auth/projecthosting",
	"https://www.googleapis.com/auth/userinfo.email",
}

const (
	ISSUE_API_URL  = "https://www.googleapis.com/projecthosting/v2/projects/"
	ISSUE_URL      = "https://code.google.com/p/skia/issues/detail?id="
	PERSON_API_URL = "https://www.googleapis.com/userinfo/v2/me"
)

// Enum for determining whether a label has been added, removed, or is
// unchanged.
const (
	LABEL_ADDED = iota
	LABEL_REMOVED
	LABEL_UNCHANGED
)

// Issue contains information about an issue.
type Issue struct {
	Id      int      `json:"id"`
	Project string   `json:"projectId"`
	Title   string   `json:"title"`
	Labels  []string `json:"labels"`
}

// URL returns the URL of a given issue.
func (i Issue) URL() string {
	return ISSUE_URL + strconv.Itoa(i.Id)
}

// IssueList represents a list of issues from the IssueTracker.
type IssueList struct {
	TotalResults int      `json:"totalResults"`
	Items        []*Issue `json:"items"`
}

// IssueTracker is the primary point of contact with the issue tracker,
// providing methods for authenticating to and interacting with it.
type IssueTracker struct {
	client *http.Client
}

// New creates and returns an IssueTracker which makes requests via the given
// http.Client.
func New(client *http.Client) *IssueTracker {
	return &IssueTracker{
		client,
	}
}

// GetBug retrieves the Issue with the given ID from the IssueTracker.
func (it IssueTracker) GetBug(project string, id int) (*Issue, error) {
	errFmt := fmt.Sprintf("error retrieving issue %d: %s", id, "%s")
	requestURL := ISSUE_API_URL + project + "/issues/" + strconv.Itoa(id)
	resp, err := it.client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf(errFmt, err)
	}
	defer util.Close(resp.Body)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errFmt, fmt.Sprintf(
			"issue tracker returned code %d:%v", resp.StatusCode, string(body)))
	}
	var issue Issue
	if err := json.Unmarshal(body, &issue); err != nil {
		return nil, fmt.Errorf(errFmt, err)
	}
	return &issue, nil
}

// GetBugs retrieves all Issues with the given owner from the IssueTracker,
// returning an IssueList.
func (it IssueTracker) GetBugs(project string, owner string) (*IssueList, error) {
	errFmt := "error retrieving issues: %s"
	params := map[string]string{
		"owner":      url.QueryEscape(owner),
		"can":        "open",
		"maxResults": "9999",
	}
	requestURL := ISSUE_API_URL + project + "/issues?"
	first := true
	for k, v := range params {
		if first {
			first = false
		} else {
			requestURL += "&"
		}
		requestURL += k + "=" + v
	}
	resp, err := it.client.Get(requestURL)
	if err != nil {
		return nil, fmt.Errorf(errFmt, err)
	}
	defer util.Close(resp.Body)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(errFmt, fmt.Sprintf(
			"issue tracker returned code %d:%v", resp.StatusCode, string(body)))
	}

	var bugList IssueList
	if err := json.Unmarshal(body, &bugList); err != nil {
		return nil, fmt.Errorf(errFmt, err)
	}
	return &bugList, nil
}

// SubmitIssueChanges creates a comment on the given Issue which modifies it
// according to the contents of the passed-in Issue struct.
func (it IssueTracker) SubmitIssueChanges(issue *Issue, comment string) error {
	errFmt := "Error updating issue " + strconv.Itoa(issue.Id) + ": %s"
	oldIssue, err := it.GetBug(issue.Project, issue.Id)
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}
	postData := struct {
		Content string `json:"content"`
		Updates struct {
			Title  *string  `json:"summary"`
			Labels []string `json:"labels"`
		} `json:"updates"`
	}{
		Content: comment,
	}
	if issue.Title != oldIssue.Title {
		postData.Updates.Title = &issue.Title
	}
	// TODO(borenet): Add other issue attributes, eg. Owner.
	labels := make(map[string]int)
	for _, label := range issue.Labels {
		labels[label] = LABEL_ADDED
	}
	for _, label := range oldIssue.Labels {
		if _, ok := labels[label]; ok {
			labels[label] = LABEL_UNCHANGED
		} else {
			labels[label] = LABEL_REMOVED
		}
	}
	labelChanges := make([]string, 0)
	for labelName, present := range labels {
		if present == LABEL_REMOVED {
			labelChanges = append(labelChanges, "-"+labelName)
		} else if present == LABEL_ADDED {
			labelChanges = append(labelChanges, labelName)
		}
	}
	if len(labelChanges) > 0 {
		postData.Updates.Labels = labelChanges
	}

	postBytes, err := json.Marshal(&postData)
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}
	requestURL := ISSUE_API_URL + issue.Project + "/issues/" + strconv.Itoa(issue.Id) + "/comments"
	resp, err := it.client.Post(requestURL, "application/json", bytes.NewReader(postBytes))
	if err != nil {
		return fmt.Errorf(errFmt, err)
	}
	defer util.Close(resp.Body)
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(errFmt, fmt.Sprintf(
			"Issue tracker returned code %d:%v", resp.StatusCode, string(body)))
	}
	return nil
}
