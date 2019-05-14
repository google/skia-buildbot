package tryjobs

type TryjobMonitor interface {
	// ForceRefresh forces a refresh of given CL.
	ForceRefresh(issueID int64) error

	// WriteGoldLinkAsComment comments on the CL with a link to Gold showing untriaged images.
	// It uses the tryjob store to ensure that the message is only added to the CL once.
	WriteGoldLinkAsComment(issueID int64) error

	// CommitIssueBaseline commits the expectations for the given issue to the master baseline.
	CommitIssueBaseline(issueID int64, user string) error
}
