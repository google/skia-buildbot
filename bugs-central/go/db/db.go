package db

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var (
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	fsClient *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
)

const (
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second
)

// The type that will stored in firestore.
type QueryData struct {
	// Do you need anotehr tag?
	Query   string    `datastore:"query" json:"query"`
	Count   int64     `datastore:"count" json:"count"`
	Created time.Time `datastore:"created" json:"created"`
}

func Init(ctx context.Context, ts oauth2.TokenSource) error {
	// Instantiate firestore.
	var err error
	fsClient, err = firestore.NewClient(ctx, *fsProjectID, "bugs-central", *fsNamespace, ts)
	if err != nil {
		return skerr.Wrapf(err, "could not init firestore")
	}
	return nil
}

func queryColId(client bugs.RecognizedClient, source bugs.IssueSource, query string, ts time.Time) string {
	return fmt.Sprintf("%s#%s#%s#%s", client, source, query, firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT))
}

// Use bugs.RecognizedClient and bugs.IssueSource instead?
func GetFromDB(ctx context.Context, client bugs.RecognizedClient, source bugs.IssueSource, query string) error {
	mtx.RLock()
	defer mtx.RUnlock()

	// Query firestore for this client+source+subComponent combination.
	clientCol := fsClient.Collection(string(client))
	sourceDoc := clientCol.Doc(string(source))
	queryCol := sourceDoc.Collection(query)
	now := time.Now()
	queryColId := queryColId(client, source, query, now)

	// var existing *QueryData
	q := queryCol.OrderBy("created", firestore_api.Desc).Limit(1)
	if err := fsClient.IterDocs(context.TODO(), "GetFromDB", queryColId, q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		// var c types.CommitComment
		fmt.Println("IN HERE?)")
		var qd *QueryData
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
		return err
	}
	fmt.Println("OUTSIDE HERE. Try to populate this now")

	

	// if err := fsClient.RunTransaction(ctx, "GetFroMDB", "???", DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(ctx context.Context, tx *firestore_api.Transaction) error {
	// 	if snap, err := tx.Get(queryCol); err == nil {
	// 		existing = new(QueryData)
	// 		if err := snap.DataTo(existing); err != nil {
	// 			return err
	// 		}
	// 	} else if st, ok := status.FromError(err); ok && st.Code() != codes.NotFound {
	// 		return err
	// 	}
	// }); err != nil {
	// 	return error
	// }

	// // col.Doc(id)
	// q := &QueryData{
	// 	Query: "test query",
	// 	Count: 99,
	// }
	// if _, createErr := fsClient.Create(ctx, doc, q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT); createErr != nil {
	// 	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
	// 		fmt.Println("ALREADY EXISTS!")
	// 	} else {
	// 		return skerr.Wrap(createErr)
	// 	}
	// }

	return nil
}

// putInDB if value is different
func PutInDB() error {
	mtx.Lock()
	defer mtx.Unlock()

	return nil
}
