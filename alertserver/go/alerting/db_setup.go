package alerting

/*
	Store/Retrieve alerts in a database.
*/

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.skia.org/infra/go/database"
)

const (
	// Default database parameters.
	PROD_DB_HOST = "173.194.253.125"
	PROD_DB_PORT = 3306
	PROD_DB_NAME = "alerts"

	TABLE_ACTIONS  = "actions"
	TABLE_ALERTS   = "alerts"
	TABLE_COMMENTS = "comments"
)

var (
	DB *sqlx.DB = nil
)

// InitDB sets up the database to be shared across the app.
func InitDB(conf *database.DatabaseConfig) error {
	db, err := sqlx.Open("mysql", conf.MySQLString)
	if err != nil {
		return fmt.Errorf("Failed to open database: %v", err)
	}
	DB = db
	return nil
}

var v1_up = []string{
	`CREATE TABLE IF NOT EXISTS alerts (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		active BOOLEAN NOT NULL,
		name VARCHAR(100) NOT NULL,
		triggered BIGINT NOT NULL,
		category VARCHAR(100) NOT NULL,
		message TEXT NOT NULL,
		nag BIGINT,
		snoozedUntil BIGINT,
		dismissedAt BIGINT,
		INDEX idx_active (active),
		INDEX idx_name (name),
		INDEX idx_activecategory (active,category)
	)`,
	`CREATE TABLE IF NOT EXISTS comments (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		alertId INT UNSIGNED NOT NULL,
		user VARCHAR(100) NOT NULL,
		time BIGINT NOT NULL,
		message TEXT NOT NULL,
		INDEX idx_alertId (alertId),
		FOREIGN KEY (alertId) REFERENCES alerts(id) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS actions (
		id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		alertId INT UNSIGNED NOT NULL,
		action VARCHAR(100) NOT NULL,
		INDEX idx_alertId (alertId),
		FOREIGN KEY (alertId) REFERENCES alerts(id) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
}

var v1_down = []string{
	`DROP TABLE IF EXISTS actions`,
	`DROP TABLE IF EXISTS comments`,
	`DROP TABLE IF EXISTS alerts`,
}

var v2_up = []string{
	`ALTER TABLE alerts ADD COLUMN autoDismiss BIGINT NOT NULL;`,
	`ALTER TABLE alerts ADD COLUMN lastFired BIGINT NOT NULL;`,
}

var v2_down = []string{
	`ALTER TABLE alerts DROP COLUMN autoDismiss;`,
	`ALTER TABLE alerts DROP COLUMN lastFired;`,
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
	// version 1. Create tables.
	{
		MySQLUp:   v1_up,
		MySQLDown: v1_down,
	},
	// version 2. AutoDismiss support.
	{
		MySQLUp:   v2_up,
		MySQLDown: v2_down,
	},
}

// MigrationSteps returns the database migration steps.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}
