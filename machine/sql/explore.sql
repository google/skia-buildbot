-- First start a local cockroachdb instance:
--
--   cockroach start-single-node --insecure --listen-addr=127.0.0.1
--
--   cockroach sql --insecure --host=127.0.0.1 < ./explore.sql
--
-- You should be able to run this file against the same database more than once
-- w/o error.
--
DROP TABLE IF EXISTS Machines;

CREATE TABLE IF NOT EXISTS Machines (
	dimensions jsonb NOT NULL,
	maintenance_mode STRING NOT NULL DEFAULT '',
	is_quarantined bool NOT NULL DEFAULT FALSE,
	powercycle bool NOT NULL DEFAULT FALSE,
	machine_id STRING PRIMARY KEY AS (
dimensions -> 'id' ->> 0) STORED,
	INDEX by_powercycle (powercycle),
	INVERTED INDEX dimensions_gin (dimensions)
);

INSERT INTO Machines (
	powercycle,
	dimensions)
VALUES (
	FALSE,
	'{ "id": ["skia-e-linux-150"], "cores": ["6"], "cpu": ["x86","x86-64"], "os": ["Linux","Debian","Debian-11","Debian-11.2"]}'),
(
	TRUE,
	'{ "id": ["skia-e-linux-151"], "cores": ["6"], "cpu": ["x86","x86-64"], "os": ["Linux","Debian","Debian-11","Debian-11.6"]}'),
(
	TRUE,
	'{ "id": ["skia-e-mac-201"],  "cores": ["8"], "cpu": ["arm","arm-64"], "os": ["Mac","Mac-12","Mac-12.1"]}')
ON CONFLICT
	DO NOTHING;

SELECT
	'Show how to do a single param query' AS desc;

SELECT
	machine_id
FROM
	Machines
WHERE
	-- Note that value of the key-value pair still needs to be inside an array:
	dimensions @> '{"cpu": ["arm-64"]}';

SELECT
	'Demonstrate we are not doing full table scans.' AS desc;

EXPLAIN
SELECT
	machine_id
FROM
	Machines
WHERE
	dimensions @> '{"cpu": ["x86-64"]}'
	AND dimensions @> '{"os": ["Debian-11.6"]}';

SELECT
	machine_id
FROM
	Machines
WHERE
	dimensions @> '{"cpu": ["x86-64"]}'
	AND dimensions @> '{"os": ["Debian-11.6"]}';

SELECT
	'Force Quarantine a machine' AS desc;

UPDATE
	Machines
SET
	is_quarantined = TRUE
WHERE
	dimensions @> '{"id": ["skia-e-linux-150"]}';

SELECT
	machine_id
FROM
	Machines
WHERE
	is_quarantined = FALSE;

SELECT
	'List powercycle' AS desc;

SELECT
	machine_id
FROM
	Machines@by_powercycle
WHERE
	powercycle = TRUE;

SELECT
	'Demonstrate we are not doing full table scans.' AS desc;

EXPLAIN
SELECT
	machine_id
FROM
	Machines@by_powercycle
WHERE
	powercycle = TRUE;

SELECT
	'Get by id' AS desc;

SELECT
	machine_id
FROM
	Machines
WHERE
	machine_id = 'skia - e - linux - 151';

