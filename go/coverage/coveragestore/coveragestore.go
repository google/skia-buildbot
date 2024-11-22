package coveragestore

import (
	"context"

	pb "go.skia.org/infra/go/coverage/proto/v1"
)

// CoverageSchema represents the SQL schema of the Coverage table.
type CoverageRequest struct {
	ID int `sql:"id INT PRIMARY KEY DEFAULT unique_rowid()"`

	// The relative file path and filename of source file.
	FileName int `sql:"file_name STRING"`

	// An Builder serialized as JSON that includes test suites.
	Builder string `sql:"builder TEXT"`

	// Stored as a Unit timestamp.
	LastModified int `sql:"last_modified INT"`
}

// Store is the interface used to persist Coverage.
type Store interface {
	// Add will insert a new file with associated builder information.
	Add(ctx context.Context, req *pb.CoverageChangeRequest) error

	// Delete removes the Filename with the given filename.
	Delete(ctx context.Context, req *pb.CoverageChangeRequest) error

	// List retrieves all the Coverage mapppings.
	List(ctx context.Context, req *pb.CoverageListRequest) ([]string, error)

	// List retrieves all the Coverage mapppings.
	ListAll(ctx context.Context, req *pb.CoverageRequest) ([]*pb.CoverageResponse, error)
}
