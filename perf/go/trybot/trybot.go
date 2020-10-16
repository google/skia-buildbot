// Package trybot has common types for the store and ingester sub-modules.
package trybot

import (
	"time"
)

// TryFile represents a single file of trybot results.
type TryFile struct {
	// CL is the Changelist Id.
	CL string

	// PatchNumber is the index of the patch. Note this isn't the git hash of
	// the patch.
	PatchNumber int

	// Filename, including the scheme, gs://, for example.
	Filename string

	// Timestamp of when the file was written.
	Timestamp time.Time
}
