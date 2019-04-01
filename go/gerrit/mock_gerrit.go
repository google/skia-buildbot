package gerrit

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/buildbucket"
)

// Mock that implements all methods of GerritInterface.
type MockedGerrit struct {
	mock.Mock
	IssueID int64
}

func (g *MockedGerrit) Initialized() bool {
	return true
}
func (g *MockedGerrit) TurnOnAuthenticatedGets() {
}
func (g *MockedGerrit) Url(issueID int64) string {
	return ""
}
func (g *MockedGerrit) GetUserEmail() (string, error) {
	return "", nil
}
func (g *MockedGerrit) GetRepoUrl() string {
	return ""
}
func (g *MockedGerrit) ExtractIssueFromCommit(commitMsg string) (int64, error) {
	return 0, nil
}
func (g *MockedGerrit) GetIssueProperties(issue int64) (*ChangeInfo, error) {
	return &ChangeInfo{Issue: issue}, nil
}
func (g *MockedGerrit) GetPatch(issue int64, revision string) (string, error) {
	return "", nil
}
func (g *MockedGerrit) SetReview(issue *ChangeInfo, message string, labels map[string]interface{}, reviewers []string) error {
	return nil
}
func (g *MockedGerrit) AddComment(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SendToDryRun(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SendToCQ(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) RemoveFromCQ(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) Approve(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) NoScore(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) DisApprove(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) Abandon(issue *ChangeInfo, message string) error {
	return nil
}
func (g *MockedGerrit) SetReadyForReview(issue *ChangeInfo) error {
	return nil
}
func (g *MockedGerrit) SetTopic(topic string, changeNum int64) error {
	return nil
}
func (g *MockedGerrit) Search(limit int, terms ...*SearchTerm) ([]*ChangeInfo, error) {
	results := make([]*ChangeInfo, 0)
	results = append(results, &ChangeInfo{Issue: g.IssueID})
	return results, nil
}
func (g *MockedGerrit) GetTrybotResults(ctx context.Context, issueID int64, patchsetID int64) ([]*buildbucket.Build, error) {
	return nil, nil
}

func (g *MockedGerrit) Files(issue int64, patch string) (map[string]*FileInfo, error) {
	return nil, nil
}

func (g *MockedGerrit) GetFileNames(issue int64, patch string) ([]string, error) {
	return nil, nil
}

func (g *MockedGerrit) IsBinaryPatch(issue int64, patch string) (bool, error) {
	return false, nil
}

func (g *MockedGerrit) CreateChange(string, string, string, string) (*ChangeInfo, error) {
	return nil, nil
}

func (g *MockedGerrit) EditFile(ci *ChangeInfo, filepath, content string) error {
	return nil
}

func (g *MockedGerrit) MoveFile(ci *ChangeInfo, oldPath, newPath string) error {
	return nil
}

func (g *MockedGerrit) DeleteFile(ci *ChangeInfo, filepath string) error {
	return nil
}

func (g *MockedGerrit) SetCommitMessage(ci *ChangeInfo, msg string) error {
	return nil
}

func (g *MockedGerrit) PublishChangeEdit(ci *ChangeInfo) error {
	return nil
}

func (g *MockedGerrit) DeleteChangeEdit(ci *ChangeInfo) error {
	return nil
}

// Make sure MockGerrit fulfills GerritInterface
var _ GerritInterface = (*MockedGerrit)(nil)
