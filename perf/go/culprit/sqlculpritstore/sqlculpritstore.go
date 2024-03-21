// Package storage implements culprit.Store using SQL.
//
// Please see perf/sql/migrations for the database schema used.
package sqlculpritstore

import (
	"context"
	"errors"
	"time"

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

// Upsert implements the culprit.Store interface.
// Inserts the given culprit elements in the persistant storage. If a culprit already exists,
// appends the anomaly_group_id into its corresponding field.
func (s *CulpritStore) Upsert(ctx context.Context, anomaly_group_id string, ip_culprits []*pb.Culprit) error {
	if len(ip_culprits) <= 0 || anomaly_group_id == "" {
		return skerr.Fmt("no culprits/anomaly_group_id provided")
	}
	for _, culprit := range ip_culprits {
		if culprit.Commit.Host != ip_culprits[0].Commit.Host ||
			culprit.Commit.Project != ip_culprits[0].Commit.Project ||
			culprit.Commit.Ref != ip_culprits[0].Commit.Ref {
			return errors.New(
				"all culprits should have same host, project and ref value")
		}
	}

	// Fetch existing anomaly_group_ids
	whereCols := []string{"host", "project", "ref", "revision"}
	statement := "SELECT id, revision, anomaly_group_ids FROM Culprits WHERE "
	statement += sqlutil.WherePlaceholders(whereCols, len(ip_culprits))
	arguments := make([]interface{}, 0, len(whereCols)*len(ip_culprits))
	for _, culprit := range ip_culprits {
		arguments = append(
			arguments, culprit.Commit.Host, culprit.Commit.Project,
			culprit.Commit.Ref,
			culprit.Commit.Revision)
	}
	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return skerr.Wrapf(err, "Failed to query Culprit")
	}

	// Collect anomaly_group_ids and merge with anomaly_group_id in request
	culprits_map := convertProtoToSchema(anomaly_group_id, ip_culprits)
	for rows.Next() {
		var row_id string
		var row_revision string
		var row_group_ids []string
		if err := rows.Scan(&row_id, &row_revision, &row_group_ids); err != nil {
			return skerr.Wrapf(err, "Failed to read Culprit results")
		}
		culprits_map[row_revision].Id = row_id
		culprits_map[row_revision].AnomalyGroupIDs =
			append(culprits_map[row_revision].AnomalyGroupIDs, row_group_ids...)
	}
	existing_culprits, new_culprits := splitIntoExistingAndNew(culprits_map)
	current_time := time.Now().Unix()

	// Update existing culprits into Culprit table
	if len(existing_culprits) > 0 {
		statement = "UPSERT INTO Culprits (id, anomaly_group_ids, last_modified) VALUES"
		const colsPerRow = 3 // should match number of columns in `statement`
		statement += sqlutil.ValuesPlaceholders(colsPerRow, len(existing_culprits))
		arguments = make([]interface{}, 0, colsPerRow*len(existing_culprits))
		for _, culprit := range existing_culprits {
			arguments = append(arguments, culprit.Id, culprit.AnomalyGroupIDs,
				current_time)
		}
		if _, err := s.db.Exec(ctx, statement, arguments...); err != nil {
			return skerr.Wrapf(err, "Failed to upsert culprit")
		}
	}

	// Insert new culprits into Culprit table
	if len(new_culprits) > 0 {
		statement = "INSERT INTO Culprits (host, project, ref, revision, anomaly_group_ids, last_modified) VALUES "
		const colsPerRow = 6 // should match number of columns in `statement`
		statement += sqlutil.ValuesPlaceholders(colsPerRow, len(new_culprits))
		arguments = make([]interface{}, 0, colsPerRow*len(new_culprits))
		for _, culprit := range new_culprits {

			arguments = append(arguments, culprit.Host,
				culprit.Project, culprit.Ref, culprit.Revision,
				culprit.AnomalyGroupIDs, current_time)
		}
		if _, err := s.db.Exec(ctx, statement, arguments...); err != nil {
			return skerr.Wrapf(err, "Failed to insert culprit")
		}
	}
	return nil
}

// Takes culprit protos and anomaly_group_id as input, and returns a culprit schema struct where
// `AnomalyGroupIds` field is populated
func convertProtoToSchema(anomaly_group_id string, culprits []*pb.Culprit) map[string]*schema.CulpritSchema {
	resp := make(map[string]*schema.CulpritSchema)
	for _, culprit := range culprits {
		elem := &schema.CulpritSchema{
			Host:            culprit.Commit.Host,
			Project:         culprit.Commit.Project,
			Ref:             culprit.Commit.Ref,
			Revision:        culprit.Commit.Revision,
			AnomalyGroupIDs: []string{anomaly_group_id},
		}
		resp[culprit.Commit.Revision] = elem
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
