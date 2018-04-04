package expstorage

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

type cloudExpStore struct {
	issueID int64
	client  *datastore.Client
}

func NewCloudExpStore(client *datastore.Client) (ExpectationsStore, error) {
	if client == nil {
		return nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	return &cloudExpStore{
		client: client,
	}, nil
}

func NewIssueExpStore(client *datastore.Client) {}

// Get the current classifications for image digests. The keys of the
// expectations map are the test names.
func (c *cloudExpStore) Get() (exp *Expectations, err error) {
	// Get all expectation changes and iterate over them updating the result.
	expChangeKeys, _, err := c.getExpChangesForIssue(issueID, -1, -1, true)
	if err != nil {
		return nil, err
	}

	testChanges := make([][]*TestDigestExp, len(expChangeKeys), len(expChangeKeys))
	// Iterate over the expectations build the expectations.
	var egroup errgroup.Group
	for idx, key := range expChangeKeys {
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				_, testChanges[idx], err = c.getTestDigestExps(key, false)
				return err
			})
		}(idx, key)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := NewExpectations()
	for _, expByChange := range testChanges {
		if len(expByChange) > 0 {
			for _, oneChange := range expByChange {
				ret.SetTestExpectation(oneChange.Name, oneChange.Digest, types.LabelFromString(oneChange.Label))
			}
		}
	}

	return ret, nil
}

// AddChange writes the given classified digests to the database and records the
// user that made the change.
func (c *cloudExpStore) AddChange(changes map[string]types.TestClassification, userId string) error {
	// Write the change record.
	ctx := context.Background()
	expChange := &ExpChange{
		IssueID:   issueID,
		UserID:    userID,
		TimeStamp: util.TimeStamp(time.Millisecond),
	}

	var changeKey *datastore.Key
	if changeKey, err = c.client.Put(ctx, ds.NewKey(ds.TRYJOB_EXP_CHANGE), expChange); err != nil {
		return err
	}

	// If we have an error later make sure to delete change record.
	defer func() {
		if err != nil {
			go func() {
				if err := c.deleteExpChanges([]*datastore.Key{changeKey}); err != nil {
					sklog.Errorf("Error deleting expectation change %s: %s", changeKey.String(), err)
				}
			}()
		}
	}()

	// Insert all the expectation changes.
	testChanges := make([]*TestDigestExp, 0, len(changes))
	for testName, classification := range changes {
		for digest, label := range classification {
			testChanges = append(testChanges, &TestDigestExp{
				Name:   testName,
				Digest: digest,
				Label:  label.String(),
			})
		}
	}

	tdeKeys := make([]*datastore.Key, len(testChanges), len(testChanges))
	for idx := range testChanges {
		key := ds.NewKey(ds.TEST_DIGEST_EXP)
		key.Parent = changeKey
		tdeKeys[idx] = key
	}

	if _, err = c.client.PutMulti(ctx, tdeKeys, testChanges); err != nil {
		return err
	}

	// Mark the expectation change as valid.
	expChange.OK = true
	if _, err = c.client.Put(ctx, changeKey, expChange); err != nil {
		return err
	}

	if c.eventBus != nil {
		c.eventBus.Publish(EV_TRYJOB_EXP_CHANGED, &IssueExpChange{IssueID: issueID}, false)
	}

	return nil
}

// QueryLog allows to paginate through the changes in the expectations.
// If details is true the result will include a list of triage operations
// that were part a change.
func (c *cloudExpStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	return nil, 0, nil
}

// UndoChange reverts a change by setting all testname/digest pairs of the
// original change to the label they had before the change was applied.
// A new entry is added to the log with a reference to the change that was
// undone.
func (c *cloudExpStore) UndoChange(changeID int, userID string) (map[string]types.TestClassification, error) {
	return nil, nil
}

// removeChange removes the given digests from the expectations store.
// The key in changes is the test name which maps to a list of digests
// to remove. Used for testing only.
func (c *cloudExpStore) removeChange(changes map[string]types.TestClassification) error {
	return nil
}

// ExpChange is used to store an expectation change in the database. Each
// expecation change is an atomic change to expectations for an issue.
// The actualy expecations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64
	Count        int64
	UndoChangeID int64
	OK           bool
}

// TestDigestExp is used to store expectations for an issue in the database.
// Each entity is a child of instance of ExpChange. It captures the expectation
// of one Test/Digest pair.
type TestDigestExp struct {
	Name   string
	Digest string
	Label  string
}
