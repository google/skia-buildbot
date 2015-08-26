package ignore

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/util"
)

type SQLIgnoreStore struct {
	vdb      *database.VersionedDB
	mutex    sync.Mutex
	revision int64
}

func NewSQLIgnoreStore(vdb *database.VersionedDB) IgnoreStore {
	ret := &SQLIgnoreStore{
		vdb: vdb,
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
	stmt := `INSERT INTO ignorerule (userid,  expires, query, note)
	         VALUES(?,?,?,?)`

	ret, err := m.vdb.DB.Exec(stmt, rule.Name, rule.Expires.Unix(), rule.Query, rule.Note)
	if err != nil {
		return err
	}
	createdId, err := ret.LastInsertId()
	if err != nil {
		return err
	}
	rule.ID = int(createdId)
	m.inc()
	return nil
}

// Update, see IgnoreStore interface.
func (m *SQLIgnoreStore) Update(id int, rule *IgnoreRule) error {
	stmt := `UPDATE ignorerule SET userid=?, expires=?, query=?, note=? WHERE id=?`

	res, err := m.vdb.DB.Exec(stmt, rule.Name, rule.Expires.Unix(), rule.Query, rule.Note, rule.ID)
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
func (m *SQLIgnoreStore) List() ([]*IgnoreRule, error) {
	stmt := `SELECT id, userid, expires, query, note
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
		err := rows.Scan(&target.ID, &target.Name, &expiresTS, &target.Query, &target.Note)
		if err != nil {
			return nil, err
		}
		target.Expires = time.Unix(expiresTS, 0)
		result = append(result, target)
	}
	return result, nil
}

// Delete, see IgnoreStore interface.
func (m *SQLIgnoreStore) Delete(id int, userId string) (int, error) {
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

// Revisison, see IngoreStore interface.
func (m *SQLIgnoreStore) Revision() int64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.revision
}

// BuildRuleMatcher, see IgnoreStore interface.
func (m *SQLIgnoreStore) BuildRuleMatcher() (RuleMatcher, error) {
	return buildRuleMatcher(m)
}

func buildRuleMatcher(store IgnoreStore) (RuleMatcher, error) {
	rulesList, err := store.List()
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
