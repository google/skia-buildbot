package tryjobs

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
)

const (
	// urlTmpl is the template to create the URL that will show image differences caused by tryjob runs.
	urlTmpl = "%s/search?issue=%d"

	// goldMessageTmpl is the text that is written as a comment to Gerrit to provide information
	// that data is begin ingested and where to see the results.
	goldMessageTmpl = "Gold results for tryjobs are being ingested. See image differences at: %s"
)

// TryjobMonitor offers a higher level api to handle tryjob-related tasks on top
// of the tryjobstore package.
type TryjobMonitor struct {
	gerritAPI   gerrit.GerritInterface
	tryjobStore tryjobstore.TryjobStore
	siteURL     string
	eventBus    eventbus.EventBus
}

// NewTryjobMonitor creates a new instance of TryjobMonitor.
// siteURL is URL under which the current site it served. It is used to
// generate URLs that are written to Gerrit CLs.
func NewTryjobMonitor(tryjobStore tryjobstore.TryjobStore, gerritAPI gerrit.GerritInterface, siteURL string, eventBus eventbus.EventBus) *TryjobMonitor {
	ret := &TryjobMonitor{
		tryjobStore: tryjobStore,
		gerritAPI:   gerritAPI,
		siteURL:     strings.TrimRight(siteURL, "/"),
		eventBus:    eventBus,
	}

	// Subscribe to events that a tryjob has been updated.
	eventBus.SubscribeAsync(tryjobstore.EV_TRYJOB_UPDATED, ret.handleTryjobUpdate)
	return ret
}

// ForceRefresh forces a refresh of given Gerrit issue.
func (t *TryjobMonitor) ForceRefresh(issueID int64) error {
	return t.WriteGoldLinkToGerrit(issueID)
}

// Write link to the GoldIssue to Gerrit and mark it in the datastore as written.
func (t *TryjobMonitor) WriteGoldLinkToGerrit(issueID int64) error {
	// TODO remove !!!!
	if issueID != 36330 {
		return nil
	}

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
		// Check for the unlikely case that the issue has been updated since we first read it.
		if issue.CommentAdded {
			return nil
		}
		issue.CommentAdded = true
		return issue
	})
}

// getGerritMsg returns the message that should be added as a comment to the Gerrit CL.
func (t *TryjobMonitor) getGerritMsg(issueID int64) string {
	url := fmt.Sprintf(urlTmpl, t.siteURL, issueID)
	return fmt.Sprintf(goldMessageTmpl, url)
}

// handleTryjobUpdate is triggered when a Tryjob is updated by the ingester.
func (t *TryjobMonitor) handleTryjobUpdate(data interface{}) {
	tryjob := data.(*tryjobstore.Tryjob)
	if err := t.WriteGoldLinkToGerrit(tryjob.IssueID); err != nil {
		sklog.Errorf("Error adding comment to Gerrit CL: %s", err)
		return
	}
	// TODO remove
	sklog.Infof("\n\n\nGerrit link written without error !!\n\n\n")
}
