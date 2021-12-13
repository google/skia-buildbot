package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/machineserver/config"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"go.skia.org/infra/machine/go/switchboard"
	switchboardMocks "go.skia.org/infra/machine/go/switchboard/mocks"
)

func setupForTest(t *testing.T) (context.Context, config.InstanceConfig) {
	unittest.RequiresFirestoreEmulator(t)
	cfg := config.InstanceConfig{
		Store: config.Store{
			Project:  "test-project",
			Instance: fmt.Sprintf("test-%s", uuid.New()),
		},
	}
	ctx := context.Background()

	// Use fake authentication.
	*baseapp.Local = true
	return ctx, cfg
}

// responseTo tries an HTTP request of the given method and path against a server and returns the
// response.
func responseTo(s *server, method, path string) *httptest.ResponseRecorder {
	router := mux.NewRouter()
	s.AddHandlers(router)
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

// assertJSONResponseWas asserts that the HTTP response w was a success (code 200) and has a body
// matching the expected string. Since JSON encoding adds a trailing newline, we expect it as well,
// though the caller should not include it in the expected string.
func assertJSONResponseWas(t *testing.T, expected string, w *httptest.ResponseRecorder) {
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, []byte(expected+"\n"), w.Body.Bytes())
}

func TestMachineToggleModeHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/toggle_mode/someid", nil)
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

	r := httptest.NewRequest("POST", "/_/machine/toggle_mode/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestMachineTogglePowerCycleHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(in machine.Description) machine.Description {
		ret := in.Copy()
		return ret
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/toggle_powercycle/someid", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.True(t, machines[0].PowerCycle)
	assert.Contains(t, machines[0].Annotation.Message, "Requested powercycle for")
	assert.Equal(t, machines[0].Annotation.User, "barney@example.org")

	// Now confirm we toggle back.
	r = httptest.NewRequest("POST", "/_/machine/toggle_powercycle/someid", nil)
	w = httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.False(t, machines[0].PowerCycle)
	assert.Contains(t, machines[0].Annotation.Message, "Requested powercycle for")
	assert.Equal(t, machines[0].Annotation.User, "barney@example.org")

}

func TestMachineTogglePowerCycleHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/toggle_powercycle/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestMachineRemoveDeviceHandler_AndroidDevice_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(in machine.Description) machine.Description {
		ret := in.Copy()
		ret.Dimensions = machine.SwarmingDimensions{
			"android_devices":  {"1"},
			"device_os":        {"Q", " QP1A.190711.020", "QP1A.190711.020_G980FXXU1ATB3"},
			"device_os_flavor": {"samsung"},
		}
		return ret
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/remove_device/someid", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	// Confirm the dimensions were cleared.
	assert.Empty(t, machines[0].Dimensions)
	assert.Contains(t, machines[0].Annotation.Message, "Requested device removal")
	assert.Equal(t, machines[0].Annotation.User, "barney@example.org")
}

func TestMachineRemoveDeviceHandler_ChromeOSDevice_Success(t *testing.T) {
	unittest.LargeTest(t)

	c, cfg := setupForTest(t)
	ctx := now.TimeTravelingContext(fakeTime).WithContext(c)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "skia-rpi-002", func(in machine.Description) machine.Description {
		ret := in.Copy()
		ret.SSHUserIP = "root@chrome-os"
		ret.SuppliedDimensions = machine.SwarmingDimensions{
			"gpu": {"IntelUHDGraphics605"},
			"os":  {"ChromeOS"},
			"cpu": {"x86", "x86_64"},
		}
		ret.Dimensions = machine.SwarmingDimensions{
			"gpu":              {"IntelUHDGraphics605"},
			"os":               {"ChromeOS"},
			"cpu":              {"x86", "x86_64"},
			"chromeos_channel": {"stable-channel"}, // supplied via device interrogation
		}
		return ret
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/remove_device/skia-rpi-002", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.Equal(t, machine.Description{
		Mode:           machine.ModeAvailable,
		AttachedDevice: machine.AttachedDeviceNone,
		Annotation: machine.Annotation{
			Message:   "Requested device removal of skia-rpi-002",
			User:      "barney@example.org",
			Timestamp: fakeTime,
		},
		Dimensions:  machine.SwarmingDimensions{},
		LastUpdated: fakeTime,
	}, machines[0])
}

func TestMachineRemoveDeviceHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/remove_device/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestMachineDeleteMachineHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(in machine.Description) machine.Description {
		ret := in.Copy()
		return ret
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/delete_machine/someid", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	// Confirm the request was successful.
	require.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 0)
}

func TestMachineDeleteMachineHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/delete_machine/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestMachineSetNoteHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(in machine.Description) machine.Description {
		ret := in.Copy()
		return ret
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	note := rpc.SetNoteRequest{
		Message: "Hello World",
	}
	b, err := json.Marshal(note)
	assert.NoError(t, err)
	r := httptest.NewRequest("POST", "/_/machine/set_note/someid", bytes.NewReader(b))
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.Equal(t, machines[0].Note.Message, "Hello World")
}

func TestMachineSetNoteHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/set_note/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestMachineSetNoteHandler_FailOnInvalidJSON(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/set_note/someid", bytes.NewReader([]byte("This isn't valid JSON.")))
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPodsHandler_Success(t *testing.T) {
	unittest.SmallTest(t)
	expected := switchboard.Pod{
		Name:        "switch-pod-3",
		LastUpdated: time.Date(2001, 2, 3, 4, 5, 6, 789012345, time.UTC),
	}

	// ListPods is already well-tested in switchboard/impl, so we can mock out the whole
	// switchboard.
	sw := switchboardMocks.Switchboard{}
	sw.On("ListPods", testutils.AnyContext).Return([]switchboard.Pod{expected}, nil)
	s := &server{
		switchboard: &sw,
	}

	w := responseTo(s, "GET", "/_/pods")
	assertJSONResponseWas(t, `[{"Name":"switch-pod-3","LastUpdated":"2001-02-03T04:05:06.789012345Z"}]`, w)
}

func TestMeetingPointsHandler_Success(t *testing.T) {
	unittest.SmallTest(t)
	expected := switchboard.MeetingPoint{
		PodName:     "somePod",
		Port:        33,
		Username:    "someUser",
		MachineID:   "someMachine",
		LastUpdated: time.Date(2001, 2, 3, 4, 5, 6, 789012345, time.UTC),
	}

	// ListMeetingPoints is already well-tested in switchboard/impl, so we can mock out the whole
	// switchboard.
	sw := switchboardMocks.Switchboard{}
	sw.On("ListMeetingPoints", testutils.AnyContext).Return([]switchboard.MeetingPoint{expected}, nil)
	s := &server{
		switchboard: &sw,
	}

	w := responseTo(s, "GET", "/_/meeting_points")
	assertJSONResponseWas(t, `[{"PodName":"somePod","Port":33,"Username":"someUser","MachineID":"someMachine","LastUpdated":"2001-02-03T04:05:06.789012345Z"}]`, w)
}

func TestMachineSupplyChromeOSInfoHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	existingTime := time.Date(2021, time.September, 3, 3, 3, 3, 0, time.UTC)
	updatedTime := time.Date(2021, time.September, 3, 3, 7, 0, 0, time.UTC)

	c, cfg := setupForTest(t)
	ctx := now.TimeTravelingContext(existingTime).WithContext(c)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(_ machine.Description) machine.Description {
		return machine.Description{
			Mode:           machine.ModeAvailable,
			AttachedDevice: machine.AttachedDeviceSSH,
			Dimensions: machine.SwarmingDimensions{
				"cpu": {"x86"},
				"os":  {"Linux"},
			},
			LastUpdated: existingTime,
		}
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	ctx.SetTime(updatedTime)

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	body := rpc.SupplyChromeOSRequest{
		SSHUserIP: "root@my-chromebook-001",
		SuppliedDimensions: map[string][]string{
			"gpu": {"some-gpu"},
			"cpu": {"some-cpu", "some-other-cpu"},
		},
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest("POST", "/_/machine/supply_chromeos/someid", bytes.NewReader(b))
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.Equal(t, machine.Description{
		Mode:           machine.ModeAvailable,
		AttachedDevice: machine.AttachedDeviceSSH,
		Dimensions: machine.SwarmingDimensions{ // These dimensions remain unchanged
			"cpu": {"x86"},
			"os":  {"Linux"},
		},
		Temperature: map[string]float64{},
		LastUpdated: updatedTime,

		// These should be set from the POST value
		SSHUserIP: "root@my-chromebook-001",
		SuppliedDimensions: map[string][]string{
			"gpu": {"some-gpu"},
			"cpu": {"some-cpu", "some-other-cpu"},
		},
	}, machines[0])
}

func TestMachineSupplyChromeOSInfoHandler_FailOnMissingField(t *testing.T) {
	unittest.LargeTest(t)

	c, cfg := setupForTest(t)
	ctx := now.TimeTravelingContext(fakeTime).WithContext(c)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	err = store.Update(ctx, "someid", func(_ machine.Description) machine.Description {
		return machine.Description{
			Mode:           machine.ModeAvailable,
			AttachedDevice: machine.AttachedDeviceSSH,
			Dimensions: machine.SwarmingDimensions{
				"cpu": {"x86"},
				"os":  {"Linux"},
			},
			LastUpdated: fakeTime,
		}
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	test := func(name string, request rpc.SupplyChromeOSRequest) {
		t.Run(name, func(t *testing.T) {
			// Put a mux.Router in place so the request path gets parsed.
			router := mux.NewRouter()
			s.AddHandlers(router)

			b, err := json.Marshal(request)
			require.NoError(t, err)
			r := httptest.NewRequest("POST", "/_/machine/supply_chromeos/someid", bytes.NewReader(b))
			w := httptest.NewRecorder()

			// Make the request.
			router.ServeHTTP(w, r)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			machines, err := store.List(ctx)
			require.NoError(t, err)
			require.Len(t, machines, 1)
			assert.Equal(t, machine.Description{
				// This all remains unchanged

				Mode:           machine.ModeAvailable,
				AttachedDevice: machine.AttachedDeviceSSH,
				Dimensions: machine.SwarmingDimensions{
					"cpu": {"x86"},
					"os":  {"Linux"},
				},
				LastUpdated: fakeTime,
			}, machines[0])
		})
	}

	test("missing dimensions", rpc.SupplyChromeOSRequest{
		SSHUserIP: "root@my-chromebook-001",
	})
	test("missing ssh ip", rpc.SupplyChromeOSRequest{
		SuppliedDimensions: map[string][]string{
			"something": {"something", "else"},
		},
	})
}

func TestMachineSupplyChromeOSInfoHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("GET", "/_/machine/supply_chromeos/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

var fakeTime = time.Date(2021, time.September, 1, 2, 3, 4, 0, time.UTC)

func TestSetAttachedDeviceHandler_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cfg := setupForTest(t)
	store, err := store.NewFirestoreImpl(ctx, true, cfg)
	require.NoError(t, err)

	// Create a machine at "someid".
	err = store.Update(ctx, "someid", func(in machine.Description) machine.Description {
		return machine.Description{
			Mode: machine.ModeAvailable,
			// Start with an AttachedDevice that isn't iOS so we can see that it changes.
			AttachedDevice: machine.AttachedDeviceSSH,
			Dimensions: machine.SwarmingDimensions{
				"cpu": {"x86"},
				"os":  {"Linux"},
			},
			LastUpdated: fakeTime,
		}
	})
	require.NoError(t, err)

	// Create our server.
	s := &server{
		store: store,
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	adReq := rpc.SetAttachedDevice{
		AttachedDevice: machine.AttachedDeviceiOS,
	}
	b, err := json.Marshal(adReq)
	assert.NoError(t, err)
	r := httptest.NewRequest("POST", "/_/machine/set_attached_device/someid", bytes.NewReader(b))
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 200, w.Code)
	machines, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, machines, 1)
	assert.Equal(t, machines[0].AttachedDevice, machine.AttachedDeviceiOS)
}

func TestSetAttachedDeviceHandler_FailOnMissingID(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/set_attached_device/", nil)
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, 404, w.Code)
}

func TestSetAttachedDeviceHandler_FailOnInvalidJSON(t *testing.T) {
	unittest.LargeTest(t)

	// Create our server.
	s := &server{}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)

	r := httptest.NewRequest("POST", "/_/machine/set_attached_device/someid", bytes.NewReader([]byte("This isn't valid JSON.")))
	w := httptest.NewRecorder()

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
