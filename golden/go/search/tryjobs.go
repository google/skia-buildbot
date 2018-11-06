package search

import (
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tryjobstore"
)

// TryjobMonitor offers a higher level api to handle tryjob-related tasks on top
// of the tryjobstore package.
type TryjobMonitor struct {
	storages           *storage.Storage
	writeGerritMonitor *util.CondMonitor
}

// NewTryjobMonitor creates a new instance of TryjobMonitor.
// siteURL is URL under which the current site it served. It is used to
// generate URLs that are written to Gerrit CLs.
func NewTryjobMonitor(storages *storage.Storage) *TryjobMonitor {
	ret := &TryjobMonitor{
		storages:           storages,
		writeGerritMonitor: util.NewCondMonitor(1),
	}

	// Subscribe to events that a tryjob has been updated.
	storages.EventBus.SubscribeAsync(tryjobstore.EV_TRYJOB_UPDATED, ret.handleTryjobUpdate)
	return ret
}

// refreshIssue forces a refresh of given Gerrit issue.
func (t *TryjobMonitor) refreshIssue(issueID int64) error {
	// TODO(stephan): This should also sync with the Gerrit issue and update
	// anything that might need to be updated for a Gerrit CL.
	return t.writeGoldLinkToGerrit(issueID)
}

// writeGoldLinkToGerrit write a link to the Gerrit CL referenced by issueID.
// It uses the tryjob store to ensure that the message is only added to the CL once.
func (t *TryjobMonitor) writeGoldLinkToGerrit(issueID int64) error {
	// Make sure this instance is allowed to write the Gerrit comment.
	if !t.storages.IsAuthoritative {
		return nil
	}

	// Only one thread per issueID can enter at a time.
	defer t.writeGerritMonitor.Enter(issueID).Release()

	// Load the issue from the database
	issue, err := t.storages.TryjobStore.GetIssue(issueID, false)
	if err != nil {
		return sklog.FmtErrorf("Error loading issue %d: %s", issueID, err)
	}

	// If the issue doesn't exist we return an error
	if issue == nil {
		return sklog.FmtErrorf("Issue %d does not exist", issueID)
	}

	// If it's already been added we are done
	if issue.CommentAdded {
		return nil
	}

	gerritIssue, err := t.storages.GerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving Gerrit issue %d: %s", issueID, err)
	}

	if err := t.storages.GerritAPI.AddComment(gerritIssue, t.getGerritMsg(issueID)); err != nil {
		return sklog.FmtErrorf("Error adding Gerrit comment to issue %d: %s", issueID, err)
	}

	// Write the updated issue to the datastore.
	return t.storages.TryjobStore.UpdateIssue(issue, func(data interface{}) interface{} {
		issue := data.(*tryjobstore.Issue)
		issue.CommentAdded = true
		return issue
	})
}

// getGerritMsg returns the message that should be added as a comment to the Gerrit CL.
func (t *TryjobMonitor) getGerritMsg(issueID int64) string {
	const (
		goldMessageTmpl = "Gold results for tryjobs are being ingested.\nSee image differences at: %s"
		urlTmpl         = "%s/search?issue=%d"
	)
	url := fmt.Sprintf(urlTmpl, t.storages.SiteURL, issueID)
	return fmt.Sprintf(goldMessageTmpl, url)
}

// handleTryjobUpdate is triggered when a Tryjob is updated by the ingester.
func (t *TryjobMonitor) handleTryjobUpdate(data interface{}) {
	// Extract the tryjob information.
	tryjob := data.(*tryjobstore.Tryjob)

	// Write the link to Gold to Gerrit as a comment.
	if err := t.writeGoldLinkToGerrit(tryjob.IssueID); err != nil {
		sklog.Errorf("Error adding comment to Gerrit CL: %s", err)
	}

	// Update the "hot" tryjobs with this one.
	if err := t.updateTryjobResults(tryjob.IssueID, tryjob.PatchsetID, tryjob.BuildBucketID); err != nil {
		sklog.Errorf("Error updating tryjob results: %s", err)
	}
}

func (t *TryjobMonitor) updateTryjobResults(issueID, patchsetID, buildBucketID int64) error {
	// Refresh the current tryjob and updata the cache.
	return nil
}
