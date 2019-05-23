package btts_testutils

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"cloud.google.com/go/bigtable"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	"golang.org/x/oauth2"
)

func TableName() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("test-%d", rand.Uint64)
}

func CreateTestTable(t sktest.TestingT, tablename string) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.CreateTableFromConf(ctx, &bigtable.TableConf{
		TableID: tablename,
		Families: map[string]bigtable.GCPolicy{
			"V": bigtable.MaxVersionsPolicy(1),
			"S": bigtable.MaxVersionsPolicy(1),
			"D": bigtable.MaxVersionsPolicy(1),
			"H": bigtable.MaxVersionsPolicy(1),
			"I": bigtable.MaxVersionsPolicy(1),
		},
	})
	assert.NoError(t, err)
}

func CleanUpTestTable(t sktest.TestingT, tablename string) {
	ctx := context.Background()
	client, _ := bigtable.NewAdminClient(ctx, "test", "test")
	err := client.DeleteTable(ctx, tablename)
	assert.NoError(t, err)
}

type MockTS struct{}

func (t *MockTS) Token() (*oauth2.Token, error) {
	return nil, nil
}
