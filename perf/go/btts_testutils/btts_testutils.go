package btts_testutils

import (
	"context"
	"testing"

	"cloud.google.com/go/bigtable"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

func CreateTestTable(t *testing.T) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: "test",
		Families: map[string]bigtable.GCPolicy{
			"V": bigtable.MaxVersionsPolicy(1),
			"S": bigtable.MaxVersionsPolicy(1),
			"D": bigtable.MaxVersionsPolicy(1),
			"H": bigtable.MaxVersionsPolicy(1),
		},
	})
	assert.NoError(t, err)
}

func CleanUpTestTable(t *testing.T) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.DeleteTable(ctx, "test")
	assert.NoError(t, err)
}

type MockTS struct{}

func (t *MockTS) Token() (*oauth2.Token, error) {
	return nil, nil
}
