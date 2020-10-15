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
)

var (
	fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'bugs-central'")
	fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")

	// mtx to control access to firestore
	mtx sync.RWMutex
)

const (
	// TODO(rmistry): Rename all constants to camel-case.
	// For accessing Firestore.
	DEFAULT_ATTEMPTS      = 3
	PUT_SINGLE_TIMEOUT    = 10 * time.Second
	DELETE_SINGLE_TIMEOUT = 10 * time.Second

	// Names of Collections
	RunIdsCol = "RunIds"
)

// firestoreDB uses Cloud Firestore for store.
type FirestoreDB struct {
	client *firestore.Client
}

// The type that will stored in firestoreDB.
type QueryData struct {
	QueryLink string    `json:"query_link"`
	Created   time.Time `json:"created"`
	RunId     string    `json:"run_id"`

	CountsData *types.IssueCountsData

	// OpenCount       int `json:"open_count"`
	// UnassignedCount int `json:"unassigned_count"`
	// P0Count         int `json:"p0_count"`
	// P1Count         int `json:"p1_count"`
	// P2Count         int `json:"p2_count"`
	// P3Count         int `json:"p3_count"`
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

// runId should replace this guy..
// func (f *FirestoreDB) queryDocId(client types.RecognizedClient, source types.IssueSource, queryShortDesc string, ts time.Time) string {
// 	// return fmt.Sprintf("%s#%s#%s#%s", client, source, queryShortDesc, firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT))
// 	return firestore.FixTimestamp(ts).Format(util.SAFE_TIMESTAMP_FORMAT)
// }

func (f *FirestoreDB) queryCol(client types.RecognizedClient, source types.IssueSource, queryShortDesc string) *firestore_api.CollectionRef {
	clientCol := f.client.Collection(string(client))
	sourceDoc := clientCol.Doc(string(source))
	queryCol := sourceDoc.Collection(queryShortDesc)
	return queryCol
}

func (f *FirestoreDB) getAllClientsCountsFromDB(ctx context.Context) (*types.IssueCountsData, error) {
	countData := &types.IssueCountsData{}
	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		} else if c.ID == "RunIds" {
			continue
		}
		qcd, err := f.getAllSourcesCountsFromDB(ctx, c)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all sources counts from db")
		}
		countData.MergeInto(qcd)
	}
	return countData, nil
}

func (f *FirestoreDB) getAllSourcesCountsFromDB(ctx context.Context, clientCol *firestore_api.CollectionRef) (*types.IssueCountsData, error) {
	countData := &types.IssueCountsData{}
	sources := clientCol.DocumentRefs(ctx)
	for {
		s, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qcd, err := f.getAllQueriesCountsFromDB(ctx, s)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all queries counts from db")
		}
		countData.MergeInto(qcd)
	}
	return countData, nil
}

func (f *FirestoreDB) getAllQueriesCountsFromDB(ctx context.Context, sourceDoc *firestore_api.DocumentRef) (*types.IssueCountsData, error) {
	countData := &types.IssueCountsData{}
	sources := sourceDoc.Collections(ctx)
	for {
		query, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qcd, _, err := f.getLatestCountsFromQuery(ctx, query)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all queries counts from db")
		}
		countData.MergeInto(qcd)
	}

	return countData, nil
}

func (f *FirestoreDB) getLatestCountsFromQuery(ctx context.Context, queryCol *firestore_api.CollectionRef) (*types.IssueCountsData, string, error) {
	var qd *QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(context.TODO(), "GetFromDB", "", q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	if qd == nil {
		// Does not exist in DB yet.
		return &types.IssueCountsData{}, "", nil
	}
	return qd.CountsData, qd.QueryLink, nil
}

// HERE HERE
func (f *FirestoreDB) getAllQueryData(ctx context.Context) ([]*QueryData, error) {
	ret := []*QueryData{}
	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		} else if c.ID == RunIdsCol {
			continue
		}
		qs, err := f.getAllQueryDataFromClient(ctx, c)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all query data from db")
		}
		ret = append(ret, qs...)
	}
	return ret, nil
}

// HERE HERE
func (f *FirestoreDB) getAllQueryDataFromClient(ctx context.Context, clientCol *firestore_api.CollectionRef) ([]*QueryData, error) {
	ret := []*QueryData{}
	sources := clientCol.DocumentRefs(ctx)
	for {
		s, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qs, err := f.getAllQueryDataFromSource(ctx, s)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all query data from db")
		}
		ret = append(ret, qs...)
	}
	return ret, nil
}

// HERE HERE
func (f *FirestoreDB) getAllQueryDataFromSource(ctx context.Context, sourceDoc *firestore_api.DocumentRef) ([]*QueryData, error) {
	ret := []*QueryData{}
	queries := sourceDoc.Collections(ctx)
	for {
		q, err := queries.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qs, err := f.getAllQueryDataFromQuery(ctx, q)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all query data from db")
		}
		ret = append(ret, qs...)
	}
	return ret, nil
}

// HERE HERE
func (f *FirestoreDB) getAllQueryDataFromQuery(ctx context.Context, queryCol *firestore_api.CollectionRef) ([]*QueryData, error) {
	ret := []*QueryData{}
	q := queryCol.OrderBy("Created", firestore_api.Desc)
	err := f.client.IterDocs(ctx, "GetAllQueryData", "", q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		var qd *QueryData
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		ret = append(ret, qd)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching all query data")
	}
	return ret, nil

}

func (f *FirestoreDB) GetQueryDataFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) ([]*QueryData, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	if client == "" {
		// Client has not been specified.
		return f.getAllQueryData(ctx)
	}
	// Client has been specified.
	clientCol := f.client.Collection(string(client))

	if source == "" {
		// Source has not been specified.
		return f.getAllQueryDataFromClient(ctx, clientCol)
	}
	// Source has been specified.
	sourceDoc := clientCol.Doc(string(source))

	if query == "" {
		// Query has not been specified.
		return f.getAllQueryDataFromSource(ctx, sourceDoc)
	}
	// Query has been specified.
	queryCol := sourceDoc.Collection(query)
	return f.getAllQueryDataFromQuery(ctx, queryCol)
}

func (f *FirestoreDB) GetCountsFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (*types.IssueCountsData, string, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	if client == "" {
		// Client has not been specified. Return the total count of all clients.
		qcd, err := f.getAllClientsCountsFromDB(ctx)
		return qcd, "", err
	}
	// Client has been specified.
	clientCol := f.client.Collection(string(client))

	if source == "" {
		// Source has not been specified. Return the total count of this client.
		qcd, err := f.getAllSourcesCountsFromDB(ctx, clientCol)
		return qcd, "", err
	}
	// Source has been specified.
	sourceDoc := clientCol.Doc(string(source))

	if query == "" {
		// Query has not been specified. Return the total count of this client+source.
		qcd, err := f.getAllQueriesCountsFromDB(ctx, sourceDoc)
		return qcd, "", err
	}
	// Query has been specified.
	queryCol := sourceDoc.Collection(query)
	return f.getLatestCountsFromQuery(ctx, queryCol)
}

func (f *FirestoreDB) GetClientsFromDB(ctx context.Context) (map[types.RecognizedClient]map[types.IssueSource]map[string]bool, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	clientsMap := map[types.RecognizedClient]map[types.IssueSource]map[string]bool{}

	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		} else if c.ID == RunIdsCol {
			continue
		}
		cID := types.RecognizedClient(c.ID)
		clientsMap[cID] = map[types.IssueSource]map[string]bool{}

		sources := c.DocumentRefs(ctx)
		for {
			s, err := sources.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, err
			}
			sID := types.IssueSource(s.ID)
			clientsMap[cID][sID] = map[string]bool{}

			queries := s.Collections(ctx)
			for {
				q, err := queries.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return nil, err
				}
				qID := q.ID

				// Populate the map.
				clientsMap[cID][sID][qID] = true
			}
		}
	}

	return clientsMap, nil
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
func (f *FirestoreDB) PutInDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query, queryLink, runId string, countsData *types.IssueCountsData) error {
	if client == "" || source == "" || query == "" {
		return errors.New("Need client and source and query specified to put in DB")
	}

	// BELOW WORKS BUT WE WANT TO KEEP IN THE DB EVERYTIME REGARDLESS!

	// // Get counts for this client+source+query combination to see if the value of the query has changed.
	// qcd, _, err := f.GetCountsFromDB(ctx, client, source, query)
	// // existingData, err := f.GetLeafNodeFromDB(ctx, client, source, query)
	// if err != nil {
	// 	return skerr.Wrapf(err, "could not get from DB")
	// }

	// if countsData.IsEqual(qcd) {
	// 	sklog.Info("Not putting in DB because counts match")
	// 	return nil
	// }
	// sklog.Info("Putting in DB because counts did not match")

	mtx.Lock()
	defer mtx.Unlock()
	now := time.Now()
	qd := &QueryData{
		QueryLink:  queryLink,
		CountsData: countsData,
		Created:    now,
		RunId:      runId,
	}
	queryCol := f.queryCol(client, source, query)
	_, createErr := f.client.Create(ctx, queryCol.Doc(runId), qd, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", runId)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}

// RUN ID

type RunId struct {
	RunId string
}

func (f *FirestoreDB) GenerateRunId(ts time.Time) string {
	return ts.UTC().Format(time.RFC1123)
}

// // func (c *Client) Get(ctx context.Context, ref *firestore.DocumentRef, attempts int, timeout time.Duration) (*firestore.DocumentSnapshot, error) {
// func (f *FirestoreDB) IsRunIdValid(ctx context.Context, runId string) (bool, error) {
// 	runIdCol := f.client.Collection(RunIdsCol)
// 	_, err := f.client.Get(ctx, runIdCol.Doc(runId), DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
// 	if st, ok := status.FromError(err); ok {
// 		if st.Code() == codes.AlreadyExists {
// 			return false, skerr.Wrapf(err, "%s already exists in firestore", runId)
// 		} else if st.Code() == codes.NotFound {
// 			return false, nil
// 		}
// 	}
// 	if err != nil {
// 		return false, err
// 	}
// 	return true, nil
// }

func (f *FirestoreDB) GetAllRecognizedRunIds(ctx context.Context) (map[string]bool, error) {
	runIds := map[string]bool{}
	runIdDocs := f.client.Collection(RunIdsCol).DocumentRefs(ctx)
	for {
		r, err := runIdDocs.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		runIds[r.ID] = true
	}
	return runIds, nil
}

func (f *FirestoreDB) StoreRunId(ctx context.Context, runId string) error {
	runIdCol := f.client.Collection(RunIdsCol)
	_, err := f.client.Create(ctx, runIdCol.Doc(runId), &RunId{RunId: runId}, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT)
	if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(err, "%s already exists in firestore", runId)
	}
	if err != nil {
		return err
	}
	return nil
}
