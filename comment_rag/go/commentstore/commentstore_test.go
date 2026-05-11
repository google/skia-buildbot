package commentstore

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commentSchema "go.skia.org/infra/comment_rag/go/schema_generate/spanner"
	commentSpanner "go.skia.org/infra/comment_rag/go/spanner"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/sklog"
)

const (
	testProjectID  = "test-project"
	testInstanceID = "test-instance"
)

// newSpannerDBForReviewTests creates a local Spanner emulator database pre-configured with the ReviewHistory table.
func newSpannerDBForReviewTests(t *testing.T, dbNamePrefix string) (*spanner.Client, error) {
	gcp_emulator.RequireSpanner(t)

	dbName := fmt.Sprintf("%s_%d", dbNamePrefix, rand.Int())
	if len(dbName) > 30 {
		dbName = dbName[:30]
	}

	ctx := context.Background()
	instancePath := fmt.Sprintf("projects/%s/instances/%s", testProjectID, testInstanceID)
	databasePath := fmt.Sprintf("projects/%s/instances/%s/databases/%s", testProjectID, testInstanceID, dbName)

	instAdminClient, err := instance.NewInstanceAdminClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	defer instAdminClient.Close()

	// Ensure Spanner instance exists
	op, err := instAdminClient.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     fmt.Sprintf("projects/%s", testProjectID),
		InstanceId: testInstanceID,
		Instance: &instancepb.Instance{
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", testProjectID),
			DisplayName: "Emulator Test Instance",
			NodeCount:   1,
		},
	})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		return nil, err
	}
	if err == nil {
		if _, err := op.Wait(ctx); err != nil && status.Code(err) != codes.AlreadyExists {
			return nil, err
		}
	}

	dbAdminClient, err := database.NewDatabaseAdminClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	defer dbAdminClient.Close()

	// Create Spanner database
	opDb, err := dbAdminClient.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          instancePath,
		CreateStatement: fmt.Sprintf("CREATE DATABASE `%s`", dbName),
	})
	if err != nil {
		return nil, err
	}
	if _, err := opDb.Wait(ctx); err != nil {
		return nil, err
	}

	// Apply generated schema DDL
	createStatements := strings.Split(commentSchema.Schema, ";")
	filteredStmts := []string{}
	unsupportedTexts := []string{
		"VECTOR INDEX",
	}

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

	opDdl, err := dbAdminClient.UpdateDatabaseDdl(ctx, &databasepb.UpdateDatabaseDdlRequest{
		Database:   databasePath,
		Statements: filteredStmts,
	})
	if err != nil {
		return nil, err
	}
	if err := opDdl.Wait(ctx); err != nil {
		return nil, err
	}

	spannerClient, err := spanner.NewClient(ctx, databasePath, option.WithoutAuthentication())
	require.NoError(t, err)

	sklog.Infof("Successfully set up local ReviewHistory Spanner client: %s", databasePath)
	return spannerClient, nil
}

func TestSpannerCommentStore(t *testing.T) {
	ctx := context.Background()
	commentSpanner.ValidCategories = append(commentSpanner.ValidCategories, "STYLE")

	spannerClient, err := newSpannerDBForReviewTests(t, "comment_store")
	require.NoError(t, err)

	store := NewSpannerCommentStore(spannerClient)

	emb1 := make([]float32, 768)
	emb1[0] = 1.0
	c1 := &CommentRecord{
		ID:            "12345-hash1",
		Project:       "chromium",
		Repo:          "chromium/src",
		Category:      "IPC_SECURITY",
		ChangeID:      12345,
		FilePath:      "content/browser/bad_mojo.cc",
		CommentText:   "This Mojo handle is unvalidated.",
		CodeSnippet:   "CHECK(handle);",
		CLSubject:     "Fix Mojo issue",
		CLDescription: "Description 1",
		Analysis:      "Vulnerability: Mojo validation",
		Embedding:     emb1,
	}

	emb2 := make([]float32, 768)
	emb2[1] = 1.0
	c2 := &CommentRecord{
		ID:            "67890-hash2",
		Project:       "chromium",
		Repo:          "chromium/src",
		Category:      "IPC_SECURITY",
		ChangeID:      67890,
		FilePath:      "content/browser/other.cc",
		CommentText:   "Integer overflow risk.",
		CodeSnippet:   "int x = y + z;",
		CLSubject:     "Fix overflow",
		CLDescription: "Description 2",
		Analysis:      "Vulnerability: Overflow",
		Embedding:     emb2,
	}

	emb3 := make([]float32, 768)
	emb3[2] = 1.0
	c3 := &CommentRecord{
		ID:            "11111-hash3",
		Project:       "skia",
		Repo:          "skia",
		Category:      "STYLE",
		ChangeID:      11111,
		FilePath:      "src/core/SkCanvas.cpp",
		CommentText:   "Use auto here.",
		CodeSnippet:   "auto x = y;",
		CLSubject:     "Style fix",
		CLDescription: "Description 3",
		Analysis:      "Style nit",
		Embedding:     emb3,
	}

	// Test WriteCommentRecord
	err = store.WriteCommentRecord(ctx, c1)
	require.NoError(t, err)

	err = store.WriteCommentRecord(ctx, c2)
	require.NoError(t, err)

	err = store.WriteCommentRecord(ctx, c3)
	require.NoError(t, err)

	// Test SearchComments (Basic Vector Search)
	// Searching with embedding closer to c1
	searchEmb := make([]float32, 768)
	searchEmb[0] = 1.0
	found, err := store.SearchComments(ctx, searchEmb, 10, "", "", nil)
	require.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, "12345-hash1", found[0].ID)
	assert.Equal(t, "11111-hash3", found[1].ID)
	assert.Equal(t, "67890-hash2", found[2].ID)

	// Test SearchComments with Category Filter ("IPC_SECURITY")
	foundIPC, err := store.SearchComments(ctx, searchEmb, 10, "", "", []string{"IPC_SECURITY"})
	require.NoError(t, err)
	assert.Len(t, foundIPC, 2)
	assert.Equal(t, "12345-hash1", foundIPC[0].ID)
	assert.Equal(t, "67890-hash2", foundIPC[1].ID)

	// Test SearchComments with Project Filter ("skia")
	foundSkia, err := store.SearchComments(ctx, searchEmb, 10, "skia", "", nil)
	require.NoError(t, err)
	assert.Len(t, foundSkia, 1)
	assert.Equal(t, "11111-hash3", foundSkia[0].ID)

	// Test SearchComments with Repo Filter ("chromium/src")
	foundRepo, err := store.SearchComments(ctx, searchEmb, 10, "chromium", "chromium/src", nil)
	require.NoError(t, err)
	assert.Len(t, foundRepo, 2)
	assert.Equal(t, "12345-hash1", foundRepo[0].ID)
	assert.Equal(t, "67890-hash2", foundRepo[1].ID)

	// Test SearchComments with Multiple Categories Filter (["IPC_SECURITY", "STYLE"])
	foundMulti, err := store.SearchComments(ctx, searchEmb, 10, "", "", []string{"IPC_SECURITY", "STYLE"})
	require.NoError(t, err)
	assert.Len(t, foundMulti, 3)
	assert.Equal(t, "12345-hash1", foundMulti[0].ID)
	assert.Equal(t, "11111-hash3", foundMulti[1].ID)
	assert.Equal(t, "67890-hash2", foundMulti[2].ID)
}
