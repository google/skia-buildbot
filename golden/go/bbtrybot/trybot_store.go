package bbtrybot

type TrybotStore interface {
	ListTrybotIssues(offset, size int) ([]*Issue, int, error)
	GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error)
	Add(details *IssueDetails) error
}
