package gerrit

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"go.skia.org/infra/go/util"
)

// ChangeEdit represents a modification to a change in Gerrit.
type ChangeEdit struct {
	g  *Gerrit
	ci *ChangeInfo
}

// Create a ChangeEdit for this ChangeInfo.
func (ci *ChangeInfo) Edit(g *Gerrit) *ChangeEdit {
	return &ChangeEdit{
		g:  g,
		ci: ci,
	}
}

// Modify the given file to have the given content.
func (ce *ChangeEdit) EditFile(filepath, content string) error {
	url := ce.g.url + fmt.Sprintf("/a/changes/%s/edit/%s", ce.ci.Id, url.QueryEscape(filepath))
	b := []byte(content)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
	if err != nil {
		return err
	}

	if err := ce.g.addAuthenticationCookie(req); err != nil {
		return err
	}
	resp, err := ce.g.client.Do(req)
	if err != nil {
		return err
	}
	defer util.Close(resp.Body)
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return fmt.Errorf("Got status %s (%d): %s", resp.Status, resp.StatusCode, string(respBytes))
	}
	return nil
}

// Move a given file.
func (ce *ChangeEdit) MoveFile(oldPath, newPath string) error {
	data := struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}{
		OldPath: oldPath,
		NewPath: newPath,
	}
	return ce.g.postJson(fmt.Sprintf("/a/changes/%s/edit", ce.ci.Id), data)
}

// Delete the given file.
func (ce *ChangeEdit) DeleteFile(filepath string) error {
	return ce.g.delete(fmt.Sprintf("/a/changes/%s/edit/%s", ce.ci.Id, url.QueryEscape(filepath)))
}

// Set the commit message for the ChangeEdit.
func (ce *ChangeEdit) SetCommitMessage(msg string) error {
	m := struct {
		Message string `json:"message"`
	}{
		Message: msg,
	}
	url := fmt.Sprintf("/a/changes/%s/edit:message", ce.ci.Id)
	return ce.g.putJson(url, m)
}

// Publish the ChangeEdit as a new patch set.
func (ce *ChangeEdit) Publish() error {
	msg := struct {
		Notify string `json:"notify,omitempty"`
	}{
		Notify: "ALL",
	}
	url := fmt.Sprintf("/a/changes/%s/edit:publish", ce.ci.Id)
	return ce.g.postJson(url, msg)
}

// Delete the ChangeEdit, restoring the state to the last patch set.
func (ce *ChangeEdit) Delete() error {
	return ce.g.delete(fmt.Sprintf("/a/changes/%s/edit", ce.ci.Id))
}

// WithNewCL creates a new empty change in Gerrit, creates a ChangeEdit for
// the change, sets the commit message, and then runs the given function. If any
// of the above fails, the ChangeEdit (but not the change itself) is deleted.
// Otherwise, the ChangeEdit is published as a new patch set on the change and
// the ChangeInfo for the new change is returned.
func WithNewCL(g *Gerrit, project, branch, commitMsg string, fn func(*ChangeEdit) error) (rv *ChangeInfo, rvErr error) {
	cl, err := g.CreateChange(project, branch, strings.Split(commitMsg, "\n")[0])
	if err != nil {
		return nil, err
	}
	edit := cl.Edit(g)
	if err := edit.SetCommitMessage(commitMsg); err != nil {
		return nil, err
	}
	defer func() {
		if rvErr == nil {
			rvErr = edit.Publish()
		}
		if rvErr != nil {
			if err := edit.Delete(); err != nil {
				rvErr = fmt.Errorf("%s and failed to delete edit with: %s", rvErr, err)
			}
		}
	}()
	if err := fn(edit); err != nil {
		return nil, err
	}
	return g.GetIssueProperties(cl.Issue)
}
