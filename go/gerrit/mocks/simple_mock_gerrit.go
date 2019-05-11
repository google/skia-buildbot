package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
)

// Mock that implements all methods of GerritInterface.
type SimpleGerritInterface struct {
	mock.Mock
	IssueID int64
}

func (g *SimpleGerritInterface) Initialized() bool {
	return true
}
func (g *SimpleGerritInterface) TurnOnAuthenticatedGets() {
}
func (g *SimpleGerritInterface) Url(issueID int64) string {
	return ""
}
func (g *SimpleGerritInterface) GetUserEmail() (string, error) {
	return "", nil
}
func (g *SimpleGerritInterface) GetRepoUrl() string {
	return ""
}
func (g *SimpleGerritInterface) ExtractIssueFromCommit(commitMsg string) (int64, error) {
	return 0, nil
}
func (g *SimpleGerritInterface) GetIssueProperties(issue int64) (*gerrit.ChangeInfo, error) {
	return &gerrit.ChangeInfo{Issue: issue}, nil
}
func (g *SimpleGerritInterface) GetPatch(issue int64, revision string) (string, error) {
	return "", nil
}
func (g *SimpleGerritInterface) SetReview(issue *gerrit.ChangeInfo, message string, labels map[string]interface{}, reviewers []string) error {
	return nil
}
func (g *SimpleGerritInterface) AddComment(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) SendToDryRun(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) SendToCQ(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) RemoveFromCQ(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) Approve(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) NoScore(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) DisApprove(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) Abandon(issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) SetReadyForReview(issue *gerrit.ChangeInfo) error {
	return nil
}
func (g *SimpleGerritInterface) SetTopic(topic string, changeNum int64) error {
	return nil
}
func (g *SimpleGerritInterface) Search(limit int, terms ...*gerrit.SearchTerm) ([]*gerrit.ChangeInfo, error) {
	results := make([]*gerrit.ChangeInfo, 0)
	results = append(results, &gerrit.ChangeInfo{Issue: g.IssueID})
	return results, nil
}
func (g *SimpleGerritInterface) GetTrybotResults(ctx context.Context, issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) Files(issue int64, patch string) (map[string]*gerrit.FileInfo, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) GetFileNames(issue int64, patch string) ([]string, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) IsBinaryPatch(issue int64, patch string) (bool, error) {
	return false, nil
}

func (g *SimpleGerritInterface) CreateChange(string, string, string, string) (*gerrit.ChangeInfo, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) EditFile(ci *gerrit.ChangeInfo, filepath, content string) error {
	return nil
}

func (g *SimpleGerritInterface) MoveFile(ci *gerrit.ChangeInfo, oldPath, newPath string) error {
	return nil
}

func (g *SimpleGerritInterface) DeleteFile(ci *gerrit.ChangeInfo, filepath string) error {
	return nil
}

func (g *SimpleGerritInterface) SetCommitMessage(ci *gerrit.ChangeInfo, msg string) error {
	return nil
}

func (g *SimpleGerritInterface) PublishChangeEdit(ci *gerrit.ChangeInfo) error {
	return nil
}

func (g *SimpleGerritInterface) DeleteChangeEdit(ci *gerrit.ChangeInfo) error {
	return nil
}

// Make sure MockGerrit fulfills GerritInterface
var _ gerrit.GerritInterface = (*SimpleGerritInterface)(nil)
