package sqltest

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/sklog"
	ragSchema "go.skia.org/infra/rag/go/schema_generate/spanner"
)

const (
	ProjectID  = "test-project"
	InstanceID = "test-instance"
)

// NewSpannerDBForTests returns a connection to a local spanner emulator database to
// be used for executing unit tests.
func NewSpannerDBForTests(t *testing.T, databaseNamePrefix string) (*spanner.Client, error) {
	// Ensure that spanner emulator is running first.
	gcp_emulator.RequireSpanner(t)

	host := emulators.GetEmulatorHostEnvVar(emulators.Spanner)
	sklog.Infof("Spanner host set to: %s", host)
	databaseName := fmt.Sprintf("%s_%d", databaseNamePrefix, rand.Int())

	if len(databaseName) > 30 {
		databaseName = databaseName[:30]
	}

	ctx := context.Background()
	instancePath := fmt.Sprintf("projects/%s/instances/%s", ProjectID, InstanceID)
	databasePath := fmt.Sprintf("projects/%s/instances/%s/databases/%s", ProjectID, InstanceID, databaseName)

	// --- 1. Create the Instance Admin Client (for CREATE INSTANCE) ---
	instAdminClient, err := instance.NewInstanceAdminClient(ctx, option.WithoutAuthentication())
	if err != nil {
		sklog.Errorf("Error creating Instance Admin client: %v\n", err)
		return nil, err
	}
	defer instAdminClient.Close()

	// --- 2. Ensure Instance Exists ---
	// This solves the "Instance not found" error
	if err := ensureInstanceExists(ctx, instAdminClient, ProjectID, InstanceID); err != nil {
		sklog.Errorf("Failed to ensure instance exists: %v\n", err)
		return nil, err
	}

	// --- 3. Create the Database Admin Client (for DDL) ---
	// It automatically detects SPANNER_EMULATOR_HOST
	dbAdminClient, err := database.NewDatabaseAdminClient(ctx, option.WithoutAuthentication())
	if err != nil {
		sklog.Errorf("Error creating Database Admin client: %v\n", err)
		return nil, err
	}
	defer dbAdminClient.Close()

	// In the emulator, you might need to create the database first if it doesn't exist
	// This step is often skipped if running against a real GCP Spanner database that's pre-created.
	if err := ensureDatabaseExists(ctx, dbAdminClient, instancePath, databaseName); err != nil {
		sklog.Errorf("Failed to ensure database exists: %v\n", err)
		return nil, err
	}

	createStatements := strings.Split(ragSchema.Schema, ";")
	filteredStmts := []string{}

	// Queries that are not supported in spanner emulator.
	// TODO(ashwinpv): Figure out a workaround to unit test this.
	unsupportedTexts := []string{
		"VECTOR INDEX",
	}

	// The splitting can potentially result in empty strings due to formatting.
	// Ensure we have non empty statements for the DDL.
	sklog.Infof("Found %d stmts", len(createStatements))
	for _, stmt := range createStatements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		add := true
		for _, unsupportedText := range unsupportedTexts {
			if strings.Contains(stmt, unsupportedText) {
				add = false
				break
			}
		}
		if add {
			filteredStmts = append(filteredStmts, stmt)
		}
	}
	op, err := dbAdminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   databasePath,
		Statements: filteredStmts,
	})
	if err != nil {
		sklog.Errorf("Error submitting DDL: %v\n", err)
		return nil, err
	}

	// Wait for the operation to complete
	if err := op.Wait(ctx); err != nil {
		sklog.Errorf("DDL operation failed: %v\n", err)
		return nil, err
	}

	// Now that the database is set up with the schema, let's create the spanner client.
	spannerClient, err := spanner.NewClient(ctx, databasePath, option.WithoutAuthentication())
	require.NoError(t, err)

	return spannerClient, nil
}

// Helper function to create the database if it doesn't exist (useful for emulator)
func ensureDatabaseExists(ctx context.Context, dbAdminClient *database.DatabaseAdminClient, instancePath, databaseID string) error {
	dbPath := fmt.Sprintf("%s/databases/%s", instancePath, databaseID)

	// Check if the database already exists
	_, err := dbAdminClient.GetDatabase(ctx, &databasepb.GetDatabaseRequest{Name: dbPath})
	if err == nil {
		sklog.Infof("Database '%s' already exists.\n", databaseID)
		return nil // Already exists
	}

	// Only proceed with creation if the error indicates "not found"
	// The emulator might return a different error, so this check is simplified
	sklog.Infof("Database '%s' not found. Creating it now...\n", databaseID)

	op, err := dbAdminClient.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          instancePath,
		CreateStatement: fmt.Sprintf("CREATE DATABASE `%s`", databaseID),
	})
	if err != nil {
		return fmt.Errorf("error submitting database creation DDL: %w", err)
	}

	// Wait for the database creation operation to complete
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("database creation DDL operation failed: %w", err)
	}

	sklog.Infof("✅ Successfully created Spanner database: %s\n", databaseID)
	return nil
}

func ensureInstanceExists(ctx context.Context, instAdminClient *instance.InstanceAdminClient, projectID, instanceID string) error {
	instancePath := fmt.Sprintf("projects/%s/instances/%s", projectID, instanceID)

	// 1. Check if the instance exists
	_, err := instAdminClient.GetInstance(ctx, &instancepb.GetInstanceRequest{Name: instancePath})
	if err == nil {
		sklog.Infof("Instance '%s' already exists.\n", instanceID)
		return nil
	}

	// If the error is NOT_FOUND, proceed to create it
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("error checking instance existence: %w", err)
	}

	// 2. Create the instance
	sklog.Infof("Instance '%s' not found. Creating it now...\n", instanceID)

	op, err := instAdminClient.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     fmt.Sprintf("projects/%s", projectID),
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			// The emulator requires the config to be 'emulator-config'
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", projectID),
			DisplayName: "Emulator Test Instance",
			NodeCount:   1,
		},
	})
	if err != nil {
		return fmt.Errorf("error submitting instance creation DDL: %w", err)
	}

	// 3. Wait for the operation to complete
	if _, err := op.Wait(ctx); err != nil {
		// This part is for resilience; the instance might have been created concurrently
		if status.Code(err) == codes.AlreadyExists {
			sklog.Infof("Instance creation completed, or was created concurrently.")
			return nil
		}
		return fmt.Errorf("instance creation operation failed: %w", err)
	}

	sklog.Infof("✅ Successfully created Spanner instance: %s\n", instanceID)
	return nil
}
