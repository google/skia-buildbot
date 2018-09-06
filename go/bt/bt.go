package bt

import (
	"context"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// TableConfig maps a table name to a list of column families, describing which
// tables and column InitBigtable should create.
type TableConfig map[string][]string

// InitBigtable takes a list of TableConfigs and creates the given tables and
// column families if they don't exist already.
func InitBigtable(projectID, instanceID string, tableConfigs ...TableConfig) error {
	ctx := context.TODO()

	// Set up admin client, tables, and column families.
	adminClient, err := bigtable.NewAdminClient(ctx, projectID, instanceID)
	if err != nil {
		return sklog.FmtErrorf("Unable to create admin client: %s", err)
	}

	tablesSlice, err := adminClient.Tables(ctx)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving tables: %s", err)
	}
	tables := util.NewStringSet(tablesSlice)

	for _, tConfig := range tableConfigs {
		for tableName, colFamilies := range tConfig {
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
	}
	return nil
}
