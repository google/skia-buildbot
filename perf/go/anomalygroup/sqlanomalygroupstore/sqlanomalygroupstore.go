package sqlanomalygroupstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"

	pb "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	createGroup statement = iota
	queryGroups
	updateGroup
	getGroup
)

// statements holds all the raw SQL statements used.
var statements = map[statement]string{
	createGroup: `
		INSERT INTO
			AnomalyGroups (anomalies, group_meta_data, common_rev_start, common_rev_end, action)
		VALUES
			($1, $2, $3, $4, $5)
		RETURNING
			id
		`,
}

type AnomalyGroupStore struct {
	//
	db     pool.Pool
	dbType config.DataStoreType
}

func New(db pool.Pool, dbType config.DataStoreType) (*AnomalyGroupStore, error) {
	return &AnomalyGroupStore{
		db:     db,
		dbType: dbType,
	}, nil
}

func (s *AnomalyGroupStore) Create(
	ctx context.Context,
	subscription_name string,
	subscription_revision string,
	domain_name string,
	benchmark_name string,
	start int64,
	end int64,
	action string) (string, error) {

	// Sanity checks
	if len(subscription_name) == 0 || len(subscription_revision) == 0 || len(domain_name) == 0 || len(benchmark_name) == 0 || len(action) == 0 {
		return "", errors.New("empty strings found in string parameters")
	}
	if end <= 0 || start <= 0 {
		return "", errors.New("negative commit position detected")
	}
	if end < start {
		return "", errors.New("the end commit position is smaller than the start commit position")
	}

	// SQL to create a anomaly group
	statement := `
		INSERT INTO
			AnomalyGroups (group_meta_data, common_rev_start, common_rev_end, action)
		VALUES
			($1, $2, $3, $4)
		RETURNING
			id
		`

	metadata := map[string]string{
		"subscription_name":     subscription_name,
		"subscription_revision": subscription_revision,
		"domain_name":           domain_name,
		"benchmark_name":        benchmark_name,
	}
	metadata_string, err := json.Marshal(metadata)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to convert group metadata json.")
	}
	new_group_id := ""
	err = s.db.QueryRow(
		ctx, statement, metadata_string, start, end, action).Scan(&new_group_id)
	if err != nil {
		return "", skerr.Wrapf(err, "Failed to create new anomaly group.")
	} else {
		return string(new_group_id), nil
	}
}

func (s *AnomalyGroupStore) LoadById(
	ctx context.Context,
	group_id string) (*pb.AnomalyGroup, error) {

	// Sanity checks
	if _, err := uuid.Parse(group_id); err != nil {
		err_msg := fmt.Sprintf("group id is not a valid uuid: %s.", group_id)
		return nil, skerr.Wrapf(err, "%s", err_msg)
	}

	statement := `
		SELECT
			id, action, anomaly_ids, culprit_ids, group_meta_data->>'subscription_name', group_meta_data->>'subscription_revision', group_meta_data->>'benchmark_name'
		FROM
			AnomalyGroups
		WHERE
			id=$1
		`

	var loaded_group_id string
	var action string
	var anomaly_ids []string
	var culprit_ids []string
	var subscription_name string
	var subscription_revision string
	var benchmark_name string
	if err := s.db.QueryRow(ctx, statement, group_id).Scan(&loaded_group_id, &action, &anomaly_ids, &culprit_ids, &subscription_name, &subscription_revision, &benchmark_name); err != nil {
		err_msg := fmt.Sprintf("failed to load the anomaly group: %s", group_id)
		return nil, skerr.Wrapf(err, "%s", err_msg)
	}

	return &pb.AnomalyGroup{
		GroupId:              loaded_group_id,
		GroupAction:          pb.GroupActionType(pb.GroupActionType_value[action]),
		AnomalyIds:           anomaly_ids,
		CulpritIds:           culprit_ids,
		SubsciptionName:      subscription_name,
		SubscriptionRevision: subscription_revision,
		BenchmarkName:        benchmark_name,
	}, nil
}

func (s *AnomalyGroupStore) UpdateBisectID(ctx context.Context, group_id string, bisection_id string) error {
	if len(bisection_id) > 0 {
		if _, err := uuid.Parse(bisection_id); err != nil {
			err_msg := fmt.Sprintf("invalid UUID value for updating bisection_id column with value %s ", bisection_id)
			return errors.New(err_msg)
		}
	}
	statement := `
		UPDATE
			AnomalyGroups
		SET
			bisection_id=$1
		WHERE
			id=$2
	`
	if _, err := s.db.Exec(ctx, statement, bisection_id, group_id); err != nil {
		return fmt.Errorf(
			"error updating bisection id for anomaly group: %s. group_id: %s, bisect_id: %s",
			err, group_id, bisection_id)
	}
	return nil
}

func (s *AnomalyGroupStore) UpdateReportedIssueID(ctx context.Context, group_id string, reported_issue_id string) error {
	if len(reported_issue_id) > 0 {
		if _, err := uuid.Parse(reported_issue_id); err != nil {
			err_msg := fmt.Sprintf("invalid UUID value for updating reported_issue_id column with value %s ", reported_issue_id)
			return errors.New(err_msg)
		}
	}
	statement := `
		UPDATE
			AnomalyGroups
		SET
			reported_issue_id=$1
		WHERE
			id=$2
	`
	if _, err := s.db.Exec(ctx, statement, reported_issue_id, group_id); err != nil {
		return fmt.Errorf("error updating anomaly group table: %s. %s", err, group_id)
	}
	return nil
}

func (s *AnomalyGroupStore) AddAnomalyID(ctx context.Context, group_id string, anomaly_id string) error {
	if len(anomaly_id) > 0 {
		if _, err := uuid.Parse(anomaly_id); err != nil {
			err_msg := fmt.Sprintf("invalid UUID value for updating anomaly_id column with value %s ", anomaly_id)
			return errors.New(err_msg)
		}
	}
	statement := `
		UPDATE
			AnomalyGroups
		SET
			anomaly_ids=array_append(anomaly_ids, $1)
		WHERE
			id=$2
	`
	if s.dbType == config.SpannerDataStoreType {
		statement = `
			UPDATE
				AnomalyGroups
			SET
				anomaly_ids=COALESCE(anomaly_ids, ARRAY[]::text[]) || ARRAY[$1]
			WHERE
				id=$2
		`
	}

	if _, err := s.db.Exec(ctx, statement, anomaly_id, group_id); err != nil {
		return fmt.Errorf("error updating anomaly group table: %s. %s", err, group_id)
	}
	return nil
}

func (s *AnomalyGroupStore) AddCulpritIDs(ctx context.Context, group_id string, culprit_ids []string) error {
	for _, v := range culprit_ids {
		if _, err := uuid.Parse(v); err != nil {
			err_msg := fmt.Sprintf("invalid UUID value for updating culprit_ids column with value %s ", culprit_ids)
			return errors.New(err_msg)
		}
	}
	statement := `
		UPDATE
			AnomalyGroups
		SET
			culprit_ids=array_cat(culprit_ids, $1)
		WHERE
			id=$2
	`
	if s.dbType == config.SpannerDataStoreType {
		statement = `
			UPDATE
				AnomalyGroups
			SET
				culprit_ids=COALESCE(culprit_ids, ARRAY[]::text[]) || $1
			WHERE
				id=$2
		`
	}
	if _, err := s.db.Exec(ctx, statement, culprit_ids, group_id); err != nil {
		return fmt.Errorf("error updating anomaly group table: %s. %s", err, group_id)
	}
	return nil
}

func (s *AnomalyGroupStore) FindExistingGroup(
	ctx context.Context,
	subscription_name string,
	subscription_revision string,
	domain_name string,
	benchmark_name string,
	start_commit int64,
	end_commit int64,
	action string) ([]*pb.AnomalyGroup, error) {
	// sanity check
	if len(subscription_name) == 0 || len(subscription_revision) == 0 || len(domain_name) == 0 || len(benchmark_name) == 0 || len(action) == 0 || start_commit <= 0 || end_commit <= 0 {
		err_msg := fmt.Sprintf(
			"invalid params when finding related groups. Params: %s, %s, %s, %s, %s, %d, %d",
			action, subscription_name, subscription_revision, domain_name, benchmark_name,
			end_commit, start_commit)
		return nil, errors.New(err_msg)
	}

	statement := `
		SELECT
			id, action, anomaly_ids, culprit_ids
		FROM
			AnomalyGroups
		WHERE
			action=$1
			AND group_meta_data ->> 'subscription_name'=$2
			AND group_meta_data ->> 'subscription_revision'=$3
			AND group_meta_data ->> 'domain_name'=$4
			AND group_meta_data ->> 'benchmark_name'=$5
			AND common_rev_start<=$6
			AND common_rev_end>=$7
	`

	rows, err := s.db.Query(ctx, statement,
		action, subscription_name, subscription_revision, domain_name, benchmark_name,
		end_commit, start_commit)
	if err != nil {
		err_msg := fmt.Sprintf(
			"failed when finding related groups. Params: %s, %s, %s, %s, %s, %d, %d",
			action, subscription_name, subscription_revision, domain_name, benchmark_name,
			end_commit, start_commit)
		return nil, skerr.Wrapf(err, "%s", err_msg)
	}
	var groups []*pb.AnomalyGroup
	for rows.Next() {
		var loaded_group_id string
		var loaded_action string
		var anomaly_ids []string
		var culprit_ids []string
		if err = rows.Scan(&loaded_group_id, &loaded_action, &anomaly_ids, &culprit_ids); err != nil {
			err_msg := fmt.Sprintf(
				"error parsing the returned group values: %s, %s, %s, %s",
				loaded_group_id, loaded_action, anomaly_ids, culprit_ids)
			return nil, skerr.Wrapf(err, "%s", err_msg)
		} else {
			groups = append(groups, &pb.AnomalyGroup{
				GroupId:     loaded_group_id,
				GroupAction: pb.GroupActionType(pb.GroupActionType_value[action]),
				AnomalyIds:  anomaly_ids,
				CulpritIds:  culprit_ids,
			})
		}
	}
	return groups, nil
}
