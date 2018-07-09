package tryjobs

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/tryjobstore"
)

// TryjobMonitor offers a higher level api to handle tryjob-related tasks on top
// of the tryjobstore package.
type TryjobMonitor struct {
	gerritAPI          gerrit.GerritInterface
	tryjobStore        tryjobstore.TryjobStore
	siteURL            string
	eventBus           eventbus.EventBus
	writeGerritMonitor *util.CondMonitor
	isAuthoritative    bool
}

// NewTryjobMonitor creates a new instance of TryjobMonitor.
// siteURL is URL under which the current site it served. It is used to
// generate URLs that are written to Gerrit CLs.
func NewTryjobMonitor(tryjobStore tryjobstore.TryjobStore, gerritAPI gerrit.GerritInterface, siteURL string, eventBus eventbus.EventBus, isAuthoritative bool) *TryjobMonitor {
	ret := &TryjobMonitor{
		tryjobStore:        tryjobStore,
		gerritAPI:          gerritAPI,
		siteURL:            strings.TrimRight(siteURL, "/"),
		eventBus:           eventBus,
		writeGerritMonitor: util.NewCondMonitor(1),
		isAuthoritative:    isAuthoritative,
	}

	// Subscribe to events that a tryjob has been updated.
	eventBus.SubscribeAsync(tryjobstore.EV_TRYJOB_UPDATED, ret.handleTryjobUpdate)
	return ret
}

// ForceRefresh forces a refresh of given Gerrit issue.
func (t *TryjobMonitor) ForceRefresh(issueID int64) error {
	// TODO(stephan): This should also sync with the Gerrit issue and update
	// anything that might need to be updated for a Gerrit CL.
	return t.WriteGoldLinkToGerrit(issueID)
}

// WriteGoldLinkToGerrit write a link to the Gerrit CL referenced by issueID.
// It uses the tryjob store to ensure that the message is only added to the CL once.
func (t *TryjobMonitor) WriteGoldLinkToGerrit(issueID int64) error {
	// Make sure this instance is allowed to write the Gerrit comment.
	if !t.isAuthoritative {
		return nil
	}

	// Only one thread per issueID can enter at a time.
	defer t.writeGerritMonitor.Enter(issueID).Release()

	// Load the issue from the database
	issue, err := t.tryjobStore.GetIssue(issueID, false)
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

	gerritIssue, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving Gerrit issue %d: %s", issueID, err)
	}

	if err := t.gerritAPI.AddComment(gerritIssue, t.getGerritMsg(issueID)); err != nil {
		return sklog.FmtErrorf("Error adding Gerrit comment to issue %d: %s", issueID, err)
	}

	// Write the updated issue to the datastore.
	return t.tryjobStore.UpdateIssue(issue, func(data interface{}) interface{} {
		issue := data.(*tryjobstore.Issue)
		issue.CommentAdded = true
		return issue
	})
}

// getGerritMsg returns the message that should be added as a comment to the Gerrit CL.
func (t *TryjobMonitor) getGerritMsg(issueID int64) string {
	const (
		goldMessageTmpl = "Gold results for tryjobs are being ingested. See image differences at: %s"
		urlTmpl         = "%s/search?issue=%d"
	)
	url := fmt.Sprintf(urlTmpl, t.siteURL, issueID)
	return fmt.Sprintf(goldMessageTmpl, url)
}

// handleTryjobUpdate is triggered when a Tryjob is updated by the ingester.
func (t *TryjobMonitor) handleTryjobUpdate(data interface{}) {
	tryjob := data.(*tryjobstore.Tryjob)
	if err := t.WriteGoldLinkToGerrit(tryjob.IssueID); err != nil {
		sklog.Errorf("Error adding comment to Gerrit CL: %s", err)
	}
}
