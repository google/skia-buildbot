package db

import (
	"context"
	"time"

	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	appForFirestore = "autogardener"

	collectionSummaryForTask = "summary-for-task"

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

func (d *firestoreDB) PutTaskSummary(ctx context.Context, taskID string, summary *types.TaskSummary) error {
	_, err := d.client.Set(ctx, d.client.Collection(collectionSummaryForTask).Doc(taskID), summary, defaultAttempts, defaultTimeout)
	return skerr.Wrap(err)
}

var _ AutoGardenerDB = &firestoreDB{}
