package anomaly

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestHandler_AbsoluteStep(t *testing.T) {
	// A simple step up in the middle.
	trace := []float32{
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 10
		5, 5, 5, 5, 5, 5, 5, 5, 5, 5, // 10
	}
	req := DetectRequest{
		Trace:          trace,
		Radius:         5,
		Threshold:      3.0,
		Algorithm:      types.AbsoluteStep,
		GroupAnomalies: true,
	}
	body, err := json.Marshal(req)
	assert.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/playground/anomaly/v1/detect", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	Handler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var detectResp DetectResponse
	err = json.NewDecoder(resp.Body).Decode(&detectResp)
	assert.NoError(t, err)

	assert.NotEmpty(t, detectResp.Anomalies)
	if len(detectResp.Anomalies) > 0 {
		a := detectResp.Anomalies[0]
		assert.Equal(t, 10, a.StartRevision)
		assert.Equal(t, "untriaged", a.State)
	}
}

func TestHandler_NoAnomaly(t *testing.T) {
	// Flat trace
	trace := []float32{
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	}
	req := DetectRequest{
		Trace:     trace,
		Radius:    5,
		Threshold: 3.0,
		Algorithm: types.OriginalStep,
	}
	body, err := json.Marshal(req)
	assert.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/playground/anomaly/v1/detect", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	Handler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var detectResp DetectResponse
	err = json.NewDecoder(resp.Body).Decode(&detectResp)
	assert.NoError(t, err)

	assert.Empty(t, detectResp.Anomalies)
}

func TestHandler_MultipleAnomalies(t *testing.T) {
	// Step up then step down.
	trace := []float32{
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 0-9
		5, 5, 5, 5, 5, 5, 5, 5, 5, 5, // 10-19 (Step up at 10)
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 20-29 (Step down at 20)
	}
	req := DetectRequest{
		Trace:          trace,
		Radius:         3, // Smaller radius to avoid overlapping windows merging too much
		Threshold:      2.0,
		Algorithm:      types.OriginalStep,
		GroupAnomalies: true,
	}
	body, err := json.Marshal(req)
	assert.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/playground/anomaly/v1/detect", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	Handler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var detectResp DetectResponse
	err = json.NewDecoder(resp.Body).Decode(&detectResp)
	assert.NoError(t, err)

	// Should have at least 2 anomalies.
	assert.True(t, len(detectResp.Anomalies) >= 2, "Expected at least 2 anomalies, got %d", len(detectResp.Anomalies))
}

func TestHandler_BadRequest(t *testing.T) {
	r := httptest.NewRequest("POST", "/_/playground/anomaly/v1/detect", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	Handler(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestHandler_ShortTrace(t *testing.T) {
	trace := []float32{1, 5}
	req := DetectRequest{
		Trace:     trace,
		Radius:    5,
		Threshold: 3.0,
		Algorithm: types.OriginalStep,
	}
	body, err := json.Marshal(req)
	assert.NoError(t, err)

	r := httptest.NewRequest("POST", "/_/playground/anomaly/v1/detect", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	Handler(w, r)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var detectResp DetectResponse
	err = json.NewDecoder(resp.Body).Decode(&detectResp)
	assert.NoError(t, err)

	assert.Empty(t, detectResp.Anomalies)
}
