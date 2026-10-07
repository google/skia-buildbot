package db

import (
	"context"
	"net/url"
	"sort"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	appForFirestore = "autogardener"

	collectionSummaryForTask = "summary-for-task"
	collectionReport         = "report-for-repo-and-branch"
	collectionFailureClass   = "failure-class"

	defaultAttempts = 3
	defaultTimeout  = 10 * time.Second
)

type firestoreDB struct {
	client *firestore.Client
}

func NewFirestoreDB(ctx context.Context, project, instance string, opts ...option.ClientOption) (AutoGardenerDB, error) {
	client, err := firestore.NewClient(ctx, project, appForFirestore, instance, opts...)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &firestoreDB{
		client: client,
	}, nil
}

func (d *firestoreDB) GetTaskSummary(ctx context.Context, taskID string) (*types.TaskSummary, error) {
	doc, err := d.client.Get(ctx, d.client.Collection(collectionSummaryForTask).Doc(taskID), defaultAttempts, defaultTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rv types.TaskSummary
	if err := doc.DataTo(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &rv, nil
}

func (d *firestoreDB) GetUnclassifiedTaskSummaries(ctx context.Context, limit int) (map[string]*types.TaskSummary, error) {
	it := d.client.Collection(collectionSummaryForTask).
		Where("FailureClassId", "==", "").
		Limit(limit).
		Documents(ctx)
	defer it.Stop()
	rv := make(map[string]*types.TaskSummary, limit)
	for {
		doc, err := it.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, skerr.Wrap(err)
		}
		taskSummary := new(types.TaskSummary)
		if err := doc.DataTo(taskSummary); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv[doc.Ref.ID] = taskSummary
	}
	return rv, nil
}

func (d *firestoreDB) PutTaskSummary(ctx context.Context, taskID string, summary *types.TaskSummary) error {
	_, err := d.client.Set(ctx, d.client.Collection(collectionSummaryForTask).Doc(taskID), summary, defaultAttempts, defaultTimeout)
	return skerr.Wrap(err)
}

func (d *firestoreDB) docForReport(repo, branch string) *fs.DocumentRef {
	docID := url.PathEscape(repo) + "_" + url.PathEscape(branch)
	return d.client.Collection(collectionReport).Doc(docID)
}

func (d *firestoreDB) GetReport(ctx context.Context, repo, branch string) (*types.Report, error) {
	doc, err := d.client.Get(ctx, d.docForReport(repo, branch), defaultAttempts, defaultTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rv types.Report
	if err := doc.DataTo(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return &rv, nil
}

func (d *firestoreDB) PutReport(ctx context.Context, repo, branch string, report *types.Report) error {
	_, err := d.client.Set(ctx, d.docForReport(repo, branch), report, defaultAttempts, defaultTimeout)
	return skerr.Wrap(err)
}

func fixLegacyFailureClass(fc *types.FailureClass) {
	if len(fc.Updates) == 0 && (fc.LegacyErrorMessage != nil || fc.LegacyAnalysis != nil) {
		fc.Updates = []*types.FailureClassUpdate{
			{
				User:         "autogardener",
				ErrorMessage: fc.LegacyErrorMessage,
				Analysis:     fc.LegacyAnalysis,
			},
		}
	}
	fc.LegacyErrorMessage = nil
	fc.LegacyAnalysis = nil
}

func decodeFailureClass(doc *fs.DocumentSnapshot) (*types.FailureClass, error) {
	var fc types.FailureClass
	if err := doc.DataTo(&fc); err != nil {
		return nil, skerr.Wrap(err)
	}
	fixLegacyFailureClass(&fc)
	return &fc, nil
}

func (d *firestoreDB) GetFailureClass(ctx context.Context, id string) (*types.FailureClass, error) {
	doc, err := d.client.Get(ctx, d.client.Collection(collectionFailureClass).Doc(id), defaultAttempts, defaultTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}
	return decodeFailureClass(doc)
}

func (d *firestoreDB) PutFailureClass(ctx context.Context, fc *types.FailureClass) error {
	fc.LastSeen = firestore.FixTimestamp(fc.LastSeen)
	for _, u := range fc.Updates {
		u.Timestamp = firestore.FixTimestamp(u.Timestamp)
	}
	ref := d.client.Collection(collectionFailureClass).Doc(fc.Id)
	var merged types.FailureClass
	err := d.client.RunTransaction(ctx, "PutFailureClass", fc.Id, defaultAttempts, defaultTimeout, func(ctx context.Context, tx *fs.Transaction) error {
		merged = *fc
		doc, err := tx.Get(ref)
		if err != nil {
			if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
				return err
			}
			merged.Updates = mergeFailureClassUpdates(nil, fc.Updates)
		} else {
			existing, err := decodeFailureClass(doc)
			if err != nil {
				return err
			}
			if existing.LastSeen.After(merged.LastSeen) {
				merged.LastSeen = existing.LastSeen
			}
			merged.Updates = mergeFailureClassUpdates(existing.Updates, fc.Updates)
		}

		// Ensure that there are no cycles in DuplicateOf.
		visited := map[string]bool{merged.Id: true}
		for currID := merged.DuplicateOf(); currID != ""; {
			if visited[currID] {
				return skerr.Fmt("cycle detected in DuplicateOf for FailureClass %s", merged.Id)
			}
			visited[currID] = true
			targetDoc, err := tx.Get(d.client.Collection(collectionFailureClass).Doc(currID))
			if err != nil {
				if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
					break
				}
				return err
			}
			targetFC, err := decodeFailureClass(targetDoc)
			if err != nil {
				return err
			}
			currID = targetFC.DuplicateOf()
		}
		for _, dupID := range merged.Duplicates() {
			if visited[dupID] {
				return skerr.Fmt("cycle detected in Duplicates for FailureClass %s", merged.Id)
			}
		}
		return tx.Set(ref, &merged)
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	*fc = merged
	return nil
}

type updateKey struct {
	ts   int64
	user string
}

func makeUpdateKey(u *types.FailureClassUpdate) updateKey {
	return updateKey{
		ts:   firestore.FixTimestamp(u.Timestamp).UnixNano(),
		user: u.User,
	}
}

func mergeFailureClassUpdates(existing, incoming []*types.FailureClassUpdate) []*types.FailureClassUpdate {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	byKey := make(map[updateKey]*types.FailureClassUpdate, len(existing)+len(incoming))
	for _, u := range existing {
		cp := *u
		cp.Timestamp = firestore.FixTimestamp(cp.Timestamp)
		byKey[makeUpdateKey(&cp)] = &cp
	}
	for _, u := range incoming {
		cp := *u
		cp.Timestamp = firestore.FixTimestamp(cp.Timestamp)
		byKey[makeUpdateKey(&cp)] = &cp
	}
	rv := make([]*types.FailureClassUpdate, 0, len(byKey))
	for _, u := range byKey {
		rv = append(rv, u)
	}
	sort.Slice(rv, func(i, j int) bool {
		if rv[i].Timestamp.Equal(rv[j].Timestamp) {
			return rv[i].User < rv[j].User
		}
		return rv[i].Timestamp.Before(rv[j].Timestamp)
	})
	return rv
}

func (d *firestoreDB) GetRecentFailureClasses(ctx context.Context, repo string, since time.Time, limit int) ([]*types.FailureClass, error) {
	query := d.client.Collection(collectionFailureClass).
		Where("Repo", "==", repo).
		Where("LastSeen", ">=", since).
		OrderBy("LastSeen", fs.Desc)
	if limit > 0 {
		query = query.Limit(limit)
	}

	var results []*types.FailureClass
	err := d.client.IterDocs(ctx, "GetRecentFailureClasses", "", query, defaultAttempts, defaultTimeout, func(doc *fs.DocumentSnapshot) error {
		fc, err := decodeFailureClass(doc)
		if err != nil {
			return err
		}
		results = append(results, fc)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return FetchLinkedFailureClasses(ctx, results, d.GetFailureClass)
}

func (d *firestoreDB) ModifiedFailureClassesCh(ctx context.Context, window time.Duration) <-chan []*types.FailureClass {
	outCh := make(chan []*types.FailureClass)
	makeQuery := func() fs.Query {
		return d.client.Collection(collectionFailureClass).Where("LastSeen", ">=", now.Now(ctx).Add(-window))
	}
	go func() {
		defer close(outCh)
		for changes := range firestore.QuerySnapshotChannel[types.FailureClass](ctx, makeQuery) {
			fcs := append(changes.Added, changes.Modified...)
			if len(fcs) > 0 {
				for _, fc := range fcs {
					fixLegacyFailureClass(fc)
				}
				outCh <- fcs
			}
		}
	}()
	return outCh
}

var _ AutoGardenerDB = &firestoreDB{}
