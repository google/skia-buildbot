package gerrit_tryjob_monitor

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/tryjobstore"
)

// GerritTryjobMonitor offers a higher level api to handle tryjob-related tasks on top
// of the tryjobstore package.
type GerritTryjobMonitor struct {
	expStore             expstorage.ExpectationsStore
	issueExpStoreFactory expstorage.IssueExpStoreFactory
	gerritAPI            gerrit.GerritInterface
	tryjobStore          tryjobstore.TryjobStore
	siteURL              string
	eventBus             eventbus.EventBus
	writeGerritMonitor   *util.CondMonitor
	isAuthoritative      bool
}

// New creates a new instance of GerritTryjobMonitor.
// siteURL is URL under which the current site it served. It is used to
// generate URLs that are written to Gerrit CLs.
func New(tryjobStore tryjobstore.TryjobStore, expStore expstorage.ExpectationsStore, iesFactory expstorage.IssueExpStoreFactory, gerritAPI gerrit.GerritInterface, siteURL string, eventBus eventbus.EventBus, isAuthoritative bool) *GerritTryjobMonitor {
	ret := &GerritTryjobMonitor{
		expStore:             expStore,
		issueExpStoreFactory: iesFactory,
		tryjobStore:          tryjobStore,
		gerritAPI:            gerritAPI,
		siteURL:              strings.TrimRight(siteURL, "/"),
		eventBus:             eventBus,
		writeGerritMonitor:   util.NewCondMonitor(1),
		isAuthoritative:      isAuthoritative,
	}

	// Subscribe to events that a tryjob has been updated.
	eventBus.SubscribeAsync(tryjobstore.EV_TRYJOB_UPDATED, ret.handleTryjobUpdate)
	return ret
}

// ForceRefresh implements the TryjobMonitor interface.
func (t *GerritTryjobMonitor) ForceRefresh(issueID int64) error {
	// Load the issue from the database
	issue, err := t.tryjobStore.GetIssue(issueID, false)
	if err != nil {
		return sklog.FmtErrorf("Error loading issue %d: %s", issueID, err)
	}

	if !issue.Committed {
		// Check if the issue has been merged and find the commit if necessary.
		changeInfo, err := t.gerritAPI.GetIssueProperties(context.TODO(), issueID)
		if err != nil {
			return skerr.Fmt("Error retrieving Gerrit issue %d: %s", issueID, err)
		}

		if changeInfo.Status == gerrit.CHANGE_STATUS_MERGED {
			if err := t.CommitIssueBaseline(issueID, issue.Owner); err != nil {
				return err
			}
			sklog.Infof("Issue %d expecations have been added to master expecations.", issueID)
		}
	}

	// TODO(stephan): This should also sync with the Gerrit issue and update
	// anything that might need to be updated for a Gerrit CL.
	return t.WriteGoldLinkAsComment(issueID)
}

// WriteGoldLinkAsComment implements the TryjobMonitor interface.
func (t *GerritTryjobMonitor) WriteGoldLinkAsComment(issueID int64) error {
	// Make sure this instance is allowed to write the Gerrit comment.
	if !t.isAuthoritative {
		sklog.Info("Not writing gold link because configured not to.")
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

	gerritIssue, err := t.gerritAPI.GetIssueProperties(context.TODO(), issueID)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving Gerrit issue %d: %s", issueID, err)
	}

	if err := t.gerritAPI.AddComment(context.TODO(), gerritIssue, t.getGerritMsg(issueID)); err != nil {
		return sklog.FmtErrorf("Error adding Gerrit comment to issue %d: %s", issueID, err)
	}

	// Write the updated issue to the datastore.
	return t.tryjobStore.UpdateIssue(issue, func(data interface{}) interface{} {
		issue := data.(*tryjobstore.Issue)
		issue.CommentAdded = true
		return issue
	})
}

// CommitIssueBaseline commits the expectations for the given issue to the master baseline.
func (t *GerritTryjobMonitor) CommitIssueBaseline(issueID int64, user string) error {
	// Get the issue expecations.
	issueExpStore := t.issueExpStoreFactory(issueID)
	issueChanges, err := issueExpStore.Get()
	if err != nil {
		return sklog.FmtErrorf("Unable to retrieve expecations for issue %d: %s", issueID, err)
	}
	if len(issueChanges) == 0 {
		return nil
	}

	if user == "" {
		user = "syntheticUser"
	}

	syntheticUser := fmt.Sprintf("%s:%d", user, issueID)

	commitFn := func() error {
		if err := t.expStore.AddChange(context.TODO(), issueChanges, syntheticUser); err != nil {
			return skerr.Fmt("Unable to add expectations for issue %d: %s", issueID, err)
		}
		return nil
	}

	return t.tryjobStore.CommitIssueExp(issueID, commitFn)
}

// getGerritMsg returns the message that should be added as a comment to the Gerrit CL.
func (t *GerritTryjobMonitor) getGerritMsg(issueID int64) string {
	const (
		goldMessageTmpl = "Gold results for tryjobs are being ingested.\nSee image differences at: %s"
		urlTmpl         = "%s/search?issue=%d"
	)
	url := fmt.Sprintf(urlTmpl, t.siteURL, issueID)
	return fmt.Sprintf(goldMessageTmpl, url)
}

// handleTryjobUpdate is triggered when a Tryjob is updated by the ingester.
func (t *GerritTryjobMonitor) handleTryjobUpdate(data interface{}) {
	tryjob := data.(*tryjobstore.Tryjob)
	if err := t.WriteGoldLinkAsComment(tryjob.IssueID); err != nil {
		sklog.Errorf("Error adding comment to Gerrit CL: %s", err)
	}
}
