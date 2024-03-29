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
	db pool.Pool
}

func New(db pool.Pool) (*AnomalyGroupStore, error) {
	return &AnomalyGroupStore{
		db: db,
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
		return nil, skerr.Wrapf(err, err_msg)
	}

	statement := `
		SELECT
			id, action, anomaly_ids, culprit_ids
		FROM
			AnomalyGroups
		WHERE
			id=$1
		`

	var loaded_group_id string
	var action string
	var anomaly_ids []string
	var culprit_ids []string
	if err := s.db.QueryRow(ctx, statement, group_id).Scan(&loaded_group_id, &action, &anomaly_ids, &culprit_ids); err != nil {
		err_msg := fmt.Sprintf("failed to load the anomaly group: %s", group_id)
		return nil, skerr.Wrapf(err, err_msg)
	}

	return &pb.AnomalyGroup{
		GroupId:     loaded_group_id,
		GroupAction: action,
		AnomalyIds:  anomaly_ids,
		CulpritIds:  culprit_ids,
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
	if _, err := s.db.Exec(ctx, statement, culprit_ids, group_id); err != nil {
		return fmt.Errorf("error updating anomaly group table: %s. %s", err, group_id)
	}
	return nil
}
