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
	urlTmpl         = "%s/search?issue=%d"
	goldMessageTmpl = "Gold results for this tryjob are being ingested. See image differences at: %s"
)

type TryjobMonitor struct {
	gerritAPI   *gerrit.Gerrit
	tryjobStore tryjobstore.TryjobStore
	siteURL     string
	eventBus    eventbus.EventBus
}

func NewTryjobMonitor(tryjobStore tryjobstore.TryjobStore, gerritAPI *gerrit.Gerrit, siteURL string, eventBus eventbus.EventBus) *TryjobMonitor {
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

func (t *TryjobMonitor) handleTryjobUpdate(data interface{}) {
	tryjob := data.(*tryjobstore.Tryjob)
	if err := t.WriteGoldLinkToGerrit(tryjob.IssueID); err != nil {
		sklog.Errorf("Error adding comment to Gerrit CL: %s", err)
	}
}

// Write link to the GoldIssue to Gerrit and mark it in the datastore as written.
func (t *TryjobMonitor) WriteGoldLinkToGerrit(issueID int64) error {
	// Load the issue from the database
	issue, err := t.tryjobStore.GetIssue(issueID, false)
	if err != nil {
		return sklog.FmtErrorf("Error loading issue %d: %s", issueID, err)
	}

	// If the issue doesn't exist we return an error
	if issue == nil {
		return sklog.FmtErrorf("Issue %d does not exist")
	}

	// If it's already been added we are done
	if issue.CommentAdded {
		return nil
	}

	gerritIssue, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		sklog.FmtErrorf("Error retrieving Gerrit issue %d: %s", issueID, err)
	}

	url := fmt.Sprintf(urlTmpl, t.siteURL, issueID)
	msg := fmt.Sprintf(goldMessageTmpl, url)
	if err := t.gerritAPI.AddComment(gerritIssue, msg); err != nil {
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
