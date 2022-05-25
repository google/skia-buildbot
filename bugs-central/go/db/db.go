package db

import (
	"context"
	"errors"
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

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// Names of Collections
	runIdsCol = "RunIds"
)

// FirestoreDB uses Cloud Firestore for store.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId string) (types.BugsDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "bugs-central", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

// getAllLatestCounts returns the latest counts data for all clients.
func (f *FirestoreDB) getAllLatestCounts(ctx context.Context) (*types.IssueCountsData, error) {
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
		qcd, err := f.getLatestCountsFromClient(ctx, c)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all sources counts from db")
		}
		countData.Merge(qcd)
	}
	return countData, nil
}

// getLatestCountsFromClient returns the latest counts data for the specified client.
func (f *FirestoreDB) getLatestCountsFromClient(ctx context.Context, clientCol *firestore_api.CollectionRef) (*types.IssueCountsData, error) {
	countData := &types.IssueCountsData{}
	sources := clientCol.DocumentRefs(ctx)
	for {
		s, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qcd, err := f.getLatestCountsFromSource(ctx, s)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all queries counts from db")
		}
		countData.Merge(qcd)
	}
	return countData, nil
}

// getLatestCountsFromSource returns the latest counts data for the specified client+source.
func (f *FirestoreDB) getLatestCountsFromSource(ctx context.Context, sourceDoc *firestore_api.DocumentRef) (*types.IssueCountsData, error) {
	countData := &types.IssueCountsData{}
	sources := sourceDoc.Collections(ctx)
	for {
		query, err := sources.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		qcd, err := f.getLatestCountsFromQuery(ctx, query)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not get all queries counts from db")
		}
		countData.Merge(qcd)
	}

	return countData, nil
}

// getLatestCountsFromQuery returns the latest counts data for the specified client+source+query.
func (f *FirestoreDB) getLatestCountsFromQuery(ctx context.Context, queryCol *firestore_api.CollectionRef) (*types.IssueCountsData, error) {
	var qd *types.QueryData
	q := queryCol.OrderBy("Created", firestore_api.Desc).Limit(1)
	if err := f.client.IterDocs(ctx, "GetFromDB", "", q, defaultAttempts, getSingleTimeout, func(doc *firestore_api.DocumentSnapshot) error {
		if err := doc.DataTo(&qd); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if qd == nil {
		// Does not exist in DB yet.
		return &types.IssueCountsData{}, nil
	}
	return qd.CountsData, nil
}

// See GetCountsFromDB documentation in types.BugsDB interface.
func (f *FirestoreDB) GetCountsFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) (*types.IssueCountsData, error) {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	if client == "" {
		// Client has not been specified. Return the total count of all clients.
		qcd, err := f.getAllLatestCounts(ctx)
		return qcd, err
	}
	// Client has been specified.
	clientCol := f.client.Collection(string(client))

	if source == "" {
		// Source has not been specified. Return the total count of this client.
		qcd, err := f.getLatestCountsFromClient(ctx, clientCol)
		return qcd, err
	}
	// Source has been specified.
	sourceDoc := clientCol.Doc(string(source))

	if query == "" {
		// Query has not been specified. Return the total count of this client+source.
		qcd, err := f.getLatestCountsFromSource(ctx, sourceDoc)
		return qcd, err
	}
	// Query has been specified.
	queryCol := sourceDoc.Collection(query)
	return f.getLatestCountsFromQuery(ctx, queryCol)
}

// getAllQueryData returns query data for all clients.
func (f *FirestoreDB) getAllQueryData(ctx context.Context) ([]*types.QueryData, error) {
	ret := []*types.QueryData{}
	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		} else if c.ID == runIdsCol {
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

// getAllQueryDataFromClient returns query data for all sources of the specified client.
func (f *FirestoreDB) getAllQueryDataFromClient(ctx context.Context, clientCol *firestore_api.CollectionRef) ([]*types.QueryData, error) {
	ret := []*types.QueryData{}
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

// getAllQueryDataFromSource returns query data for all queries of the specified client+source.
func (f *FirestoreDB) getAllQueryDataFromSource(ctx context.Context, sourceDoc *firestore_api.DocumentRef) ([]*types.QueryData, error) {
	ret := []*types.QueryData{}
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

// getAllQueryDataFromQuery returns query data for the specified client+source+query.
func (f *FirestoreDB) getAllQueryDataFromQuery(ctx context.Context, queryCol *firestore_api.CollectionRef) ([]*types.QueryData, error) {
	ret := []*types.QueryData{}
	q := queryCol.OrderBy("Created", firestore_api.Desc)
	err := f.client.IterDocs(ctx, "GetAllQueryDataFromQuery", "", q, defaultAttempts, getSingleTimeout, func(doc *firestore_api.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		var qd *types.QueryData
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

// See GetQueryDataFromDB documentation in types.BugsDB interface.
func (f *FirestoreDB) GetQueryDataFromDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query string) ([]*types.QueryData, error) {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

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

// See GetClientsFromDB documentation in types.BugsDB interface.
func (f *FirestoreDB) GetClientsFromDB(ctx context.Context) (map[types.RecognizedClient]map[types.IssueSource]map[string]bool, error) {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	clientsMap := map[types.RecognizedClient]map[types.IssueSource]map[string]bool{}

	clients := f.client.Collections(ctx)
	for {
		c, err := clients.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		} else if c.ID == runIdsCol {
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

// See PutInDB documentation in types.BugsDB interface.
func (f *FirestoreDB) PutInDB(ctx context.Context, client types.RecognizedClient, source types.IssueSource, query, runId string, countsData *types.IssueCountsData) error {
	if client == "" || source == "" || query == "" {
		return errors.New("Need client and source and query specified to put in DB")
	}

	f.mtx.Lock()
	defer f.mtx.Unlock()
	now := time.Now()
	qd := &types.QueryData{
		CountsData: countsData,
		Created:    now,
		RunId:      runId,
	}
	clientCol := f.client.Collection(string(client))
	sourceDoc := clientCol.Doc(string(source))
	queryCol := sourceDoc.Collection(query)
	_, createErr := f.client.Create(ctx, queryCol.Doc(runId), qd, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", runId)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}

type RunId struct {
	RunId string
}

// See GenerateRunId documentation in types.BugsDB interface.
func (f *FirestoreDB) GenerateRunId(ts time.Time) string {
	return ts.UTC().Format(time.RFC1123)
}

// See GetAllRecognizedRunIds documentation in types.BugsDB interface.
func (f *FirestoreDB) GetAllRecognizedRunIds(ctx context.Context) (map[string]bool, error) {
	runIds := map[string]bool{}
	runIdDocs := f.client.Collection(runIdsCol).DocumentRefs(ctx)
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

// See StoreRunId documentation in types.BugsDB interface.
func (f *FirestoreDB) StoreRunId(ctx context.Context, runId string) error {
	runIdCol := f.client.Collection(runIdsCol)
	_, err := f.client.Create(ctx, runIdCol.Doc(runId), &RunId{RunId: runId}, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(err, "%s already exists in firestore", runId)
	}
	if err != nil {
		return err
	}
	return nil
}
