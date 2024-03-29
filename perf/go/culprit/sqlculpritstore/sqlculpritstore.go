// Package storage implements culprit.Store using SQL.
//
// Please see perf/sql/migrations for the database schema used.
package sqlculpritstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/sqlutil"

	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/culprit/sqlculpritstore/schema"
)

// CulpritStore implements the culprits.Store interface.
type CulpritStore struct {
	// db is the database interface.
	db pool.Pool
}

// New returns a new *CulpritStore.
func New(db pool.Pool) (*CulpritStore, error) {
	return &CulpritStore{
		db: db,
	}, nil
}

// Returns Culprit objects corresponding to given set of ids.
func (s *CulpritStore) Get(ctx context.Context, ids []string) ([]*pb.Culprit, error) {
	statement := "SELECT id, host, project, ref, revision, anomaly_group_ids, issue_ids FROM Culprits where id IN (%s)"
	query := fmt.Sprintf(statement, quotedSlice(ids))
	fmt.Println(query)
	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query Culprit")
	}
	var resp []*pb.Culprit
	for rows.Next() {
		var id string
		var host string
		var project string
		var ref string
		var revision string
		var anomaly_group_ids []string
		var issue_ids []string
		if err := rows.Scan(&id, &host, &project, &ref, &revision, &anomaly_group_ids, &issue_ids); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read Culprit results")
		}
		resp = append(resp, &pb.Culprit{
			Commit:          &pb.Commit{Host: host, Project: project, Ref: ref, Revision: revision},
			AnomalyGroupIds: anomaly_group_ids,
			IssueIds:        issue_ids,
		})
	}
	return resp, nil
}

// Upsert implements the culprit.Store interface.
// Inserts the given culprit elements in the persistant storage. If a culprit already exists,
// appends the anomaly_group_id into its corresponding field.
func (s *CulpritStore) Upsert(ctx context.Context, anomaly_group_id string, ip_commits []*pb.Commit) ([]string, error) {
	if len(ip_commits) <= 0 || anomaly_group_id == "" {
		return nil, skerr.Fmt("no culprits/anomaly_group_id provided")
	}
	for _, commit := range ip_commits {
		if commit.Host != ip_commits[0].Host ||
			commit.Project != ip_commits[0].Project ||
			commit.Ref != ip_commits[0].Ref {
			return nil, skerr.Fmt(
				"all culprits should have same host, project and ref value")
		}
	}

	// Fetch existing anomaly_group_ids
	whereCols := []string{"host", "project", "ref", "revision"}
	statement := "SELECT id, revision, anomaly_group_ids FROM Culprits WHERE "
	statement += sqlutil.WherePlaceholders(whereCols, len(ip_commits))
	arguments := make([]interface{}, 0, len(whereCols)*len(ip_commits))
	for _, commit := range ip_commits {
		arguments = append(
			arguments, commit.Host, commit.Project,
			commit.Ref,
			commit.Revision)
	}
	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to query Culprit")
	}

	// Collect anomaly_group_ids and merge with anomaly_group_id in request
	culprits_map := convertProtoToSchema(anomaly_group_id, ip_commits)
	for rows.Next() {
		var row_id string
		var row_revision string
		var row_group_ids []string
		if err := rows.Scan(&row_id, &row_revision, &row_group_ids); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read Culprit results")
		}
		culprits_map[row_revision].Id = row_id
		culprits_map[row_revision].AnomalyGroupIDs =
			append(culprits_map[row_revision].AnomalyGroupIDs, row_group_ids...)
	}
	existing_culprits, new_culprits := splitIntoExistingAndNew(culprits_map)
	current_time := time.Now().Unix()

	return_ids := make([]string, 0)
	// Update existing culprits into Culprit table
	if len(existing_culprits) > 0 {
		statement = "UPSERT INTO Culprits (id, anomaly_group_ids, last_modified) VALUES"
		const colsPerRow = 3 // should match number of columns in `statement`
		statement += sqlutil.ValuesPlaceholders(colsPerRow, len(existing_culprits))
		arguments = make([]interface{}, 0, colsPerRow*len(existing_culprits))
		ids := make([]string, 0)
		for _, culprit := range existing_culprits {
			arguments = append(arguments, culprit.Id, culprit.AnomalyGroupIDs,
				current_time)
			ids = append(ids, culprit.Id)
		}
		_, err := s.db.Exec(ctx, statement, arguments...)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to upsert culprit")
		} else {
			return_ids = append(return_ids, ids...)
		}
	}

	// Insert new culprits into Culprit table
	if len(new_culprits) > 0 {
		statement = "INSERT INTO Culprits (id, host, project, ref, revision, anomaly_group_ids, last_modified) VALUES "
		const colsPerRow = 7 // should match number of columns in `statement`
		statement += sqlutil.ValuesPlaceholders(colsPerRow, len(new_culprits))
		arguments = make([]interface{}, 0, colsPerRow*len(new_culprits))
		ids := make([]string, 0)
		for _, culprit := range new_culprits {
			id := uuid.NewString()
			arguments = append(arguments, id, culprit.Host,
				culprit.Project, culprit.Ref, culprit.Revision,
				culprit.AnomalyGroupIDs, current_time)
			ids = append(ids, id)
		}
		_, err := s.db.Exec(ctx, statement, arguments...)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to insert culprit")
		} else {
			return_ids = append(return_ids, ids...)
		}
	}
	return return_ids, nil
}

// Takes a string array as input, and returns a comma joined string where each element
// is single quoted.
func quotedSlice(a []string) string {
	q := make([]string, len(a))
	for i, s := range a {
		q[i] = fmt.Sprintf("'%s'", s)
	}
	return strings.Join(q, ", ")
}

// Takes culprit protos and anomaly_group_id as input, and returns a culprit schema struct where
// `AnomalyGroupIds` field is populated
func convertProtoToSchema(anomaly_group_id string, commits []*pb.Commit) map[string]*schema.CulpritSchema {
	resp := make(map[string]*schema.CulpritSchema)
	for _, commit := range commits {
		elem := &schema.CulpritSchema{
			Host:            commit.Host,
			Project:         commit.Project,
			Ref:             commit.Ref,
			Revision:        commit.Revision,
			AnomalyGroupIDs: []string{anomaly_group_id},
		}
		resp[commit.Revision] = elem
	}
	return resp
}

// Takes the map of type returned by convertProtoToSchema() above, and returns two maps, where
// first one represents culprits where ID is set, and second where it isn't.
func splitIntoExistingAndNew(culprits map[string]*schema.CulpritSchema) (map[string]*schema.CulpritSchema, map[string]*schema.CulpritSchema) {
	existing := make(map[string]*schema.CulpritSchema)
	new := make(map[string]*schema.CulpritSchema)
	for revision, culprit := range culprits {
		if len(culprit.Id) > 0 {
			existing[revision] = culprit
		} else {
			new[revision] = culprit
		}
	}
	return existing, new
}
