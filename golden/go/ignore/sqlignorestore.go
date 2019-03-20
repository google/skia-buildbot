package ignore

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type SQLIgnoreStore struct {
	vdb           *database.VersionedDB
	mutex         sync.Mutex
	revision      int64
	cpxTileStream <-chan *types.ComplexTile
	lastCpxTile   *types.ComplexTile
	expStore      expstorage.ExpectationsStore
}

// NewSQLIgnoreStore creates a new SQL based IgnoreStore.
//   vdb - database to connect to.
//   expStore - expectations store needed to count the untriaged digests per rule.
//   tileStream - continuously provides an updated copy of the current tile.
func NewSQLIgnoreStore(vdb *database.VersionedDB, expStore expstorage.ExpectationsStore, tileStream <-chan *types.ComplexTile) IgnoreStore {
	ret := &SQLIgnoreStore{
		vdb:           vdb,
		cpxTileStream: tileStream,
		expStore:      expStore,
	}

	return ret
}

func (m *SQLIgnoreStore) inc() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.revision += 1
}

// Create, see IgnoreStore interface.
func (m *SQLIgnoreStore) Create(rule *IgnoreRule) error {
	stmt := `INSERT INTO ignorerule (userid, updated_by, expires, query, note)
	         VALUES(?,?,?,?,?)`

	ret, err := m.vdb.DB.Exec(stmt, rule.Name, rule.Name, rule.Expires.Unix(), rule.Query, rule.Note)
	if err != nil {
		return err
	}
	createdId, err := ret.LastInsertId()
	if err != nil {
		return err
	}
	rule.ID = createdId
	m.inc()
	return nil
}

// Update, see IgnoreStore interface.
func (m *SQLIgnoreStore) Update(id int64, rule *IgnoreRule) error {
	stmt := `UPDATE ignorerule SET updated_by=?, expires=?, query=?, note=? WHERE id=?`

	res, err := m.vdb.DB.Exec(stmt, rule.UpdatedBy, rule.Expires.Unix(), rule.Query, rule.Note, rule.ID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return fmt.Errorf("Did not find an IgnoreRule with id: %d", id)
	}
	m.inc()
	return nil
}

// List, see IgnoreStore interface.
func (m *SQLIgnoreStore) List(addCounts bool) ([]*IgnoreRule, error) {
	stmt := `SELECT id, userid, updated_by, expires, query, note
	         FROM ignorerule
	         ORDER BY expires ASC`
	rows, err := m.vdb.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	result := []*IgnoreRule{}
	for rows.Next() {
		target := &IgnoreRule{}
		var expiresTS int64
		err := rows.Scan(&target.ID, &target.Name, &target.UpdatedBy, &expiresTS, &target.Query, &target.Note)
		if err != nil {
			return nil, err
		}
		target.Expires = time.Unix(expiresTS, 0)
		result = append(result, target)
	}

	if addCounts {
		m.lastCpxTile, err = addIgnoreCounts(result, m, m.lastCpxTile, m.expStore, m.cpxTileStream)
		if err != nil {
			sklog.Errorf("Unable to add counts to ignore list result: %s", err)
		}
	}

	return result, nil
}

// Delete, see IgnoreStore interface.
func (m *SQLIgnoreStore) Delete(id int64) (int, error) {
	stmt := "DELETE FROM ignorerule WHERE id=?"
	ret, err := m.vdb.DB.Exec(stmt, id)
	if err != nil {
		return 0, err
	}
	rowsAffected, err := ret.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rowsAffected > 0 {
		m.inc()
	}
	return int(rowsAffected), nil
}

// Revision, see IngoreStore interface.
func (m *SQLIgnoreStore) Revision() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.revision
}

// BuildRuleMatcher, see IgnoreStore interface.
func (m *SQLIgnoreStore) BuildRuleMatcher() (RuleMatcher, error) {
	return buildRuleMatcher(m)
}

// TODO(stephana): move buildRuleMatcher into a separate file since it's
// used by multiple implementations of IgnoreStore

func buildRuleMatcher(store IgnoreStore) (RuleMatcher, error) {
	rulesList, err := store.List(false)
	if err != nil {
		return noopRuleMatcher, err
	}

	ignoreRules := make([]QueryRule, len(rulesList))
	for idx, rawRule := range rulesList {
		parsedQuery, err := url.ParseQuery(rawRule.Query)
		if err != nil {
			return noopRuleMatcher, err
		}
		ignoreRules[idx] = NewQueryRule(parsedQuery)
	}

	return func(params map[string]string) ([]*IgnoreRule, bool) {
		result := []*IgnoreRule{}

		for ruleIdx, rule := range ignoreRules {
			if rule.IsMatch(params) {
				result = append(result, rulesList[ruleIdx])
			}
		}

		return result, len(result) > 0
	}, nil
}

// TODO(stephana): Add unit tests to addIgnoreCounts once we have a framework ready to
// easily test against live (vs synthetic) data.

// addIgnoreCounts adds counts for the current tile to the given list of rules.
func addIgnoreCounts(rules []*IgnoreRule, ignoreStore IgnoreStore, lastCpxTile *types.ComplexTile, expStore expstorage.ExpectationsStore, tileStream <-chan *types.ComplexTile) (*types.ComplexTile, error) {
	if (expStore == nil) || (tileStream == nil) {
		return nil, fmt.Errorf("Either expStore or tileStream is nil. Cannot count ignores.")
	}

	exp, err := expStore.Get()
	if err != nil {
		return nil, err
	}

	ignoreMatcher, err := ignoreStore.BuildRuleMatcher()
	if err != nil {
		return nil, err
	}

	// Get the next tile.
	var cpxTile *types.ComplexTile = nil
	select {
	case cpxTile = <-tileStream:
	default:
		cpxTile = lastCpxTile
	}
	if cpxTile == nil {
		return nil, fmt.Errorf("No tile available to count ignores")
	}

	// Count the untriaged digests in HEAD.
	// matchingDigests[rule.ID]map[digest]bool
	matchingDigests := make(map[int64]map[string]bool, len(rules))
	rulesByDigest := map[string]map[int64]bool{}
	tileWithIgnores := cpxTile.GetTile(true)
	for _, trace := range tileWithIgnores.Traces {
		gTrace := trace.(*types.GoldenTrace)
		if matchRules, ok := ignoreMatcher(gTrace.Params_); ok {
			testName := gTrace.Params_[types.PRIMARY_KEY_FIELD]
			if digest := gTrace.LastDigest(); digest != types.MISSING_DIGEST && (exp.Classification(testName, digest) == types.UNTRIAGED) {
				k := testName + ":" + digest
				for _, r := range matchRules {
					// Add the digest to all matching rules.
					if t, ok := matchingDigests[r.ID]; ok {
						t[k] = true
					} else {
						matchingDigests[r.ID] = map[string]bool{k: true}
					}

					// Add the rule to the test-digest.
					if t, ok := rulesByDigest[k]; ok {
						t[r.ID] = true
					} else {
						rulesByDigest[k] = map[int64]bool{r.ID: true}
					}
				}
			}
		}
	}

	for _, r := range rules {
		r.Count = len(matchingDigests[r.ID])
		r.ExclusiveCount = 0
		for testDigestKey := range matchingDigests[r.ID] {
			// If exactly this one rule matches then account for it.
			if len(rulesByDigest[testDigestKey]) == 1 {
				r.ExclusiveCount++
			}
		}
	}
	return cpxTile, nil
}
