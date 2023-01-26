-- First start a local cockroachdb instance:
--
--   cockroach start-single-node --insecure --listen-addr=127.0.0.1
--
--   cockroach sql --insecure --host=127.0.0.1 < ./explore.sql
--
-- You should be able to run this file against the same database more than once
-- w/o error.
--
DROP TABLE IF EXISTS Description;

-- V2 Schema
CREATE TABLE IF NOT EXISTS Description (
	maintenance_mode STRING NOT NULL DEFAULT '',
	is_quarantined bool NOT NULL DEFAULT FALSE,
	recovering STRING NOT NULL DEFAULT '',
	attached_device STRING NOT NULL DEFAULT 'nodevice',
	annotation jsonb NOT NULL,
	note jsonb NOT NULL,
	version STRING NOT NULL DEFAULT '',
	powercycle bool NOT NULL DEFAULT FALSE,
	powercycle_state STRING NOT NULL DEFAULT 'not_available',
	last_updated timestamptz NOT NULL,
	battery int NOT NULL DEFAULT 0,
	temperatures jsonb NOT NULL,
	running_swarmingTask bool NOT NULL DEFAULT FALSE,
	launched_swarming bool NOT NULL DEFAULT FALSE,
	recovery_start timestamptz NOT NULL,
	device_uptime int4 DEFAULT 0,
	ssh_user_ip STRING NOT NULL DEFAULT '',
	supplied_dimensions jsonb NOT NULL,
	dimensions jsonb NOT NULL,
	task_request jsonb,
	task_started timestamptz NOT NULL,
	machine_id STRING PRIMARY KEY AS (dimensions -> 'id' ->> 0) STORED,
	INVERTED INDEX dimensions_gin (dimensions),
	INDEX by_powercycle (powercycle),
);

SHOW COLUMNS
FROM
	description;

DROP TABLE IF EXISTS Description;

-- V1 Schema
CREATE TABLE IF NOT EXISTS Description (
	maintenance_mode STRING NOT NULL DEFAULT '',
	is_quarantined bool NOT NULL DEFAULT FALSE,
	recovering STRING NOT NULL DEFAULT '',
	attached_device STRING NOT NULL DEFAULT 'nodevice',
	annotation jsonb NOT NULL,
	note jsonb NOT NULL,
	version STRING NOT NULL DEFAULT '',
	powercycle bool NOT NULL DEFAULT FALSE,
	powercycle_state STRING NOT NULL DEFAULT 'not_available',
	last_updated timestamptz NOT NULL,
	battery int NOT NULL DEFAULT 0,
	temperatures jsonb NOT NULL,
	running_swarmingTask bool NOT NULL DEFAULT FALSE,
	launched_swarming bool NOT NULL DEFAULT FALSE,
	recovery_start timestamptz NOT NULL,
	device_uptime int4 DEFAULT 0,
	ssh_user_ip STRING NOT NULL DEFAULT '',
	supplied_dimensions jsonb NOT NULL,
	dimensions jsonb NOT NULL,
	machine_id STRING PRIMARY KEY AS (dimensions -> 'id' ->> 0) STORED,
	INVERTED INDEX dimensions_gin (dimensions),
	INDEX by_powercycle (powercycle)
);

-- Statements needed to migrate from V1 to V2.
ALTER TABLE
	Description
ADD
	COLUMN IF NOT EXISTS task_request jsonb;

ALTER TABLE
	Description
ADD
	COLUMN IF NOT EXISTS task_started timestamptz NOT NULL;

SHOW COLUMNS
FROM
	description;

DROP TABLE IF EXISTS Description;

CREATE TABLE IF NOT EXISTS Description (
	dimensions jsonb NOT NULL,
	maintenance_mode STRING NOT NULL DEFAULT '',
	is_quarantined bool NOT NULL DEFAULT FALSE,
	powercycle bool NOT NULL DEFAULT FALSE,
	machine_id STRING PRIMARY KEY AS (dimensions -> 'id' ->> 0) STORED,
	INDEX by_powercycle (powercycle),
	INVERTED INDEX dimensions_gin (dimensions)
);

INSERT INTO
	Description (powercycle, dimensions)
VALUES
	(
		FALSE,
		'{ "id": ["skia-e-linux-150"], "cores": ["6"], "cpu": ["x86","x86-64"], "os": ["Linux","Debian","Debian-11","Debian-11.2"]}'
	),
	(
		TRUE,
		'{ "id": ["skia-e-linux-151"], "cores": ["6"], "cpu": ["x86","x86-64"], "os": ["Linux","Debian","Debian-11","Debian-11.6"]}'
	),
	(
		TRUE,
		'{ "id": ["skia-e-mac-201"],  "cores": ["8"], "cpu": ["arm","arm-64"], "os": ["Mac","Mac-12","Mac-12.1"]}'
	) ON CONFLICT DO NOTHING;

SELECT
	'Show how to do a single param query' AS desc;

SELECT
	machine_id
FROM
	Description
WHERE
	-- Note that value of the key-value pair still needs to be inside an array:
	dimensions @ > '{"cpu": ["arm-64"]}';

SELECT
	'Demonstrate we are not doing full table scans.' AS desc;

EXPLAIN
SELECT
	machine_id
FROM
	Description
WHERE
	dimensions @ > '{"cpu": ["x86-64"]}'
	AND dimensions @ > '{"os": ["Debian-11.6"]}';

SELECT
	machine_id
FROM
	Description
WHERE
	dimensions @ > '{"cpu": ["x86-64"]}'
	AND dimensions @ > '{"os": ["Debian-11.6"]}';

SELECT
	'Force Quarantine a machine' AS desc;

UPDATE
	Description
SET
	is_quarantined = TRUE
WHERE
	dimensions @ > '{"id": ["skia-e-linux-150"]}';

SELECT
	machine_id
FROM
	Description
WHERE
	is_quarantined = FALSE;

SELECT
	'List powercycle' AS desc;

SELECT
	machine_id
FROM
	Description @by_powercycle
WHERE
	powercycle = TRUE;

SELECT
	'Demonstrate we are not doing full table scans.' AS desc;

EXPLAIN
SELECT
	machine_id
FROM
	Description @by_powercycle
WHERE
	powercycle = TRUE;

SELECT
	'Get by id' AS desc;

SELECT
	machine_id
FROM
	Description
WHERE
	machine_id = 'skia - e - linux - 151';