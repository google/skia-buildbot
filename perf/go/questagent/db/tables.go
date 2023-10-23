package sql

import "time"

//go:generate bazelisk run --config=mayberemote //:go -- run ./tosql

// Execution represents a single execution in the Execution table.
type Execution struct {
	ExecutionID   string      `sql:"execution_id UUID NOT NULL DEFAULT gen_random_uuid()"`
	QuestType     string      `sql:"quest_type STRING NOT NULL"`
	Status        string      `sql:"status STRING"`
	CreationTime  time.Time   `sql:"creation_time TIMESTAMPTZ NOT NULL DEFAULT current_timestamp()"`
	StatedTime    time.Time   `sql:"started_time TIMESTAMPTZ"`
	CompletedTime time.Time   `sql:"completed_time TIMESTAMPTZ"`
	Arguments     interface{} `sql:"arguments JSONB"`
	Properties    interface{} `sql:"properties JSONB"`
}

// Tables represents the full schema of the QuestAgent database.
type Tables struct {
	Executions []Execution
}
