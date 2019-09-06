// Package fs_tjstore implements the tjstore.Store interface with
// a FireStore backend.
package fs_tjstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/cenkalti/backoff"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/clstore"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// These are the collections in Firestore.
	tryJobCollection   = "tjstore_tryjob"
	tjResultCollection = "tjstore_result"
	paramsCollection   = "tjstore_params"

	// These are the fields we query by
	changeListIDField = "clid"
	crsField          = "crs"
	patchSetIDField   = "psid"
	digestField       = "digest"

	maxAttempts      = 10
	maxOperationTime = time.Minute

	// For now, this is a wild guess based on some prior work by the fs_expstore package.
	resultShards = 16
)

// StoreImpl is the firestore based implementation of tjstore.
type StoreImpl struct {
	client  *ifirestore.Client
	cisName string
}

// New returns a new StoreImpl
func New(client *ifirestore.Client, cisName string) *StoreImpl {
	return &StoreImpl{
		client:  client,
		cisName: cisName,
	}
}

// tryJobEntry represents how a TryJob is stored in FireStore.
type tryJobEntry struct {
	SystemID string `firestore:"systemid"`
	CISystem string `firestore:"cis"`

	CRSystem     string `firestore:"crs"`
	ChangeListID string `firestore:"clid"`
	PatchsetID   string `firestore:"psid"`

	DisplayName string    `firestore:"displayname"`
	Updated     time.Time `firestore:"updated"`
}

// resultEntry represents how a TryJob is stored in FireStore.
type resultEntry struct {
	TryJobID string `firestore:"tjid"`
	CISystem string `firestore:"cis"`

	CRSystem     string `firestore:"crs"`
	ChangeListID string `firestore:"clid"`
	PatchsetID   string `firestore:"psid"`

	Digest          types.Digest      `firestore:"digest"`
	ResultParams    map[string]string `firestore:"result_params"`
	GroupParamsHash string            `firestore:"group_hash"`
	OptionsHash     string            `firestore:"options_hash"`
}

// paramEntry represents a paramTools.Params stored in FireStore
type paramEntry struct {
	Map map[string]string `firestore:"map"`
}

// GetTryJob implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJob(ctx context.Context, id string) (ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(id)
	doc, err := s.client.Collection(tryJobCollection).Doc(fID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ci.TryJob{}, clstore.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "retrieving TryJob %s from firestore", fID)
	}
	if doc == nil {
		return ci.TryJob{}, clstore.ErrNotFound
	}

	tje := tryJobEntry{}
	if err := doc.DataTo(&tje); err != nil {
		id := doc.Ref.ID
		return ci.TryJob{}, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s tryjob with id %s", s.cisName, id)
	}
	tj := ci.TryJob{
		SystemID:    tje.SystemID,
		DisplayName: tje.DisplayName,
		Updated:     tje.Updated,
	}

	return tj, nil
}

// tryJobFirestoreID returns the id for a given TryJob in a given CIS - this allows us to
// look up a document by id w/o having to perform a query.
func (s *StoreImpl) tryJobFirestoreID(tjID string) string {
	return tjID + "_" + s.cisName
}

// GetTryJobs implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJobs(ctx context.Context, psID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(tryJobCollection).Where(patchSetIDField, "==", psID.PS).
		Where(crsField, "==", psID.CRS).Where(changeListIDField, "==", psID.CL)

	var xtj []ci.TryJob

	err := s.client.IterDocs("GetTryJobs", psID.Key(), q, maxAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := tryJobEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal entry with id %s", id)
		}
		xtj = append(xtj, ci.TryJob{
			SystemID:    entry.SystemID,
			DisplayName: entry.DisplayName,
			Updated:     entry.Updated,
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjobs for cl/ps %s", psID.Key())
	}

	// Sort after the fact to save us a composite index and due to the fact that the amount of
	// TryJobs per PatchSet should be small (< 100).
	ci.SortTryJobsByName(xtj)

	return xtj, nil
}

// GetResults implements the tjstore.Store interface.
func (s *StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID) ([]tjstore.TryJobResult, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(tjResultCollection).Where(patchSetIDField, "==", psID.PS).
		Where(crsField, "==", psID.CRS).Where(changeListIDField, "==", psID.CL)

	shardResults := make([][]resultEntry, resultShards)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, resultShards)

	// maps hash -> params we need to fetch
	// We will first add keys to this map, then go fetch the actual params
	shardParams := make([]map[string]paramtools.Params, resultShards)

	maxRetries := 3
	err := s.client.IterDocsInParallel("GetResults", psID.Key(), queries, maxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := resultEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal entry with id %s", id)
		}
		shardResults[i] = append(shardResults[i], entry)
		if shardParams[i] == nil {
			shardParams[i] = map[string]paramtools.Params{}
		}
		shardParams[i][entry.GroupParamsHash] = nil
		shardParams[i][entry.OptionsHash] = nil
		return nil
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjob results for %v", psID)
	}

	paramsToFetch := map[string]paramtools.Params{}
	for _, sp := range shardParams {
		for hash, _ := range sp {
			if _, ok := paramsToFetch[hash]; ok {
				// Skip it, we've already retrieved it
				continue
			}
			p, err := s.fetchParamMap(ctx, hash)
			if err != nil {
				return nil, skerr.Wrapf(err, "fetching param map with hash %s", hash)
			}
			paramsToFetch[hash] = p
		}
	}

	// inflate results
	var ret []tjstore.TryJobResult
	for _, sr := range shardResults {
		for _, re := range sr {
			tr := tjstore.TryJobResult{
				Digest:       re.Digest,
				ResultParams: re.ResultParams,
				Options:      paramsToFetch[re.OptionsHash],
				GroupParams:  paramsToFetch[re.GroupParamsHash],
			}
			ret = append(ret, tr)
		}
	}

	return ret, nil
}

func (s *StoreImpl) fetchParamMap(ctx context.Context, hash string) (map[string]string, error) {
	doc, err := s.client.Collection(paramsCollection).Doc(hash).Get(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "retrieving paramResult %s from firestore", hash)
	}
	if doc == nil {
		return nil, clstore.ErrNotFound
	}
	tje := paramEntry{}
	if err := doc.DataTo(&tje); err != nil {
		id := doc.Ref.ID
		return nil, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal paramEntry with id %s", id)
	}
	return tje.Map, nil
}

// PutTryJob implements the tjstore.Store interface.
func (s *StoreImpl) PutTryJob(ctx context.Context, psID tjstore.CombinedPSID, tj ci.TryJob) error {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(tj.SystemID)
	cd := s.client.Collection(tryJobCollection).Doc(fID)
	record := tryJobEntry{
		SystemID:     tj.SystemID,
		CISystem:     s.cisName,
		CRSystem:     psID.CRS,
		ChangeListID: psID.CL,
		PatchsetID:   psID.PS,
		DisplayName:  tj.DisplayName,
		Updated:      tj.Updated,
	}
	_, err := s.client.Set(cd, record, maxAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write TryJob %v to tjstore", tj)
	}
	return nil
}

// PutResults implements the tjstore.Store interface.
func (s *StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, tjID string, r []tjstore.TryJobResult) error {
	if len(r) == 0 {
		return nil
	}

	// maps hash -> params we want to store
	paramsToStore := map[string]paramtools.Params{}
	var xtr []resultEntry
	for _, tr := range r {
		tre := resultEntry{
			CISystem: s.cisName,
			TryJobID: tjID,

			CRSystem:     psID.CRS,
			ChangeListID: psID.CL,
			PatchsetID:   psID.PS,

			Digest:       tr.Digest,
			ResultParams: tr.ResultParams,
		}
		gh, err := hashParams(tr.GroupParams)
		if err != nil {
			return skerr.Wrapf(err, "hashing group params of %v", tr)
		}
		paramsToStore[gh] = tr.GroupParams

		oh, err := hashParams(tr.Options)
		if err != nil {
			return skerr.Wrapf(err, "hashing options params of %v", tr)
		}
		paramsToStore[oh] = tr.Options

		tre.GroupParamsHash = gh
		tre.OptionsHash = oh

		xtr = append(xtr, tre)
	}

	// batch add the maps first, so if we fail, we don't have missing data (i.e. a TryJobResult
	// pointing at a non-existent map

	b := s.client.Batch()
	count := 0
	for h, m := range paramsToStore {
		if count >= 500 { // Firestore has a built in limit of 500 docs per batch
			w := func() error {
				_, err := b.Commit(ctx)
				return err
			}
			if err := backoff.Retry(w, backoffParams); err != nil {
				return skerr.Wrapf(err, "writing batch of %d paramEntry objects", count)
			}
			b = s.client.Batch()
			count = 0
		}
		pr := s.client.Collection(paramsCollection).Doc(h)
		b.Set(pr, paramEntry{
			Map: m,
		}) //TODO(kjlubick): don't write Map if it exists to maybe help performance

		count++
	}
	w := func() error {
		_, err := b.Commit(ctx)
		return err
	}
	if err := backoff.Retry(w, backoffParams); err != nil {
		return skerr.Wrapf(err, "writing batch of %d paramEntry objects", count)
	}

	// batch add the results - we really hope this doesn't fail to avoid partial data. We won't
	// be able to easily roll back if there are more than one batch and batch 2+ fails.
	b = s.client.Batch()
	count = 0

	for _, tr := range xtr {
		if count >= 500 { // Firestore has a built in limit of 500 docs per batch
			w := func() error {
				_, err := b.Commit(ctx)
				return err
			}
			if err := backoff.Retry(w, backoffParams); err != nil {
				return skerr.Wrapf(err, "writing batch of %d resultEntry objects", count)
			}
			b = s.client.Batch()
			count = 0
		}
		t := s.client.Collection(tjResultCollection).NewDoc()
		b.Set(t, tr)
	}

	w = func() error {
		_, err := b.Commit(ctx)
		return err
	}
	if err := backoff.Retry(w, backoffParams); err != nil {
		return skerr.Wrapf(err, "writing batch of %d resultEntry objects", count)
	}

	return nil
}

var backoffParams = &backoff.ExponentialBackOff{
	InitialInterval:     time.Second,
	RandomizationFactor: 0.5,
	Multiplier:          2,
	MaxInterval:         maxOperationTime / 4,
	MaxElapsedTime:      maxOperationTime,
	Clock:               backoff.SystemClock,
}

// System implements the tjstore.Store interface.
func (s *StoreImpl) System() string {
	return s.cisName
}

// hashParams returns a hex-encoded sha256 hash of the contents of the map in a
// deterministic fashion. It uses the fact that the query package can deterministically
// turn a map into a trace key, and hashes that output.
func hashParams(m map[string]string) (string, error) {
	s, err := query.MakeKeyFast(m)
	if err != nil {
		return "", skerr.Wrapf(err, "flattening map")
	}
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:]), nil
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
