package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/sk8s/go/bot_config/adb/mocks"
)

func TestGetSettings_Success(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/get_settings", nil)
	s, err := New()
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
	unittest.SmallTest(t)

	const someRackName = "some-rack-name"

	err := os.Setenv("MY_RACK_NAME", someRackName)
	require.NoError(t, err)

	r := httptest.NewRequest("POST", "/get_state", strings.NewReader("{\"foo\":\"bar\"}"))

	s, err := New()
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
}

func TestGetState_ErrOnInvalidJSON(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("POST", "/get_state", strings.NewReader("This is not valid JSON"))

	s, err := New()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getState(w, r)

	res := w.Result()
	require.Equal(t, 500, res.StatusCode)
}

func TestGetDimensions_Success(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("{\"foo\": [\"bar\"]}"))

	s, err := New()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	a := &mocks.Adb{}
	a.On("DimensionsFromProperties", testutils.AnyContext, mock.Anything).Return(map[string][]string{
		"foo": {"bar", "baz"},
	}, nil)
	s.a = a

	s.getDimensions(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	var dict map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&dict)
	require.NoError(t, err)
	expected := map[string]interface{}{
		"foo": []interface{}{"bar", "baz"},
	}
	assert.Equal(t, expected, dict)
}

func TestGetDimensions_ErrOnInvalidJSON(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("This is not valid JSON."))

	s, err := New()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getDimensions(w, r)

	res := w.Result()
	require.Equal(t, 500, res.StatusCode)
}

func TestGetDimensions_ErrOnAdbFail(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("{}"))

	const adbError = "adb no device"

	s, err := New()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	a := &mocks.Adb{}
	a.On("DimensionsFromProperties", mock.Anything, mock.Anything).Return(nil, fmt.Errorf(adbError))
	s.a = a

	s.getDimensions(w, r)

	res := w.Result()
	assert.Equal(t, 500, res.StatusCode)
	b, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Contains(t, string(b), adbError)
}
