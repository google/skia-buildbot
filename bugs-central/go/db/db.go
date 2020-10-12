package db

import (
	"context"
	"errors"
	"flag"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
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

func (f *FirestoreDB) getAllClientsCountsFromDB(ctx context.Context) (int, int, error) {
	openCount := 0
	unassignedCount := 0
	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return -1, -1, err
		}
		o, u, err := f.getAllSourcesCountsFromDB(ctx, c)
		if err != nil {
			return -1, -1, skerr.Wrapf(err, "could not get all sources counts from db")
		}
		openCount += o
		unassignedCount += u
	}
	return openCount, unassignedCount, nil
}

func (f *FirestoreDB) getAllSourcesCountsFromDB(ctx context.Context, clientCol *firestore_api.CollectionRef) (int, int, error) {
	openCount := 0
	unassignedCount := 0
	sources := clientCol.DocumentRefs(ctx)
	for {
		s, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return -1, -1, err
		}
		o, u, err := f.getAllQueriesCountsFromDB(ctx, s)
		if err != nil {
			return -1, -1, skerr.Wrapf(err, "could not get all queries counts from db")
		}
		openCount += o
		unassignedCount += u
	}
	return openCount, unassignedCount, nil
}

func (f *FirestoreDB) getAllQueriesCountsFromDB(ctx context.Context, sourceDoc *firestore_api.DocumentRef) (int, int, error) {
	openCount := 0
	unassignedCount := 0

	sources := sourceDoc.Collections(ctx)
	for {
		query, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return -1, -1, err
		}
		o, u, err := f.getLatestCountsFromQuery(ctx, query)
		openCount += o
		unassignedCount += u
	}

	return openCount, unassignedCount, nil
}

// More things should call this!
func (f *FirestoreDB) getLatestCountsFromQuery(ctx context.Context, queryCol *firestore_api.CollectionRef) (int, int, error) {
	var qd *QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(context.TODO(), "GetFromDB", "", q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return -1, -1, err
	}
	if qd == nil {
		// Does not exist in DB yet.
		return 0, 0, nil
	}
	return qd.OpenCount, qd.UnassignedCount, nil
}

// GetCountsFromDB
func (f *FirestoreDB) GetCountsFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (int, int, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	if client == "" {
		// Client has not been specified. Return the total count of all clients.
		return f.getAllClientsCountsFromDB(ctx)
	}
	// Client has been specified.
	clientCol := f.client.Collection(string(client))

	if source == "" {
		// Source has not been specified. Return the total count of this client.
		return f.getAllSourcesCountsFromDB(ctx, clientCol)
	}
	// Source has been specified.
	sourceDoc := clientCol.Doc(string(source))

	if query == "" {
		// Query has not been specified. Return the total count of this client+source.
		return f.getAllQueriesCountsFromDB(ctx, sourceDoc)
	}
	// Query has been specified.
	queryCol := sourceDoc.Collection(query)
	return f.getLatestCountsFromQuery(ctx, queryCol)
}

// // Change this to return counts!
// func (f *FirestoreDB) GetLeafNodeFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (int, int, error) {
// 	if client == "" || source == "" || query == "" {
// 		return nil, errors.New("Need client and source and query specified to get from leaf node in DB")
// 	}

// 	mtx.RLock()
// 	defer mtx.RUnlock()

// 	// Query firestore for this client+source+query combination.
// 	queryCol := f.queryCol(client, source, query)
// 	return getLatestCountsFromQuery(ctx, queryCol)
// }

// putInDB if value is different
func (f *FirestoreDB) PutInDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query, queryLink string, openCount, unassignedCount int) error {
	if client == "" || source == "" || query == "" {
		return errors.New("Need client and source and query specified to put in DB")
	}
	// Get counts for this client+source+query combination to see if the value of the query has changed.
	o, u, err := f.GetCountsFromDB(ctx, client, source, query)
	// existingData, err := f.GetLeafNodeFromDB(ctx, client, source, query)
	if err != nil {
		return skerr.Wrapf(err, "could not get from DB")
	}
	if o == openCount && u == unassignedCount {
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
	queryDocId := f.queryDocId(client, source, query, now)
	queryCol := f.queryCol(client, source, query)
	_, createErr := f.client.Create(ctx, queryCol.Doc(queryDocId), qd, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", queryDocId)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}
