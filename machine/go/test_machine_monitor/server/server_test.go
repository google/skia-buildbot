package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	botmachine "go.skia.org/infra/machine/go/test_machine_monitor/machine"
)

func TestGetSettings_Success(t *testing.T) {

	r := httptest.NewRequest("GET", "/get_settings", nil)
	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getSettings(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	b, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"caches":{"isolated":{"size":8589934592}}}`, strings.TrimSpace(string(b)))
}

func TestGetState_Success(t *testing.T) {

	const someRackName = "some-rack-name"

	err := os.Setenv("MY_RACK_NAME", someRackName)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/get_state", strings.NewReader("{\"foo\":\"bar\"}"))

	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getState(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	var dict map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&dict)
	require.NoError(t, err)
	assert.Equal(t, someRackName, dict["sk_rack"])
	assert.Equal(t, "bar", dict["foo"])
	_, ok := dict["maintenance"]
	assert.False(t, ok)
}

func TestGetState_ErrOnInvalidJSON(t *testing.T) {

	r := httptest.NewRequest("POST", "/get_state", strings.NewReader("This is not valid JSON"))

	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getState(w, r)

	res := w.Result()
	require.Equal(t, 500, res.StatusCode)
}

func TestGetDimensions_Success(t *testing.T) {

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("{\"foo\": [\"bar\"]}"))

	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	s.machine.UpdateDescription(rpc.FrontendDescription{
		Dimensions: machine.SwarmingDimensions{"foo": {"baz", "quux"}},
	})

	w := httptest.NewRecorder()

	s.getDimensions(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	var dict map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&dict)
	require.NoError(t, err)
	// Expect the whole dimension to be replaced.
	expected := map[string]interface{}{
		"foo": []interface{}{"baz", "quux"},
	}
	assert.Equal(t, expected, dict)
}

func TestOnBeginTask_Success(t *testing.T) {

	r := httptest.NewRequest("GET", "/on_begin_task", nil)

	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	require.False(t, s.machine.IsRunningSwarmingTask())
	s.onBeforeTaskSuccess.Reset()
	s.onAfterTaskSuccess.Reset()

	w := httptest.NewRecorder()

	s.onBeforeTask(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	require.True(t, s.machine.IsRunningSwarmingTask())
	assert.Equal(t, int64(1), s.onBeforeTaskSuccess.Get())
	assert.Equal(t, int64(0), s.onAfterTaskSuccess.Get())
}

func TestOnAfterTask_Success(t *testing.T) {

	r := httptest.NewRequest("GET", "/on_after_task", nil)

	s, err := New(&botmachine.Machine{})
	require.NoError(t, err)
	s.machine.SetIsRunningSwarmingTask(true)
	require.True(t, s.machine.IsRunningSwarmingTask())
	s.onBeforeTaskSuccess.Reset()
	s.onAfterTaskSuccess.Reset()

	w := httptest.NewRecorder()

	s.onAfterTask(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	require.False(t, s.machine.IsRunningSwarmingTask())
	assert.Equal(t, int64(0), s.onBeforeTaskSuccess.Get())
	assert.Equal(t, int64(1), s.onAfterTaskSuccess.Get())
}
