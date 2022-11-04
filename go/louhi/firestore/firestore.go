package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	fs "cloud.google.com/go/firestore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/louhi"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	collectionExecutions = "executions"
	collectionFlows      = "flows"
	defaultAttempts      = 3
	defaultTimeout       = 10 * time.Second
)

// FirestoreDB is a louhi.DB implementation backed by Firestore.
type FirestoreDB struct {
	client *firestore.Client
	flows  *fs.CollectionRef
}

// NewDB returns a louhi.DB implementation backed by Firestore.
func NewDB(ctx context.Context, project, app, instance string) (*FirestoreDB, error) {
	ts, err := google.DefaultTokenSource(ctx, datastore.ScopeDatastore)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create TokenSource")
	}
	client, err := firestore.NewClient(ctx, project, app, instance, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create firestore client")
	}
	return &FirestoreDB{
		client: client,
		flows:  client.Collection(collectionFlows),
	}, nil
}

// PutFlowExecution implements DB.
func (db *FirestoreDB) PutFlowExecution(ctx context.Context, flow *louhi.FlowExecution) error {
	if flow.ID == "" {
		return skerr.Fmt("ID is required.")
	}
	if flow.FlowID == "" {
		return skerr.Fmt("FlowUniqueKey is required.")
	}
	ref := db.flows.Doc(flow.FlowID).Collection(collectionExecutions).Doc(flow.ID)
	if _, err := ref.Set(ctx, flow); err != nil {
		return skerr.Wrapf(err, "failed to set flow")
	}
	return nil
}

// GetFlowExecution implements DB.
func (db *FirestoreDB) GetFlowExecution(ctx context.Context, id string) (*louhi.FlowExecution, error) {
	docs, err := db.client.CollectionGroup(collectionExecutions).Where("ID", "==", id).Documents(ctx).GetAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if len(docs) == 0 {
		return nil, nil
	}
	if len(docs) > 1 {
		return nil, skerr.Fmt("Found multiple flow executions with ID %s", id)
	}
	var rv louhi.FlowExecution
	if err := docs[0].DataTo(&rv); err != nil {
		return nil, skerr.Wrapf(err, "failed to decode flow from DB")
	}
	return &rv, nil
}

// GetLatestFlowExecutions implements DB.
func (db *FirestoreDB) GetLatestFlowExecutions(ctx context.Context) (map[string]*louhi.FlowExecution, error) {
	iter := db.flows.DocumentRefs(ctx)
	rv := map[string]*louhi.FlowExecution{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, skerr.Wrapf(err, "failed to search FlowExecutions")
		}
		docs, err := doc.Collection(collectionExecutions).OrderBy("CreatedAt", fs.Desc).Where("Result", "!=", louhi.FlowResultUnknown).OrderBy("Result", fs.Asc).Limit(1).Documents(ctx).GetAll()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		if len(docs) == 0 {
			continue
		}
		fe := new(louhi.FlowExecution)
		if err := docs[0].DataTo(fe); err != nil {
			return nil, skerr.Wrap(err)
		}
		// The DB stores flows by unique ID, not name, and the ID may change
		// as the flow is edited, so we should deduplicate by name.
		if prev, ok := rv[fe.FlowName]; !ok || prev.CreatedAt.Before(fe.CreatedAt) {
			if ok && prev.CreatedAt.Before(fe.CreatedAt) {
				sklog.Infof("Throwing away old flow result for %q (%s) created at %s in favor of new flow created at %s", prev.FlowName, prev.FlowID, prev.CreatedAt, fe.CreatedAt)
			}
			rv[fe.FlowName] = fe
		}
	}
	return rv, nil
}

var _ louhi.DB = &FirestoreDB{}
