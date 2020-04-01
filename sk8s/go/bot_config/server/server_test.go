package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"gotest.tools/v3/assert"
)

func TestGetSettings_Success(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("GET", "/get_settings", nil)
	s, err := NewServer()
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

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("{}"))
	s, err := NewServer()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getState(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	var dict map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&dict)
	require.NoError(t, err)
	assert.Equal(t, someRackName, dict["sk_rack"])
}
