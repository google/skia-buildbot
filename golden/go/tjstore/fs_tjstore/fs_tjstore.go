// Package fs_tjstore implements the tjstore.Store interface with a FireStore backend.
package fs_tjstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
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
	changelistIDField = "clid"
	crsField          = "crs"
	patchsetIDField   = "psid"
	digestField       = "digest"
	timestampField    = "ts"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute

	// Based on data with 400k results for a single Changelist
	// 16 shards = 5s
	// 64 shards = 2.3s
	// 256 shards = 2.3s
	resultShards = 64

	emptyParamsHash = ""
)

// StoreImpl is the firestore based implementation of tjstore.
type StoreImpl struct {
	client *ifirestore.Client

	badParamMaps metrics2.Counter
	badResults   metrics2.Counter
}

// New returns a new StoreImpl
func New(client *ifirestore.Client) *StoreImpl {
	return &StoreImpl{
		client: client,

		badParamMaps: metrics2.GetCounter("bad_param_maps"),
		badResults:   metrics2.GetCounter("bad_results"),
	}
}

// tryJobEntry represents how a TryJob is stored in FireStore.
type tryJobEntry struct {
	SystemID string `firestore:"systemid"`
	CISystem string `firestore:"cis"`

	CRSystem     string `firestore:"crs"`
	ChangelistID string `firestore:"clid"`
	PatchsetID   string `firestore:"psid"`

	DisplayName string    `firestore:"displayname"`
	Updated     time.Time `firestore:"updated"`
}

// resultEntry represents how a TryJobResult is stored in FireStore.
type resultEntry struct {
	TryJobID string `firestore:"tjid"`
	CISystem string `firestore:"cis"`

	CRSystem     string `firestore:"crs"`
	ChangelistID string `firestore:"clid"`
	PatchsetID   string `firestore:"psid"`

	Digest          types.Digest      `firestore:"digest"`
	ResultParams    map[string]string `firestore:"result_params"`
	GroupParamsHash string            `firestore:"group_hash"`
	OptionsHash     string            `firestore:"options_hash"`

	CreatedTS time.Time `firestore:"ts"`
}

// paramEntry represents a paramTools.Params stored in FireStore
type paramEntry struct {
	Map map[string]string `firestore:"map"`
}

// GetTryJob implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJob(ctx context.Context, id, cisName string) (ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(id, cisName)
	doc, err := s.client.Collection(tryJobCollection).Doc(fID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return ci.TryJob{}, tjstore.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "retrieving TryJob %s from firestore", fID)
	}
	if doc == nil {
		return ci.TryJob{}, tjstore.ErrNotFound
	}

	tje := tryJobEntry{}
	if err := doc.DataTo(&tje); err != nil {
		id := doc.Ref.ID
		return ci.TryJob{}, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal %s tryjob with id %s", cisName, id)
	}
	tj := ci.TryJob{
		SystemID:    tje.SystemID,
		System:      cisName,
		DisplayName: tje.DisplayName,
		Updated:     tje.Updated,
	}

	return tj, nil
}

// tryJobFirestoreID returns the id for a given TryJob in a given CIS - this allows us to
// look up a document by id w/o having to perform a query.
func (s *StoreImpl) tryJobFirestoreID(tjID, cisName string) string {
	return tjID + "_" + cisName
}

// GetTryJobs implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJobs(ctx context.Context, psID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(tryJobCollection).Where(crsField, "==", psID.CRS).
		Where(changelistIDField, "==", psID.CL).Where(patchsetIDField, "==", psID.PS)

	var xtj []ci.TryJob

	err := s.client.IterDocs(ctx, "GetTryJobs", psID.Key(), q, maxReadAttempts, maxOperationTime, func(doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := tryJobEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal tryJobEntry with id %s", id)
		}
		xtj = append(xtj, ci.TryJob{
			SystemID:    entry.SystemID,
			System:      entry.CISystem,
			DisplayName: entry.DisplayName,
			Updated:     entry.Updated,
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjobs for cl/ps %s", psID.Key())
	}

	// Sort after the fact to save us a composite index and due to the fact that the amount of
	// TryJobs per Patchset should be small (< 100).
	ci.SortTryJobsByName(xtj)

	return xtj, nil
}

// GetResults implements the tjstore.Store interface.
func (s *StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID, updatedAfter time.Time) ([]tjstore.TryJobResult, error) {
	defer metrics2.FuncTimer().Stop()
	q := s.client.Collection(tjResultCollection).Where(crsField, "==", psID.CRS).
		Where(changelistIDField, "==", psID.CL).Where(patchsetIDField, "==", psID.PS)

	shards := resultShards
	var queries []firestore.Query
	if !updatedAfter.IsZero() {
		// If we are including a time, we can't shard, since sharding uses inequalities and firestore
		// won't let you do two inequalities on different fields. This may result in reduced
		// performance, but in practice this shouldn't matter too much because we:
		//   1) Only provide a time when doing a partial load of results while indexing changelists.
		//   2) Index changelists in the background (and in parallel), not in a user-visible way.
		shards = 1
		queries = []firestore.Query{q.Where(timestampField, ">=", updatedAfter)}
	} else {
		queries = fs_utils.ShardOnDigest(q, digestField, shards)
	}

	shardResults := make([][]resultEntry, shards)

	// maps hash -> params we need to fetch
	// We will first add keys to this map, then go fetch the actual params
	shardParams := make([]util.StringSet, shards)

	err := s.client.IterDocsInParallel(ctx, "GetResults", psID.Key(), queries, maxReadAttempts, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := resultEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal resultEntry with id %s", id)
		}
		shardResults[i] = append(shardResults[i], entry)
		if shardParams[i] == nil {
			shardParams[i] = util.StringSet{}
		}
		shardParams[i][entry.GroupParamsHash] = true
		shardParams[i][entry.OptionsHash] = true
		return nil
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjob results for %v", psID)
	}

	// Fetch all the param maps that were requested
	paramsByHash := map[string]paramtools.Params{
		emptyParamsHash: {},
	}
	var xdr []*firestore.DocumentRef

	// deduplicate params that appeared on multiple shards.
	for _, sp := range shardParams {
		for hash := range sp {
			if _, ok := paramsByHash[hash]; ok {
				// Skip it, we've already "scheduled" it for fetching.
				continue
			}
			paramsByHash[hash] = map[string]string{}
			xdr = append(xdr, s.client.Collection(paramsCollection).Doc(hash))
		}
	}

	s.client.CountReadQueryAndRows(s.client.Collection(paramsCollection).Path, len(xdr))
	paramDocs, err := s.client.GetAll(ctx, xdr)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching %d params", len(xdr))
	}

	for _, doc := range paramDocs {
		if !doc.Exists() {
			s.badParamMaps.Inc(1)
			sklog.Warningf("Could not find param map with id %s", doc.Ref.ID)
			continue
		}
		tje := paramEntry{}
		if err := doc.DataTo(&tje); err != nil {
			id := doc.Ref.ID
			return nil, skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal paramEntry with id %s", id)
		}
		// doc.Ref.ID is the hash of the param map
		paramsByHash[doc.Ref.ID] = tje.Map
	}

	// inflate results with those param maps
	var ret []tjstore.TryJobResult
	for _, sr := range shardResults {
		for _, re := range sr {
			// ignore any results with missing Params (hopefully none are missing)
			o, ok := paramsByHash[re.OptionsHash]
			if !ok {
				s.badResults.Inc(1)
				continue
			}
			g, ok := paramsByHash[re.GroupParamsHash]
			if !ok {
				s.badResults.Inc(1)
				continue
			}
			tr := tjstore.TryJobResult{
				Digest:       re.Digest,
				ResultParams: re.ResultParams,
				Options:      o,
				GroupParams:  g,
			}
			ret = append(ret, tr)
		}
	}
	return ret, nil
}

// PutTryJob implements the tjstore.Store interface.
func (s *StoreImpl) PutTryJob(ctx context.Context, psID tjstore.CombinedPSID, tj ci.TryJob) error {
	defer metrics2.FuncTimer().Stop()
	fID := s.tryJobFirestoreID(tj.SystemID, tj.System)
	cd := s.client.Collection(tryJobCollection).Doc(fID)
	record := tryJobEntry{
		SystemID:     tj.SystemID,
		CISystem:     tj.System,
		CRSystem:     psID.CRS,
		ChangelistID: psID.CL,
		PatchsetID:   psID.PS,
		DisplayName:  tj.DisplayName,
		Updated:      tj.Updated,
	}
	_, err := s.client.Set(ctx, cd, record, maxWriteAttempts, maxOperationTime)
	if err != nil {
		return skerr.Wrapf(err, "could not write TryJob %v to tjstore", tj)
	}
	return nil
}

// PutResults implements the tjstore.Store interface.
// Firestore has a limit of 500 documents written for either a transaction or a batch write.
// This would make a rollback difficult, so we opt to retry any failures multiple times.
// We store maps first, so if we do fail, we can bail out w/o having written the
// (incomplete) TryJobResults.  We take a similar approach in fs_expstore, which has been fine.
func (s *StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, tjID, cisName string, r []tjstore.TryJobResult, ts time.Time) error {
	if len(r) == 0 {
		return nil
	}
	defer metrics2.FuncTimer().Stop()
	// maps hash -> params have seen and will need to store
	seenParams := map[string]paramtools.Params{}
	var xtr []resultEntry
	for _, tr := range r {
		tre := resultEntry{
			CISystem: cisName,
			TryJobID: tjID,

			CRSystem:     psID.CRS,
			ChangelistID: psID.CL,
			PatchsetID:   psID.PS,

			Digest:       tr.Digest,
			ResultParams: tr.ResultParams,

			CreatedTS: ts,
		}
		gh, err := hashParams(tr.GroupParams)
		if err != nil {
			return skerr.Wrapf(err, "hashing group params of %v", tr)
		}
		seenParams[gh] = tr.GroupParams

		oh, err := hashParams(tr.Options)
		if err != nil {
			return skerr.Wrapf(err, "hashing options params of %v", tr)
		}
		seenParams[oh] = tr.Options

		tre.GroupParamsHash = gh
		tre.OptionsHash = oh

		xtr = append(xtr, tre)
	}

	// batch add the maps first, so if we fail, we don't have missing data (i.e. a TryJobResult
	// pointing at a non-existent map
	type keyAndParam struct {
		key    string
		params paramtools.Params
	}
	var paramsToWrite []keyAndParam
	for h, m := range seenParams {
		if h == emptyParamsHash {
			continue
		}
		paramsToWrite = append(paramsToWrite, keyAndParam{
			key:    h,
			params: m,
		})
	}

	s.client.CountWriteQueryAndRows(s.client.Collection(paramsCollection).Path, len(paramsToWrite))
	err := s.client.BatchWrite(ctx, len(paramsToWrite), ifirestore.MAX_TRANSACTION_DOCS, maxOperationTime, nil, func(b *firestore.WriteBatch, i int) error {
		p := paramsToWrite[i]
		pr := s.client.Collection(paramsCollection).Doc(p.key)
		b.Set(pr, paramEntry{
			Map: p.params,
		}) //TODO(kjlubick): don't write Map if it exists to maybe help performance
		return nil
	})
	if err != nil {
		return skerr.Wrapf(err, "writing batch paramEntry objects")
	}

	s.client.CountWriteQueryAndRows(s.client.Collection(tjResultCollection).Path, len(xtr))
	// batch add the results - we really hope this doesn't fail to avoid partial data. We won't
	// be able to easily roll back if there is more than one batch, and batch 2+ fails.
	err = s.client.BatchWrite(ctx, len(xtr), ifirestore.MAX_TRANSACTION_DOCS, maxOperationTime, nil, func(b *firestore.WriteBatch, i int) error {
		t := s.client.Collection(tjResultCollection).NewDoc()
		b.Set(t, xtr[i])
		return nil
	})
	if err != nil {
		return skerr.Wrapf(err, "writing batch of resultEntry objects")
	}
	return nil
}

// hashParams returns a hex-encoded sha256 hash of the contents of the map in a
// deterministic fashion. It uses the fact that the query package can deterministically
// turn a map into a trace key, and hashes that output.
func hashParams(m paramtools.Params) (string, error) {
	if len(m) == 0 {
		return emptyParamsHash, nil
	}
	s, err := query.MakeKeyFast(m)
	if err != nil {
		return "", skerr.Wrapf(err, "flattening map")
	}
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:]), nil
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
