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
	filesProcessedTable  = "files-processed"
	filesProcessedColFam = "fproc"
	stateCol             = "st"
)

var (
	// True value to store in a table - also used as dummy value if we don't care
	// about the actual cell value.
	valTrue = []byte("t")

	// tableFamilies maaps from tableName to list of column families. This is
	// used to create the column families when instantiating BTIStore.
	tableFamilies = map[string][]string{
		filesProcessedTable: {
			filesProcessedColFam,
		},
	}
)

type BTIStore struct {
	projectID  string
	instanceID string
	nameSpace  string
	client     *bigtable.Client
}

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

func (b *BTIStore) ContainsResultFileHash(md5Sum string) (bool, error) {
	ctx := context.TODO()
	rowKey := b.getResultFileRowKey(md5Sum)
	tbl := b.client.Open(filesProcessedTable)

	row, err := tbl.ReadRow(ctx, rowKey)
	if err != nil {
		return false, err
	}

	if len(row) == 0 {
		return false, nil
	}

	if len(row[filesProcessedColFam]) > 1 {
		return false, sklog.FmtErrorf("Received %d values for %s:%s", len(row[filesProcessedColFam]), filesProcessedColFam, stateCol)
	}

	return len(row[filesProcessedColFam]) == 1, nil
}

func (b *BTIStore) SetResultFileHash(md5Sum string) error {
	ctx := context.TODO()
	rowKey := b.getResultFileRowKey(md5Sum)
	tbl := b.client.Open(filesProcessedTable)

	now := bigtable.Now()
	setMut := bigtable.NewMutation()
	setMut.Set(filesProcessedColFam, stateCol, now, valTrue)

	if err := tbl.Apply(ctx, rowKey, setMut); err != nil {
		return sklog.FmtErrorf("Error setting processed file value: %s", err)
	}

	// Delete an previous set value.
	delMut := bigtable.NewMutation()
	delMut.DeleteTimestampRange(filesProcessedColFam, stateCol, 0, now)

	if err := tbl.Apply(ctx, rowKey, delMut); err != nil {
		return sklog.FmtErrorf("Error deleting old timestamps: %s", err)
	}

	return nil
}

func (b *BTIStore) Clear() error {
	ctx := context.TODO()
	adminClient, err := bigtable.NewAdminClient(ctx, b.projectID, b.instanceID)
	if err != nil {
		return sklog.FmtErrorf("Unable to create admin client: %s", err)
	}

	var egroup errgroup.Group
	for tableName := range tableFamilies {
		func(tableName string) {
			egroup.Go(func() error {
				return adminClient.DeleteTable(ctx, tableName)
			})
		}(tableName)
	}
	return egroup.Wait()
}

func (b *BTIStore) getResultFileRowKey(md5Sum string) string {
	return fmt.Sprintf("%s:%s", b.nameSpace, md5Sum)
}

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
	for tableName, colFamilies := range tableFamilies {
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
					sklog.FmtErrorf("Error creating column family %s in table %s: %s", colFamName, tableName, err)
				}
			}
		}
	}
	return nil
}
