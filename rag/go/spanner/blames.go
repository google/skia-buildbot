package spanner

import "google.golang.org/genproto/googleapis/type/datetime"

type BlamedFiles struct {
	Id            string            `sql:"id STRING(36) PRIMARY KEY"`
	FilePath      string            `sql:"file_path STRING(MAX) NOT NULL"`
	FileHash      string            `sql:"file_hash STRING(MAX) NOT NULL"`
	Version       string            `sql:"version STRING(MAX) NOT NULL"`
	CommitHash    string            `sql:"commit_hash STRING(MAX) NOT NULL"`
	LastUpdated   datetime.DateTime `sql:"last_updated TIMESTAMP OPTIONS (allow_commit_timestamp=true)"`
	byFilePathIdx struct{}          `sql:"INDEX by_file_path (file_path)"`
}

type LineBlames struct {
	Id         string   `sql:"id STRING(36)"`
	BlamedFile string   `sql:"blamed_file STRING(MAX) NOT NULL"`
	LineNumber int      `sql:"line_number INT64 NOT NULL"`
	CommitHash string   `sql:"commit_hash STRING(MAX) NOT NULL"`
	pk         struct{} `sql:"PRIMARY KEY (id, line_number)"`
	interleave struct{} `sql:"INTERLEAVE IN PARENT BlamedFiles ON DELETE CASCADE"`
}
