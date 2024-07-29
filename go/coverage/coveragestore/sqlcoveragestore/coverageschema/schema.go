package coverageschema

import "time"

// CoverageSchema represents the SQL schema of the Coverage table.
type CoverageSchema struct {
	ID string `sql:"id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

	// Relative path of filename.
	FileName string `sql:"file_name STRING NOT NULL"`

	// Name of builder.
	BuilderName string `sql:"builder_name STRING NOT NULL"`

	// Name of builder.
	TestSuiteName []string `sql:"test_suite_name STRING ARRAY NOT NULL"`

	// Stored as a Unit timestamp.
	LastModified time.Time `sql:"last_modified TIMESTAMPTZ DEFAULT now()"`
}
