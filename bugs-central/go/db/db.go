package db

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
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
type firestoreDB struct {
	client *firestore.Client
}

// The type that will stored in firestoreDB.
type QueryData struct {
	// Do you need anotehr tag?
	Count   int       `datastore:"count" json:"count"`
	Created time.Time `datastore:"created" json:"created"`
}

func Init(ctx context.Context, ts oauth2.TokenSource) (*firestoreDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, *fsProjectID, "bugs-central", *fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &firestoreDB{
		client: fsClient,
	}, nil
}

func (f *firestoreDB) queryDocId(client bugs.RecognizedClient, source bugs.IssueSource, query string, ts time.Time) string {
	return fmt.Sprintf("%s#%s#%s#%s", client, source, query, firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT))
}

func (f *firestoreDB) queryCol(client bugs.RecognizedClient, source bugs.IssueSource, query string) *firestore_api.CollectionRef {
	clientCol := f.client.Collection(string(client))
	sourceDoc := clientCol.Doc(string(source))
	queryCol := sourceDoc.Collection(query)
	return queryCol
}

// Use bugs.RecognizedClient and bugs.IssueSource instead?
func (f *firestoreDB) GetFromDB(ctx context.Context, client bugs.RecognizedClient, source bugs.IssueSource, query string) (*QueryData, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	// Query firestore for this client+source+subComponent combination.
	queryCol := f.queryCol(client, source, query)

	// var existing *QueryData
	var qd *QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(context.TODO(), "GetFromDB", f.queryDocId(client, source, query, time.Now()), q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		// var c types.CommitComment
		fmt.Println("IN HERE?)")
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		fmt.Println("AFTER EVERYTHIGN!")
		fmt.Printf("%+v", qd)
		// if comments, ok := commentsByRepo[c.Repo]; ok {
		// 		if comments.CommitComments == nil {
		// 				comments.CommitComments = map[string][]*types.CommitComment{}
		// 		}
		// 		comments.CommitComments[c.Revision] = append(comments.CommitComments[c.Revision], &c)
		// }
		return nil
	}); err != nil {
		return nil, err
	}

	return qd, nil
}

// putInDB if value is different
func (f *firestoreDB) PutInDB(ctx context.Context, client bugs.RecognizedClient, source bugs.IssueSource, query string, count int) error {
	// GetFromDB to see if the value of the query changed.
	existingData, err := f.GetFromDB(ctx, client, source, query)
	if err != nil {
		return skerr.Wrapf(err, "could not get from DB")
	}
	if existingData.Count == count {
		fmt.Println("THE COUNTS MATCH!!!")
		return nil
	}
	fmt.Println("THE COUNTS DO NOT MATCH SO CONTINUING.")

	mtx.Lock()
	defer mtx.Unlock()
	now := time.Now()
	qd := &QueryData{
		Count:   count,
		Created: now,
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

	fmt.Println("DONE POPULATING IT!")

	return nil
}
