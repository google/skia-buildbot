package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

var once sync.Once

func handlerInit() {
	_, currentFile, _, _ := runtime.Caller(0)
	Init(filepath.Join(filepath.Dir(currentFile), "../.."), true)
}

func TestUpload(t *testing.T) {
	testutils.SmallTest(t)
	once.Do(handlerInit)

	testCases := []struct {
		value  string
		code   int
		recent string
		length int
	}{
		{
			value:  "{}",
			code:   200,
			recent: "{}",
			length: 1,
		},
		{
			value:  "{",
			code:   500,
			recent: "{}",
			length: 1,
		},
		{
			value:  `{"foo": "bar"}`,
			code:   200,
			recent: `{"foo": "bar"}`,
			length: 2,
		},
	}

	for _, tc := range testCases {
		b := bytes.NewBufferString(tc.value)
		r, err := http.NewRequest("POST", "/upload", b)
		assert.NoError(t, err)
		w := httptest.NewRecorder()
		UploadHandler(w, r)
		if got, want := w.Code, tc.code; got != want {
			t.Errorf("Failed case %q, Got %v Want %v", tc.value, got, want)
		}
		if got, want := len(recent), tc.length; got != want {
			t.Errorf("Failed case %q, Got %v Want %v", tc.value, got, want)
		}
		if got, want := recent[0].JSON, tc.recent; got != want {
			t.Errorf("Failed case %q, Got %v Want %v", tc.value, got, want)
		}
	}
}

func TestMain(t *testing.T) {
	testutils.SmallTest(t)
	once.Do(handlerInit)

	b := bytes.NewBufferString(`{"key": "a distinct string"}`)
	r, err := http.NewRequest("POST", "/upload", b)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	UploadHandler(w, r)

	r, err = http.NewRequest("GET", "/", b)
	assert.NoError(t, err)
	w = httptest.NewRecorder()
	MainHandler(w, r)
	assert.Contains(t, w.Body.String(), "a distinct string")
}
