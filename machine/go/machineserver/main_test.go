package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
)

func setupForTest(t *testing.T) (context.Context, config.InstanceConfig) {
	require.NotEmpty(t, os.Getenv("FIRESTORE_EMULATOR_HOST"), "This test requires the firestore emulator.")
	cfg := config.InstanceConfig{
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%s", uuid.New()),
		},
	}
	ctx := context.Background()
	return ctx, cfg
}

func TestMachineToggleModeHandler_Success(t *testing.T) {
	unittest.LargeTest(t)
	*baseapp.Local = true

	ctx, cfg := setupForTest(t)
	store, err := store.New(ctx, true, cfg)
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("GET", "/_/machine/toggle_mode/someid", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.Equal(t, machine.ModeMaintenance, machines[0].Mode)
	assert.Contains(t, machines[0].Annotation.Message, "Changed mode to")
	assert.Equal(t, machines[0].Annotation.User, "barney@example.org")
}

func TestMachineToggleModeHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("GET", "/_/machine/toggle_mode/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}
