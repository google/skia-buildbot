package ingestion

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"
)

const (
	// table_filesProcessed is the table to keep track of processed files.
	TABLE_FILES_PROCESSED = "files-processed"

	// colfam_filesProcessed is the column family used to keep track of processed files.
	COLFAM_FILES_PROCESSED = "fproc"

	// col_state is the column used to keep track of processed files.
	COL_STATE = "st"
)

var (
	// True value to store in a table - also used as dummy value if we don't care
	// about the actual cell value.
	VAL_TRUE = []byte("t")

	// TABLES_COLUMN_FAMILIES maps from tableName to list of column families. This is
	// used to create the column families when instantiating BTIStore.
	TABLES_COLUMN_FAMILIES = map[string][]string{
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

	// Make sure the tables exists.
	if err := ret.initTables(); err != nil {
		return nil, err
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
	adminClient, err := bigtable.NewAdminClient(ctx, b.projectID, b.instanceID)
	if err != nil {
		return sklog.FmtErrorf("Unable to create admin client: %s", err)
	}

	var egroup errgroup.Group
	for tableName := range TABLES_COLUMN_FAMILIES {
		func(tableName string) {
			egroup.Go(func() error {
				return adminClient.DeleteTable(ctx, tableName)
			})
		}(tableName)
	}
	return egroup.Wait()
}

// Returns the row key for keeping track of file hashes.
func (b *BTIStore) getResultFileRowKey(md5Sum string) string {
	return fmt.Sprintf("%s:%s", b.nameSpace, md5Sum)
}

// initTables make sure all tables and column families exist in the BT instance.
func (b *BTIStore) initTables() error {
	ctx := context.TODO()

	// Set up admin client, tables, and column families.
	adminClient, err := bigtable.NewAdminClient(ctx, b.projectID, b.instanceID)
	if err != nil {
		return sklog.FmtErrorf("Unable to create admin client: %s", err)
	}

	tablesSlice, err := adminClient.Tables(ctx)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving tables: %s", err)
	}

	tables := util.NewStringSet(tablesSlice)
	for tableName, colFamilies := range TABLES_COLUMN_FAMILIES {
		if !tables[tableName] {
			if err := adminClient.CreateTable(ctx, tableName); err != nil {
				return sklog.FmtErrorf("Error creating table %s: %s", err)
			}
			sklog.Infof("Created table: %s", tableName)
		}

		tableInfo, err := adminClient.TableInfo(ctx, tableName)
		if err != nil {
			return sklog.FmtErrorf("Error getting table info: %s", err)
		}

		allColFamilies := util.NewStringSet(tableInfo.Families)
		for _, colFamName := range colFamilies {
			if !allColFamilies[colFamName] {
				if err := adminClient.CreateColumnFamily(ctx, tableName, colFamName); err != nil {
					return sklog.FmtErrorf("Error creating column family %s in table %s: %s", colFamName, tableName, err)
				}
			}
		}
	}
	return nil
}
