package sqlalertstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (alerts.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "alertstore")
	store, err := New(db)
	require.NoError(t, err)

	return store, db
}

// Tests a hypothetical pipeline of Store.
func TestStore_SaveListDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	// TODO(jcgregorio) Break up into finer grained tests.
	cfg := alerts.NewConfig()
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	err := store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
	})
	assert.NoError(t, err)
	require.NotEqual(t, alerts.BadAlertIDAsAsString, cfg.IDAsString)

	// Confirm it appears in the list.
	cfgs, err := store.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, cfgs, 1)

	// Delete it.
	err = store.Delete(ctx, int(cfgs[0].IDAsStringToInt()))
	assert.NoError(t, err)

	// Confirm it is still there if we list deleted configs.
	cfgs, err = store.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 1)
	require.NotEqual(t, alerts.BadAlertIDAsAsString, cfgs[0].IDAsString)

	// Confirm it is not there if we don't list deleted configs.
	cfgs, err = store.List(ctx, false)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 0)

	// Store a second config.
	cfg = alerts.NewConfig()
	cfg.Query = "source_type=skp"
	cfg.DisplayName = "foo"
	err = store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
	})
	assert.NoError(t, err)

	// Confirm they are both listed when including deleted configs, and they are
	// ordered by DisplayName.
	cfgs, err = store.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Equal(t, "bar", cfgs[0].DisplayName)
	assert.Equal(t, "foo", cfgs[1].DisplayName)
}

// Tests we can list two active Alerts.
func TestStoreList_ListActiveAlerts(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg1 := alerts.NewConfig()
	cfg1.SetIDFromInt64(1)
	cfg1.Query = "source_type=svg"
	cfg1.DisplayName = "bar"
	insertAlertToDb(t, ctx, db, cfg1)

	cfg2 := alerts.NewConfig()
	cfg2.SetIDFromInt64(2)
	cfg2.Query = "source_type=skp"
	cfg2.DisplayName = "foo"
	insertAlertToDb(t, ctx, db, cfg2)

	cfgs, err := store.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Equal(t, "bar", cfgs[0].DisplayName)
	assert.Equal(t, "foo", cfgs[1].DisplayName)
	assert.Equal(t, "source_type=svg", cfgs[0].Query)
	assert.Equal(t, "source_type=skp", cfgs[1].Query)
	assert.Equal(t, "1", cfgs[0].IDAsString)
	assert.Equal(t, "2", cfgs[1].IDAsString)

	// Confirm all alerts are active.
	cfgs2, err := store.List(ctx, false)
	assert.Equal(t, cfgs, cfgs2)
}

// Tests we can list one deleted Alert.
func TestStoreList_ListDeletedAlert(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg1 := alerts.NewConfig()
	cfg1.SetIDFromInt64(1)
	cfg1.Query = "source_type=svg"
	cfg1.DisplayName = "bar"
	insertAlertToDb(t, ctx, db, cfg1)

	cfg2 := alerts.NewConfig()
	cfg2.SetIDFromInt64(2)
	cfg2.Query = "source_type=skp"
	cfg2.DisplayName = "foo"
	cfg2.StateAsString = alerts.DELETED
	insertAlertToDb(t, ctx, db, cfg2)

	cfgs, err := store.List(ctx, true)
	assert.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Equal(t, "bar", cfgs[0].DisplayName)
	assert.Equal(t, "foo", cfgs[1].DisplayName)
	assert.Equal(t, "source_type=svg", cfgs[0].Query)
	assert.Equal(t, "source_type=skp", cfgs[1].Query)
	assert.Equal(t, "1", cfgs[0].IDAsString)
	assert.Equal(t, "2", cfgs[1].IDAsString)

	// Confirm one alert is active
	cfgs2, err := store.List(ctx, false)
	assert.NotEqual(t, cfgs, cfgs2)
	assert.Len(t, cfgs2, 1)
	assert.Equal(t, "bar", cfgs2[0].DisplayName)
	assert.Equal(t, "source_type=svg", cfgs2[0].Query)
	assert.Equal(t, "1", cfgs2[0].IDAsString)
}

// Tests we can mark one Alert as deleted using Delete
func TestStoreDelete_DeleteOneAlert(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg := alerts.NewConfig()
	cfg.SetIDFromInt64(2)
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	insertAlertToDb(t, ctx, db, cfg)

	_, configState1 := getAlertFromDb(t, ctx, db, 2)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState1)

	err := store.Delete(ctx, 2)
	require.NoError(t, err)

	cfg2, configState2 := getAlertFromDb(t, ctx, db, 2)
	assert.Equal(t, "bar", cfg2.DisplayName)
	assert.Equal(t, "source_type=svg", cfg2.Query)
	assert.Equal(t, "2", cfg2.IDAsString)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.DELETED), configState2)
}

// Tests that inserting an ID with -1 ID, generates a new ID for the Alert.
func TestStoreSave_SaveWithBadID(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg := alerts.NewConfig()
	cfg.SetIDFromInt64(alerts.BadAlertID)
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	err := store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
	})
	require.NoError(t, err)

	_, configState1 := getAlertFromDb(t, ctx, db, cfg.IDAsStringToInt())
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState1)
	assert.NotEqual(t, alerts.BadAlertID, cfg.IDAsStringToInt())
}

// Tests inserting an Alert with valid ID.
func TestStoreSave_SaveWithValidID(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg := alerts.NewConfig()
	cfg.SetIDFromInt64(1)
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	err := store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
	})
	require.NoError(t, err)
	_, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)
}

// Tests inserting an Alert with an associated Subscription
func TestStoreSave_SaveWithSubscription(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	cfg := alerts.NewConfig()
	cfg.SetIDFromInt64(1)
	cfg.Query = "source_type=svg"
	cfg.DisplayName = "bar"
	cfg.SubscriptionName = "Test Subscription"
	cfg.SubscriptionRevision = "abcd"
	err := store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
	})
	require.NoError(t, err)

	alert, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, "Test Subscription", alert.SubscriptionName)
	assert.Equal(t, "abcd", alert.SubscriptionRevision)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)
}

func insertAlertToDb(t *testing.T, ctx context.Context, db pool.Pool, cfg *alerts.Alert) {
	b, err := json.Marshal(cfg)
	require.NoError(t, err)

	const query = `UPSERT INTO Alerts
        (id, alert, config_state, last_modified)
        VALUES ($1,$2,$3,$4)`
	if _, err := db.Exec(ctx, query, cfg.IDAsStringToInt(), string(b), cfg.StateToInt(), time.Now().Unix()); err != nil {
		require.NoError(t, err)
	}
}

func getAlertFromDb(t *testing.T, ctx context.Context, db pool.Pool, id int64) (*alerts.Alert, int) {
	alert := &alerts.Alert{}
	var serializedAlert string

	var configState int
	err := db.QueryRow(ctx, "SELECT alert, config_state FROM Alerts WHERE id = $1", id).Scan(
		&serializedAlert,
		&configState,
	)
	require.NoError(t, err)

	err = json.Unmarshal([]byte(serializedAlert), alert)
	require.NoError(t, err)

	return alert, configState
}
