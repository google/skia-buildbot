package digeststore

import (
	"database/sql"
	"fmt"

	"go.skia.org/infra/go/database"
	ptypes "go.skia.org/infra/perf/go/types"
)

type SQLDigestStore struct {
	vdb *database.VersionedDB
}

func NewSQLDigestStore(vdb *database.VersionedDB) DigestStore {
	return &SQLDigestStore{
		vdb: vdb,
	}
}

// See DigestStore interface.
func (s *SQLDigestStore) GetDigestInfo(testName, digest string) (*DigestInfo, bool, error) {
	const stmt = `SELECT first, last, exception
				  FROM test_digest
				  WHERE name=? AND digest=?`

	row := s.vdb.DB.QueryRow(stmt, testName, digest)
	ret := &DigestInfo{
		TestName: testName,
		Digest:   digest,
	}
	err := row.Scan(&ret.First, &ret.Last, &ret.Exception)
	if err != nil {
		// Unknown testname/digest pair.
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return ret, true, nil
}

// See DigestStore interface.
func (s *SQLDigestStore) UpdateDigestTimeStamps(testName, digest string, commit *ptypes.Commit) (*DigestInfo, error) {
	const stmt = `INSERT INTO test_digest (name, digest, first, last)
				  VALUES (?, ?, ?, ?)
				  ON DUPLICATE KEY UPDATE
					first= IF(first > ?, ?, first),
				    last = IF(last < ?, ?, last)`
	t := commit.CommitTime
	_, err := s.vdb.DB.Exec(stmt, testName, digest, t, t, t, t, t, t)
	if err != nil {
		return nil, err
	}

	ret, ok, err := s.GetDigestInfo(testName, digest)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("Cannot find digest info that was just added to the database.")
	}
	return ret, nil
}
