package types

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/util"
)

var (
	CLEANUP_INTERVAL = time.Hour
)

type SQLIgnoreStore struct {
	vdb *database.VersionedDB
}

func NewSQLIgnoreStore(vdb *database.VersionedDB) IgnoreStore {
	ret := &SQLIgnoreStore{
		vdb: vdb,
	}

	// Routinely clean the expired records in the database.
	go func() {
		for _ = range time.Tick(CLEANUP_INTERVAL) {
			stmt := `DELETE from ignorerule WHERE expires < ?`
			_, err := ret.vdb.DB.Exec(stmt, time.Now().Unix())
			if err != nil {
				glog.Errorf("Expunging ignore rules failed: %s", err)
			}
		}
	}()

	return ret
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
	return nil
}

// List, see IgnoreStore interface.
func (m *SQLIgnoreStore) List() ([]*IgnoreRule, error) {
	stmt := `SELECT id, userid, expires, query, note
	         FROM ignorerule
	         WHERE expires > ?
	         ORDER BY id ASC`
	rows, err := m.vdb.DB.Query(stmt, time.Now().Unix())
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
	return int(rowsAffected), nil
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

	ignoreRules := make([]map[string]map[string]bool, len(rulesList))
	for idx, rawRule := range rulesList {
		ignoreRules[idx], err = compileRule(rawRule.Query)
		if err != nil {
			return noopRuleMatcher, err
		}
	}

	return func(params map[string]string) ([]*IgnoreRule, bool) {
		result := []*IgnoreRule{}

	Loop:
		for ruleIdx, rule := range ignoreRules {
			// All elements in the rules are AND connected. If the list is
			// longer than available parameters the result will be false.
			if len(rule) > len(params) {
				continue
			}

			// Check if the parameters match the rule.
			for ruleKey, ruleValues := range rule {
				if _, ok := params[ruleKey]; !ok {
					continue Loop
				}
				if !ruleValues[params[ruleKey]] {
					continue Loop
				}
			}
			result = append(result, rulesList[ruleIdx])
		}

		return result, len(result) > 0
	}, nil
}
