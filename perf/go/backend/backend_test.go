package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	alert_store "go.skia.org/infra/perf/go/alerts/sqlalertstore"
	ag_store "go.skia.org/infra/perf/go/anomalygroup/sqlanomalygroupstore"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit/notify"
	culprit_store "go.skia.org/infra/perf/go/culprit/sqlculpritstore"
	"go.skia.org/infra/perf/go/regression/sqlregression2store"
	"go.skia.org/infra/perf/go/sql/sqltest"
	subscription_store "go.skia.org/infra/perf/go/subscription/sqlsubscriptionstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupTestApp(t *testing.T) *Backend {
	db := sqltest.NewSpannerDBForTests(t, "backend")
	alertStore, _ := alert_store.New(db)
	configProvider, _ := alerts.NewConfigProvider(context.Background(), alertStore, 600)
	anomalygroupStore, _ := ag_store.New(db)
	culpritStore, _ := culprit_store.New(db)
	subscriptionStore, _ := subscription_store.New(db)
	regressionStore, _ := sqlregression2store.New(db, configProvider)
	configFile := testutils.TestDataFilename(t, "demo.json")
	sklog.Infof("Config file: %s", configFile)
	flags := &config.BackendFlags{
		Port:           ":0",
		PromPort:       ":0",
		ConfigFilename: configFile,
	}
	b, err := New(flags, anomalygroupStore, culpritStore, subscriptionStore, regressionStore, &notify.DefaultCulpritNotifier{})
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
