package sqlalertstore

import (
	"context"
	"database/sql"
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
		Cfg:    cfg,
		SubKey: nil,
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
		Cfg:    cfg,
		SubKey: nil,
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
	insertAlertToDb(t, ctx, db, cfg1, nil)

	cfg2 := alerts.NewConfig()
	cfg2.SetIDFromInt64(2)
	cfg2.Query = "source_type=skp"
	cfg2.DisplayName = "foo"
	insertAlertToDb(t, ctx, db, cfg2, nil)

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
	insertAlertToDb(t, ctx, db, cfg1, nil)

	cfg2 := alerts.NewConfig()
	cfg2.SetIDFromInt64(2)
	cfg2.Query = "source_type=skp"
	cfg2.DisplayName = "foo"
	cfg2.StateAsString = alerts.DELETED
	insertAlertToDb(t, ctx, db, cfg2, nil)

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
	insertAlertToDb(t, ctx, db, cfg, nil)

	_, subKey1, configState1 := getAlertFromDb(t, ctx, db, 2)
	assert.Nil(t, subKey1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState1)

	err := store.Delete(ctx, 2)
	require.NoError(t, err)

	cfg2, subKey2, configState2 := getAlertFromDb(t, ctx, db, 2)
	assert.Nil(t, subKey2)
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
		Cfg:    cfg,
		SubKey: nil,
	})
	require.NoError(t, err)

	_, subKey1, configState1 := getAlertFromDb(t, ctx, db, cfg.IDAsStringToInt())
	assert.Nil(t, subKey1)
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
		Cfg:    cfg,
		SubKey: nil,
	})
	require.NoError(t, err)
	_, subKey, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Nil(t, subKey)
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
	err := store.Save(ctx, &alerts.SaveRequest{
		Cfg: cfg,
		SubKey: &alerts.SubKey{
			SubName:     "Test Subscription",
			SubRevision: "abcd",
		},
	})
	require.NoError(t, err)

	_, subKey, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, "Test Subscription", subKey.SubName)
	assert.Equal(t, "abcd", subKey.SubRevision)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)
}

func TestStoreReplaceAll_EmptyAlerts(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)
	oldAlert := &alerts.SaveRequest{
		Cfg: &alerts.Alert{
			IDAsString:  "1",
			DisplayName: "Alert A",
		},
		SubKey: &alerts.SubKey{
			SubName:     "a",
			SubRevision: "abcd",
		},
	}

	// First populate DB with 1 alert and confirm that it's set to active.
	insertAlertToDb(t, ctx, db, oldAlert.Cfg, oldAlert.SubKey)
	_, _, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)

	newAlerts := []*alerts.SaveRequest{}

	err := store.ReplaceAll(ctx, newAlerts)
	require.NoError(t, err)

	// Now check that old alert is inactive and new alerts are active
	_, _, configState = getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.DELETED), configState)

	activeAlerts := listAllAlertsInDb(t, ctx, db, alerts.ConfigStateToInt(alerts.ACTIVE))
	assert.Len(t, activeAlerts, 0)
}

func TestStoreReplaceAll_ValidAlerts(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)
	oldAlert := &alerts.SaveRequest{
		Cfg: &alerts.Alert{
			IDAsString:  "1",
			DisplayName: "Alert A",
		},
		SubKey: &alerts.SubKey{
			SubName:     "a",
			SubRevision: "abcd",
		},
	}

	// First populate DB with 1 alert and confirm that it's set to active.
	insertAlertToDb(t, ctx, db, oldAlert.Cfg, oldAlert.SubKey)
	_, _, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)

	newAlerts := []*alerts.SaveRequest{
		{
			Cfg: &alerts.Alert{
				IDAsString:  "2",
				DisplayName: "Alert B",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcde",
			},
		},
		{
			Cfg: &alerts.Alert{
				IDAsString:  "3",
				DisplayName: "Alert C",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcde",
			},
		},
	}

	err := store.ReplaceAll(ctx, newAlerts)
	require.NoError(t, err)

	// Now check that old alert is inactive and new alerts are active
	_, _, configState = getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.DELETED), configState)

	activeAlerts := listAllAlertsInDb(t, ctx, db, alerts.ConfigStateToInt(alerts.ACTIVE))
	assert.Len(t, activeAlerts, 2)
	for _, alert := range activeAlerts {
		assert.True(t, alert.IDAsString == "2" || alert.IDAsString == "3")
	}
}

func TestStoreReplaceAll_DuplicateAlerts(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)
	oldAlert := &alerts.SaveRequest{
		Cfg: &alerts.Alert{
			IDAsString:  "1",
			DisplayName: "Alert A",
		},
		SubKey: &alerts.SubKey{
			SubName:     "a",
			SubRevision: "abcd",
		},
	}

	// First populate DB with 1 alert and confirm that it's set to active.
	insertAlertToDb(t, ctx, db, oldAlert.Cfg, oldAlert.SubKey)
	_, _, configState := getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.ACTIVE), configState)

	newAlerts := []*alerts.SaveRequest{
		{
			Cfg: &alerts.Alert{
				IDAsString:  "2",
				DisplayName: "Alert B",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcde",
			},
		},
		{
			Cfg: &alerts.Alert{
				IDAsString:  "3",
				DisplayName: "Alert C",
			},
			SubKey: &alerts.SubKey{
				SubName:     "b",
				SubRevision: "abcde",
			},
		},
	}

	err := store.ReplaceAll(ctx, newAlerts)
	require.NoError(t, err)

	// Now check that old alert is inactive and new alerts are active
	_, _, configState = getAlertFromDb(t, ctx, db, 1)
	assert.Equal(t, alerts.ConfigStateToInt(alerts.DELETED), configState)

	activeAlerts := listAllAlertsInDb(t, ctx, db, alerts.ConfigStateToInt(alerts.ACTIVE))
	assert.Len(t, activeAlerts, 2)
	for _, alert := range activeAlerts {
		assert.True(t, alert.IDAsString == "2" || alert.IDAsString == "3")
	}
}

// insertAlertToDb inserts the given Alert into the database.
func insertAlertToDb(t *testing.T, ctx context.Context, db pool.Pool, cfg *alerts.Alert, subKey *alerts.SubKey) {
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	nameOrNull := sql.NullString{Valid: false}
	revisionOrNull := sql.NullString{Valid: false}

	if subKey != nil {
		nameOrNull.String = subKey.SubName
		nameOrNull.Valid = true
		revisionOrNull.String = subKey.SubRevision
		revisionOrNull.Valid = true
	}
	const query = `UPSERT INTO Alerts
        (id, alert, config_state, last_modified, sub_name, sub_revision)
        VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := db.Exec(ctx, query, cfg.IDAsStringToInt(), string(b), cfg.StateToInt(), time.Now().Unix(), nameOrNull, revisionOrNull); err != nil {
		require.NoError(t, err)
	}
}

func getAlertFromDb(t *testing.T, ctx context.Context, db pool.Pool, id int64) (*alerts.Alert, *alerts.SubKey, int) {
	alert := &alerts.Alert{}
	var serializedAlert string

	var nameOrNull sql.NullString
	var revisionOrNull sql.NullString
	var configState int
	err := db.QueryRow(ctx, "SELECT alert, sub_name, sub_revision, config_state FROM Alerts WHERE id = $1", id).Scan(
		&serializedAlert,
		&nameOrNull,
		&revisionOrNull,
		&configState,
	)
	require.NoError(t, err)

	err = json.Unmarshal([]byte(serializedAlert), alert)
	require.NoError(t, err)

	if !nameOrNull.Valid || !revisionOrNull.Valid {
		return alert, nil, configState
	}

	return alert, &alerts.SubKey{
		SubName:     nameOrNull.String,
		SubRevision: revisionOrNull.String,
	}, configState
}

// List all Alerts that match the given configState
func listAllAlertsInDb(t *testing.T, ctx context.Context, db pool.Pool, configState int) []*alerts.Alert {
	rows, err := db.Query(ctx, "SELECT alert FROM Alerts WHERE config_state = $1", configState)
	require.NoError(t, err)
	ret := []*alerts.Alert{}
	for rows.Next() {
		var serializedAlert string
		if err := rows.Scan(&serializedAlert); err != nil {
			require.NoError(t, err)
		}
		a := &alerts.Alert{}
		if err := json.Unmarshal([]byte(serializedAlert), a); err != nil {
			require.NoError(t, err)
		}
		ret = append(ret, a)
	}
	return ret
}
