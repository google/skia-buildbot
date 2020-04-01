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
	"go.skia.org/infra/sk8s/go/bot_config/adb/adbtest"
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

func TestGetDimensions_Success(t *testing.T) {
	unittest.SmallTest(t)

	r := httptest.NewRequest("POST", "/get_settings", strings.NewReader("{}"))

	ctxWithAdbProperties := adbtest.AdbMockHappy(t, `
	[ro.product.manufacturer]: [asus]
	[ro.product.model]: [Nexus 7]
	[ro.product.name]: [razor]
`)
	r = r.WithContext(ctxWithAdbProperties)
	s, err := NewServer()
	require.NoError(t, err)
	w := httptest.NewRecorder()

	s.getDimensions(w, r)

	res := w.Result()
	assert.Equal(t, 200, res.StatusCode)
	var dict map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&dict)
	require.NoError(t, err)
	expected := map[string]interface{}{
		"android_devices": []interface{}{"1"},
		"inside_docker":   []interface{}{"1", "containerd"},
		"os":              []interface{}{"Android"},
		"zone":            []interface{}{"us", "us-skolo", "us-skolo-1"},
	}
	assert.DeepEqual(t, expected, dict)
}
