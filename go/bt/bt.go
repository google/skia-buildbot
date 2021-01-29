package bt

import (
	"context"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// TestingAppProfile is the profile that can be used when running an instance locally
	// or for integration tests (unit tests should primarily use the BigTable Emulator).
	TestingAppProfile = "local-testing"
)

// InitBigtable takes a list of TableConfigs and creates the given tables and
// column families if they don't exist already.
func InitBigtable(projectID, instanceID, tableID string, colFamilies []string) error {
	ctx := context.TODO()

	// Set up admin client, tables, and column families.
	adminClient, err := bigtable.NewAdminClient(ctx, projectID, instanceID)
	if err != nil {
		return skerr.Fmt("Unable to create admin client: %s", err)
	}

	// Create the table. Ignore error if it already existed.
	err, code := ErrToCode(adminClient.CreateTable(ctx, tableID))
	if err != nil && code != codes.AlreadyExists {
		return skerr.Fmt("Error creating table %s: %s", tableID, err)
	} else {
		sklog.Infof("Created table: %s", tableID)
	}

	// Create the column families. Ignore errors if they already existed.
	for _, colFamName := range colFamilies {
		err, code = ErrToCode(adminClient.CreateColumnFamily(ctx, tableID, colFamName))
		if err != nil && code != codes.AlreadyExists {
			return skerr.Fmt("Error creating column family %s in table %s: %s", colFamName, tableID, err)
		}
	}

	return nil
}

// DeleteTables deletes the tables given in the TableConfig.
func DeleteTables(projectID, instanceID string, tableNames ...string) (err error) {
	ctx := context.TODO()

	// Set up admin client, tables, and column families.
	adminClient, err := bigtable.NewAdminClient(ctx, projectID, instanceID)
	if err != nil {
		return skerr.Fmt("Unable to create admin client: %s", err)
	}
	defer func() {
		if err != nil {
			util.Close(adminClient)
		} else {
			err = adminClient.Close()
		}
	}()

	// Delete all tables if they exist.
	for _, tableName := range tableNames {
		// Ignore NotFound errors.
		err, code := ErrToCode(adminClient.DeleteTable(ctx, tableName))
		if err != nil && code != codes.NotFound {
			return err
		}
	}
	return nil
}

// ErrToCode returns the error that is passed and a gRPC code extracted from the error.
// If the error did not originate in gRPC the returned code is codes.Unknown.
// See https://godoc.org/google.golang.org/grpc/codes for a list of codes.
func ErrToCode(err error) (error, codes.Code) {
	st, _ := status.FromError(err)
	return err, st.Code()
}

// EnsureNotEmulator will panic if it detects the BigTable Emulator is configured.
func EnsureNotEmulator() {
	if emulators.GetEmulatorHostEnvVar(emulators.BigTable) != "" {
		panic("BigTable Emulator detected. Be sure to unset the following environment variable: " + emulators.GetEmulatorHostEnvVarName(emulators.BigTable))
	}
}
