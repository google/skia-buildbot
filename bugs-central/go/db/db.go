package db

import (
	"context"
	"flag"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	// mtx to control access to firestore
	mtx sync.RWMutex
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second
)

// firestoreDB uses Cloud Firestore for store.
type FirestoreDB struct {
	client *firestore.Client
}

// The type that will stored in firestoreDB.
type QueryData struct {
	QueryLink       string    `json:"query_link"`
	OpenCount       int       `json:"open_count"`
	UnassignedCount int       `json:"unassigned_count"`
	Created         time.Time `json:"created"`
}

func Init(ctx context.Context, ts oauth2.TokenSource) (*FirestoreDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "bugs-central", *fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

func (f *FirestoreDB) queryDocId(client types.RecognizedClient, source types.IssueSource, queryShortDesc string, ts time.Time) string {
	// return fmt.Sprintf("%s#%s#%s#%s", client, source, queryShortDesc, firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT))
	return firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT)
}

func (f *FirestoreDB) queryCol(client types.RecognizedClient, source types.IssueSource, queryShortDesc string) *firestore_api.CollectionRef {
	clientCol := f.client.Collection(string(client))
	sourceDoc := clientCol.Doc(string(source))
	queryCol := sourceDoc.Collection(queryShortDesc)
	return queryCol
}

// Use bugs.RecognizedClient and bugs.IssueSource instead?
func (f *FirestoreDB) GetFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, queryShortDesc string) (*QueryData, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	// Query firestore for this client+source+subComponent combination.
	queryCol := f.queryCol(client, source, queryShortDesc)

	// var existing *QueryData
	var qd *QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(context.TODO(), "GetFromDB", f.queryDocId(client, source, queryShortDesc, time.Now()), q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return qd, nil
}

// putInDB if value is different
func (f *FirestoreDB) PutInDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, queryShortDesc, queryLink string, openCount, unassignedCount int) error {
	// GetFromDB to see if the value of the query changed.
	existingData, err := f.GetFromDB(ctx, client, source, queryShortDesc)
	if err != nil {
		return skerr.Wrapf(err, "could not get from DB")
	}
	if existingData != nil && existingData.OpenCount == openCount && existingData.UnassignedCount == unassignedCount {
		sklog.Info("Not putting in DB because counts match")
		return nil
	}
	sklog.Info("Putting in DB because counts did not match")

	mtx.Lock()
	defer mtx.Unlock()
	now := time.Now()
	qd := &QueryData{
		QueryLink:       queryLink,
		OpenCount:       openCount,
		UnassignedCount: unassignedCount,
		Created:         now,
	}
	queryDocId := f.queryDocId(client, source, queryShortDesc, now)
	queryCol := f.queryCol(client, source, queryShortDesc)
	_, createErr := f.client.Create(ctx, queryCol.Doc(queryDocId), qd, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", queryDocId)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}
