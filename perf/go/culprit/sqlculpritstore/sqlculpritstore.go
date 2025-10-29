// Package storage implements culprit.Store using SQL.
package sqlculpritstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	statement := "SELECT id, host, project, ref, revision, anomaly_group_ids, issue_ids, group_issue_map FROM Culprits where id IN (%s)"
	query := fmt.Sprintf(statement, quotedSlice(ids))
	sklog.Debugf("[CP] Get query: %s", query)
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
		var group_issue_map_in_jsonb pgtype.Text
		if err := rows.Scan(&id, &host, &project, &ref, &revision, &anomaly_group_ids, &issue_ids, &group_issue_map_in_jsonb); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read Culprit results")
		}
		var group_issue_map map[string]string
		if group_issue_map_in_jsonb.String != "" {
			err = json.Unmarshal([]byte(group_issue_map_in_jsonb.String), &group_issue_map)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to unmarshal the group issue map string %s", group_issue_map_in_jsonb.String)
			}
		}
		resp = append(resp, &pb.Culprit{
			Id:              id,
			Commit:          &pb.Commit{Host: host, Project: project, Ref: ref, Revision: revision},
			AnomalyGroupIds: anomaly_group_ids,
			IssueIds:        issue_ids,
			GroupIssueMap:   group_issue_map,
		})
	}
	return resp, nil
}

func (s *CulpritStore) GetAnomalyGroupIdsForIssueId(ctx context.Context, issueId string) ([]string, error) {
	query := "SELECT agid FROM Culprits, UNNEST(anomaly_group_ids) as agid WHERE $1 = ANY(issue_ids)"
	rows, err := s.db.Query(ctx, query, issueId)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to query anomaly group ids for issue id from culprit")
	}
	var res []string
	for rows.Next() {
		var agid string
		if err := rows.Scan(&agid); err != nil {
			return nil, skerr.Wrapf(err, "failed to read culprit results")
		}
		res = append(res, agid)
	}
	return res, nil
}

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
		statement = "UPDATE Culprits SET anomaly_group_ids=$2, last_modified=$3 WHERE id=$1"
		for _, culprit := range existing_culprits {
			_, err := s.db.Exec(ctx, statement, culprit.Id, culprit.AnomalyGroupIDs, current_time)
			if err != nil {
				return nil, skerr.Wrapf(err, "Failed to update culprit with id %s", culprit.Id)
			} else {
				return_ids = append(return_ids, culprit.Id)
			}
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

// Adds issue id to a Culprit row.
func (s *CulpritStore) AddIssueId(ctx context.Context, id string, issue_id string, group_id string) error {
	// Fetch existing anomaly_group_ids
	sklog.Debugf("[CP] AddIssueId. Culprit: %s, Issue: %s, Group: %s.", id, issue_id, group_id)
	culprits, err := s.Get(ctx, []string{id})
	if err != nil {
		return skerr.Wrapf(err, "Error fetching Culprit id")
	}
	if len(culprits) == 0 {
		return skerr.Fmt("No culprit found for id %s", id)
	} else if len(culprits) > 1 {
		panic(fmt.Sprintf("Database invariant broken. More than one culprits found for id : %s", id))
	}
	group_id_exist := false
	for i := 0; i < len(culprits[0].AnomalyGroupIds); i++ {
		if culprits[0].AnomalyGroupIds[0] == group_id {
			group_id_exist = true
			break
		}
	}
	if !group_id_exist {
		return skerr.Fmt("adding issue %s for group %s which is not related to the culprit %s", issue_id, group_id, id)
	}

	issue_ids := culprits[0].IssueIds
	issue_ids = append(issue_ids, issue_id)
	issue_ids = removeDuplicateStr(issue_ids)

	group_issue_map := culprits[0].GroupIssueMap
	if group_issue_map == nil {
		group_issue_map = map[string]string{}
	}
	if _, ok := group_issue_map[group_id]; ok {
		return skerr.Fmt("group id %s has related issue already: %s", group_id, err)
	}
	group_issue_map[group_id] = issue_id
	group_issue_map_in_jsonb, err := json.Marshal(group_issue_map)
	if err != nil {
		return skerr.Wrapf(err, "Error marshal group issue map: %s", group_issue_map)
	}
	statement := `
		UPDATE
			Culprits
		SET
			issue_ids=$1, group_issue_map=$2
		WHERE
			id=$3
	`
	if _, err := s.db.Exec(ctx, statement, issue_ids, group_issue_map_in_jsonb, id); err != nil {
		return fmt.Errorf("error adding issue_id %s to culprit %s: %s ", issue_id, id, err)
	}
	return nil
}

func removeDuplicateStr(strSlice []string) []string {
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range strSlice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
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
