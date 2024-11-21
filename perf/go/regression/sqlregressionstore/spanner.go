package sqlregressionstore

// spannerStatements are the SQL statements compatible with Spanner postgres.
var spannerStatements = map[statement]string{
	write: `
		INSERT INTO
			Regressions (commit_number, alert_id, regression, migrated)
		VALUES
			($1, $2, $3, false)
		ON CONFLICT (commit_number, alert_id) DO UPDATE
		SET commit_number=EXCLUDED.commit_number, alert_id=EXCLUDED.alert_id,
		regression=EXCLUDED.regression, migrated=EXCLUDED.migrated
		`,
	read: `
		SELECT
			regression
		FROM
			Regressions
		WHERE
			commit_number=$1 AND
			alert_id=$2`,
	readOldest: `
		SELECT
			commit_number
		FROM
			Regressions
		ORDER BY
			commit_number ASC
		LIMIT 1
		`,
	readRange: `
		SELECT
			commit_number, alert_id, regression
		FROM
			Regressions
		WHERE
			commit_number >= $1
			AND commit_number <= $2
		`,
	batchReadMigration: `
		SELECT
			commit_number, alert_id, regression, regression_id
		FROM
			Regressions
		WHERE
			migrated is NULL OR migrated=false
		LIMIT $1
		`,
	markMigrated: `
		UPDATE
			Regressions
		SET
			migrated=true, regression_id=$1
		WHERE
			commit_number=$2 AND alert_id=$3
		`,
	deleteByCommit: `
		DELETE
		FROM
			Regressions
		WHERE
			commit_number=$1
		`,
}
