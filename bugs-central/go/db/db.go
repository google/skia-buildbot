package db

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	return -1, -1, nil
}

func (f *FirestoreDB) getAllSourcesCountsFromDB(ctx context.Context, clientCol *firestore_api.CollectionRef) (int, int, error) {
	return -1, -1, nil
}

func (f *FirestoreDB) getAllQueriesCountsFromDB(ctx context.Context, sourceDoc *firestore_api.DocumentRef) (int, int, error) {
	return -1, -1, nil
}

// Add documentation
func (f *FirestoreDB) GetCountsFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (int, int, error) {
	mtx.RLock()
	defer mtx.RUnlock()

	openCount := 0
	unassignedCount := 0
	if client == "" {
		// Client has not been specified. Return the total count of all clients.
		fmt.Println("IN NO CLIENT SPECIFIED")
		itClients := f.client.Collections(ctx)
		for {
			c, err := itClients.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return -1, -1, err
			}

			itSources := c.DocumentRefs(ctx)
			for {
				s, err := itSources.Next()
				if err == iterator.Done {
					break
				} else if err != nil {
					return -1, -1, err
				}

				itQueries := s.Collections(ctx)
				for {
					qry, err := itQueries.Next()
					if err == iterator.Done {
						break
					} else if err != nil {
						return -1, -1, err
					}

					var qd *QueryData
					q := qry.OrderBy("Created", firestore_api.Desc).Limit(1)
					if err := f.client.IterDocs(context.TODO(), "GetFromDB", f.queryDocId(client, source, query, time.Now()), q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
						if err := doc.DataTo(&qd); err != nil {
							return err
						}
						return nil
					}); err != nil {
						return -1, -1, err
					}
					openCount += qd.OpenCount
					unassignedCount += qd.UnassignedCount
				}
			}
		}
	}

	return openCount, unassignedCount, nil

	/*
		if client == "" {
			// Client has not been specified. Return the total count of all clients.
			for _, sourceToQueries := range openIssues {
				for _, queryToIssues := range sourceToQueries {
					for _, issues := range queryToIssues {
						totalCount += len(issues)
					}
				}
			}
		} else {
			if sourceToQueries, ok := openIssues[client]; ok {
				if source == "" {
					// Source has not been specified. Return the total count of this client.
					for _, queryToIssues := range sourceToQueries {
						for _, issues := range queryToIssues {
							totalCount += len(issues)
						}
					}
				} else {
					if queryToIssues, ok := sourceToQueries[source]; ok {
						if query == "" {
							// Query has not been specified. Return the total count of this client+source.
							for _, issues := range queryToIssues {
								totalCount += len(issues)
							}
						} else {
							if issues, ok := queryToIssues[query]; ok {
								// Retuen the total count of this client+source+query.
								totalCount = len(issues)
							} else {
								return -1, fmt.Errorf("Query %s is not recognized", query)
							}
						}
					} else {
						return -1, fmt.Errorf("Source %s is not recognized", source)
					}
				}
			} else {
				return -1, fmt.Errorf("Client %s is not recognized", client)
			}
		}
	*/

}

// Change this to return counts!
func (f *FirestoreDB) GetLeafNodeFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (*QueryData, error) {
	if client == "" || source == "" || query == "" {
		return nil, errors.New("Need client and source and query specified to get from leaf node in DB")
	}

	mtx.RLock()
	defer mtx.RUnlock()

	// Query firestore for this client+source+subComponent combination.
	queryCol := f.queryCol(client, source, query)

	// var existing *QueryData
	var qd *QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(context.TODO(), "GetFromDB", f.queryDocId(client, source, query, time.Now()), q, DEFAULT_ATTEMPTS, PUT_SINGLE_TIMEOUT, func(doc *firestore_api.DocumentSnapshot) error {
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
func (f *FirestoreDB) PutInDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query, queryLink string, openCount, unassignedCount int) error {
	if client == "" || source == "" || query == "" {
		return errors.New("Need client and source and query specified to put in DB")
	}
	// GetLeafNodeFromDB to see if the value of the query changed.
	existingData, err := f.GetLeafNodeFromDB(ctx, client, source, query)
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
