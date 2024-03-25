package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/config"
	culprit_store "go.skia.org/infra/perf/go/culprit/sqlculpritstore"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupTestApp(t *testing.T) *Backend {
	db := sqltest.NewCockroachDBForTests(t, "backend")
	culpritStore, _ := culprit_store.New(db)
	configFile := testutils.TestDataFilename(t, "demo.json")
	sklog.Infof("Config file: %s", configFile)
	flags := &config.BackendFlags{
		Port:           ":0",
		PromPort:       ":0",
		ConfigFilename: configFile,
	}
	b, err := New(flags, culpritStore)
	require.NoError(t, err)
	ch := make(chan interface{})
	go func() {
		err := b.ServeGRPC()
		assert.NoError(t, err)
		ch <- nil
	}()

	t.Cleanup(func() {
		b.Cleanup()
		<-ch
	})

	return b
}

func TestAppSetup(t *testing.T) {
	b := setupTestApp(t)

	_, err := grpc.Dial(b.grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
}
