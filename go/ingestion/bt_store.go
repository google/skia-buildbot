package ingestion

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/sklog"
)

const (
	// TABLE_FILES_PROCESSED is the table to keep track of processed files.
	TABLE_FILES_PROCESSED = "files-processed"

	// COLFAM_FILES_PROCESSED is the column family used to keep track of processed files.
	COLFAM_FILES_PROCESSED = "fproc"

	// COL_STATE is the column used to keep track of processed files.
	COL_STATE = "st"
)

var (
	// VAL_TRUE is a true value.
	VAL_TRUE = []byte("t")

	// BigTableConfig describes the tables and column families used by this
	// package. It can be used by bt.InitBigtable to set up the tables.
	BigTableConfig = bt.TableConfig{
		TABLE_FILES_PROCESSED: {
			COLFAM_FILES_PROCESSED,
		},
	}
)

// BTIStore implementes the IngestionStore interface.
type BTIStore struct {
	projectID  string
	instanceID string
	nameSpace  string
	client     *bigtable.Client
}

// NewBTIStore creates a BigTable backed implemenation of IngestionStore.
// nameSpace is a prefix that is added to every row key to allow multitenancy.
func NewBTIStore(projectID, bigTableInstance, nameSpace string) (IngestionStore, error) {
	if nameSpace == "" {
		return nil, sklog.FmtErrorf("Namespace cannot be empty")
	}

	ret := &BTIStore{
		projectID:  projectID,
		instanceID: bigTableInstance,
		nameSpace:  nameSpace,
	}

	// Create the client.
	ctx := context.TODO()
	var err error
	ret.client, err = bigtable.NewClient(ctx, projectID, bigTableInstance)
	if err != nil {
		return nil, sklog.FmtErrorf("Error creating client: %s", err)
	}
	return ret, nil
}

// ContainsResultFileHash implements the IngestionStore interface.
func (b *BTIStore) ContainsResultFileHash(md5Sum string) (bool, error) {
	ctx := context.TODO()
	rowKey := b.getResultFileRowKey(md5Sum)
	tbl := b.client.Open(TABLE_FILES_PROCESSED)

	row, err := tbl.ReadRow(ctx, rowKey)
	if err != nil {
		return false, err
	}

	if len(row) == 0 {
		return false, nil
	}

	if len(row[COLFAM_FILES_PROCESSED]) > 1 {
		return false, sklog.FmtErrorf("Received %d values for %s:%s", len(row[COLFAM_FILES_PROCESSED]), COLFAM_FILES_PROCESSED, COL_STATE)
	}

	return len(row[COLFAM_FILES_PROCESSED]) == 1, nil
}

// SetResultFileHash implements the IngestionStore interface.
func (b *BTIStore) SetResultFileHash(md5Sum string) error {
	ctx := context.TODO()
	rowKey := b.getResultFileRowKey(md5Sum)
	tbl := b.client.Open(TABLE_FILES_PROCESSED)

	now := bigtable.Now()
	setMut := bigtable.NewMutation()
	setMut.Set(COLFAM_FILES_PROCESSED, COL_STATE, now, VAL_TRUE)

	if err := tbl.Apply(ctx, rowKey, setMut); err != nil {
		return sklog.FmtErrorf("Error setting processed file value: %s", err)
	}

	// Delete an previous set value.
	delMut := bigtable.NewMutation()
	delMut.DeleteTimestampRange(COLFAM_FILES_PROCESSED, COL_STATE, 0, now)

	if err := tbl.Apply(ctx, rowKey, delMut); err != nil {
		return sklog.FmtErrorf("Error deleting old timestamps: %s", err)
	}

	return nil
}

// Clear implements the IngestionStore interface.
func (b *BTIStore) Clear() error {
	ctx := context.TODO()
	tbl := b.client.Open(TABLE_FILES_PROCESSED)

	// Get all keys and delete them.
	delMut := bigtable.NewMutation()
	delMut.DeleteRow()
	allRows := bigtable.InfiniteRange("")
	allKeys := []string{}
	muts := []*bigtable.Mutation{}
	err := tbl.ReadRows(ctx, allRows, func(r bigtable.Row) bool {
		allKeys = append(allKeys, r.Key())
		muts = append(muts, delMut)
		return true
	})
	if err != nil {
		return err
	}

	_, err = tbl.ApplyBulk(ctx, allKeys, muts)
	return err
}

// Close implement the IngestionStore interface.
func (b *BTIStore) Close() error {
	return b.client.Close()
}

// Returns the row key for keeping track of file hashes.
func (b *BTIStore) getResultFileRowKey(md5Sum string) string {
	return fmt.Sprintf("%s:%s", b.nameSpace, md5Sum)
}
