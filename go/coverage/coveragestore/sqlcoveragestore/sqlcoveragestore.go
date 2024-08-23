package sqlcoveragestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgconn"
	pb "go.skia.org/infra/go/coverage/proto/v1"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	addFile statement = iota
	addBuilder
	addTestSuite
	deleteFile
	listTestSuite
	listAll
)

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statements = map[statement]string{
	addFile: `
		INSERT INTO testsuitemapping (file_name, builder_name, test_suite_name)
		SELECT * FROM (
			VALUES ($1::STRING, $2::STRING, $3::STRING[])
		) AS vals (file_name, builder_name, test_suite_name)
		 WHERE NOT EXISTS (
		 SELECT 1 FROM testsuitemapping
		 WHERE testsuitemapping.file_name = vals.file_name)
		`,
	addBuilder: `
		INSERT INTO testsuitemapping (file_name, builder_name, test_suite_name)
		SELECT * FROM (
			VALUES ($1::STRING, $2::STRING, $3::STRING[])
		) AS vals (file_name, builder_name, test_suite_name)
		 WHERE NOT EXISTS (
		 SELECT 1 FROM testsuitemapping
		 WHERE testsuitemapping.file_name = vals.file_name
		 AND testsuitemapping.builder_name = vals.builder_name)
		`,
	addTestSuite: `
		UPDATE testsuitemapping
		SET test_suite_name = array_append(test_suite_name,$3::STRING)
		WHERE testsuitemapping.file_name = $1::STRING
		AND testsuitemapping.builder_name = $2::STRING
		AND NOT $3::STRING=ANY(testsuitemapping.test_suite_name)
		`,
	deleteFile: `
		DELETE FROM
			testsuitemapping WHERE file_name=$1 AND builder_name=$2`,
	listTestSuite: `
		SELECT * FROM testsuitemapping
		WHERE file_name=$1 AND builder_name=$2`,
	listAll: `
		SELECT * FROM testsuitemapping`,
}

// coverageStore implements the coverage.Store interface.
type CoverageStore struct {
	// db is the database interface.
	db pool.Pool
}

func New(db pool.Pool) (*CoverageStore, error) {
	return &CoverageStore{
		db: db,
	}, nil
}

// Add implements the coverage.CoverageStore interface.
func (s *CoverageStore) Add(ctx context.Context, req *pb.CoverageChangeRequest) error {
	rows, err := s.sqlExecInsert(ctx, statements[addFile], req)
	if err != nil || rows > 0 {
		sklog.Errorf("Add Failed: %s", statements[addFile])
		return err
	}
	rows, err = s.sqlExecInsert(ctx, statements[addBuilder], req)
	if err != nil || rows > 0 {
		return err
	}
	rows, err = s.sqlExecUpdate(ctx, statements[addTestSuite], req)
	if err == nil && rows == 0 {
		err = errors.New("No Rows Added")
	}
	return err
}

// Delete removes the Filename with the given filename.
func (s *CoverageStore) Delete(ctx context.Context, req *pb.CoverageChangeRequest) error {
	rows, err := s.sqlExecDelete(ctx, statements[deleteFile], req)
	if err == nil && rows == 0 {
		err = errors.New("No Rows Deleted")
	}
	return err
}

// List retrieves all the Coverage mapppings.
func (s *CoverageStore) List(ctx context.Context, req *pb.CoverageListRequest) ([]string, error) {
	sklog.Debugf("List: %s", req)
	var response struct {
		id              string
		file_name       string
		builder_name    string
		test_suite_name []string
		last_modified   time.Time
	}
	rows, err := s.db.Query(ctx, statements[listTestSuite], req.GetFileName(), req.GetBuilderName())
	if err != nil {
		sklog.Errorf("SQL: %s", statements[listTestSuite])
		return nil, err
	}

	defer rows.Close()
	counter := 0

	for rows.Next() {
		counter++
		if err := rows.Scan(&response.id, &response.file_name, &response.builder_name,
			&response.test_suite_name, &response.last_modified); err != nil {
			sklog.Debugf("Row Error: %s", err)
		} else {
			sklog.Debugf("Response: %s", response.test_suite_name)
		}
	}
	if counter == 0 {
		err = errors.New(fmt.Sprintf("No Rows Found for: %v", req.GetFileName()))
	}
	return response.test_suite_name, err
}

// List retrieves all the Coverage mapppings.
func (s *CoverageStore) ListAll(ctx context.Context, req *pb.CoverageRequest) ([]*pb.CoverageResponse, error) {
	sklog.Debugf("List: %s", req)

	rows, err := s.db.Query(ctx, statements[listAll])
	if err != nil {
		sklog.Errorf("SQL: %s", statements[listAll])
		return nil, err
	}

	defer rows.Close()
	counter := 0
	var responses []*pb.CoverageResponse

	for rows.Next() {
		var coverageResponse pb.CoverageResponse
		counter++

		var response struct {
			id            string
			last_modified time.Time
		}
		if err := rows.Scan(&response.id, &coverageResponse.FileName, &coverageResponse.BuilderName,
			&coverageResponse.TestSuiteName, &response.last_modified); err != nil {
			sklog.Debugf("Row Error: %s", err)
		} else {
			responses = append(responses, &coverageResponse)
		}
	}
	if counter == 0 {
		err = errors.New(fmt.Sprintf("No Rows Found for: %v", req))
	}
	sklog.Debugf("Responses: %s", responses)
	return responses, err
}

func (s *CoverageStore) sqlExecUpdate(ctx context.Context, sqlStatement string, req *pb.CoverageChangeRequest) (int64, error) {
	var result pgconn.CommandTag
	var err error
	var rows int64

	for _, suite := range req.GetTestSuiteName() {
		result, err = s.db.Exec(ctx, sqlStatement, req.GetFileName(),
			req.GetBuilderName(), suite)
		if err != nil {
			sklog.Errorf("Update Failed")
			return 0, err
		}
		rows += result.RowsAffected()
	}
	return rows, nil
}

func (s *CoverageStore) sqlExecInsert(ctx context.Context, sqlStatement string, req *pb.CoverageChangeRequest) (int64, error) {
	result, err := s.db.Exec(ctx, sqlStatement, req.GetFileName(),
		req.GetBuilderName(), req.GetTestSuiteName())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (s *CoverageStore) sqlExecDelete(ctx context.Context, sqlStatement string, req *pb.CoverageChangeRequest) (int64, error) {
	result, err := s.db.Exec(ctx, sqlStatement, req.GetFileName(), req.GetBuilderName())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
