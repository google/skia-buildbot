package sqlalertstore

// spannerStatements holds all the raw SQL statements used when running against a spanner database.
var spannerStatements = map[statement]string{
	insertAlert: `
		INSERT INTO
			Alerts (alert, last_modified, sub_name, sub_revision)
		VALUES
			($1, $2, $3, $4)
		RETURNING
			id
		`,
	updateAlert: `
		INSERT INTO
			Alerts (id, alert, config_state, last_modified, sub_name, sub_revision)
		VALUES
			($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET id=EXCLUDED.id, alert=EXCLUDED.alert, config_state=EXCLUDED.config_state,
			last_modified=EXCLUDED.last_modified, sub_name=EXCLUDED.sub_name,
			sub_revision=EXCLUDED.sub_revision
		`,
	deleteAlert: `
		UPDATE
		  	Alerts
		SET
			config_state=1, -- alerts.DELETED
			last_modified=$1
		WHERE
			id=$2
		`,
	deleteAllAlerts: `
		UPDATE
			Alerts
		SET
			config_state=1, -- alerts.DELETED
			last_modified=$1
		WHERE
			config_state=0 -- alerts.ACTIVE
	`,
	listActiveAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		WHERE
			config_state=0 -- alerts.ACTIVE
		`,
	listAllAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		`,
}
