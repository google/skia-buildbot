package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
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
func (g *SimpleGerritInterface) Config() *gerrit.Config {
	args := g.Called()
	return args.Get(0).(*gerrit.Config)
}
func (g *SimpleGerritInterface) TurnOnAuthenticatedGets() {
}
func (g *SimpleGerritInterface) Url(issueID int64) string {
	return ""
}
func (g *SimpleGerritInterface) GetUserEmail(context.Context) (string, error) {
	return "", nil
}
func (g *SimpleGerritInterface) GetRepoUrl() string {
	return ""
}
func (g *SimpleGerritInterface) ExtractIssueFromCommit(commitMsg string) (int64, error) {
	return 0, nil
}
func (g *SimpleGerritInterface) GetIssueProperties(ctx context.Context, issue int64) (*gerrit.ChangeInfo, error) {
	return &gerrit.ChangeInfo{Issue: issue}, nil
}
func (g *SimpleGerritInterface) GetPatch(ctx context.Context, issue int64, revision string) (string, error) {
	return "", nil
}
func (g *SimpleGerritInterface) SetReview(ctx context.Context, issue *gerrit.ChangeInfo, message string, labels map[string]int, reviewers []string) error {
	return nil
}
func (g *SimpleGerritInterface) AddComment(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) AddCC(ctx context.Context, issue *gerrit.ChangeInfo, ccList []string) error {
	return nil
}
func (g *SimpleGerritInterface) SendToDryRun(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) SendToCQ(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) RemoveFromCQ(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) Approve(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) NoScore(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) DisApprove(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) DownloadCommitMsgHook(ctx context.Context, dest string) error {
	return nil
}
func (g *SimpleGerritInterface) SelfApprove(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) Abandon(ctx context.Context, issue *gerrit.ChangeInfo, message string) error {
	return nil
}
func (g *SimpleGerritInterface) SetReadyForReview(ctx context.Context, issue *gerrit.ChangeInfo) error {
	return nil
}
func (g *SimpleGerritInterface) SetTopic(ctx context.Context, topic string, changeNum int64) error {
	return nil
}
func (g *SimpleGerritInterface) Search(ctx context.Context, limit int, sortResults bool, terms ...*gerrit.SearchTerm) ([]*gerrit.ChangeInfo, error) {
	results := make([]*gerrit.ChangeInfo, 0)
	results = append(results, &gerrit.ChangeInfo{Issue: g.IssueID})
	return results, nil
}
func (g *SimpleGerritInterface) GetTrybotResults(ctx context.Context, issueID int64, patchsetID int64) ([]*buildbucketpb.Build, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) Files(ctx context.Context, issue int64, patch string) (map[string]*gerrit.FileInfo, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) GetFileNames(ctx context.Context, issue int64, patch string) ([]string, error) {
	return nil, nil
}
func (g *SimpleGerritInterface) GetChange(ctx context.Context, id string) (*gerrit.ChangeInfo, error) {
	return nil, nil
}
func (g *SimpleGerritInterface) IsBinaryPatch(ctx context.Context, issue int64, patch string) (bool, error) {
	return false, nil
}

func (g *SimpleGerritInterface) CreateChange(context.Context, string, string, string, string) (*gerrit.ChangeInfo, error) {
	return nil, nil
}

func (g *SimpleGerritInterface) EditFile(ctx context.Context, ci *gerrit.ChangeInfo, filepath, content string) error {
	return nil
}

func (g *SimpleGerritInterface) MoveFile(ctx context.Context, ci *gerrit.ChangeInfo, oldPath, newPath string) error {
	return nil
}

func (g *SimpleGerritInterface) DeleteFile(ctx context.Context, ci *gerrit.ChangeInfo, filepath string) error {
	return nil
}

func (g *SimpleGerritInterface) SetCommitMessage(ctx context.Context, ci *gerrit.ChangeInfo, msg string) error {
	return nil
}

func (g *SimpleGerritInterface) PublishChangeEdit(ctx context.Context, ci *gerrit.ChangeInfo) error {
	return nil
}

func (g *SimpleGerritInterface) DeleteChangeEdit(ctx context.Context, ci *gerrit.ChangeInfo) error {
	return nil
}

func (g *SimpleGerritInterface) Submit(ctx context.Context, ci *gerrit.ChangeInfo) error {
	return nil
}

// Make sure MockGerrit fulfills GerritInterface
var _ gerrit.GerritInterface = (*SimpleGerritInterface)(nil)
