package emailservice

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
)

var errMyMockError = fmt.Errorf("my mock error")

const (
	validMessage = `From: Alert Service <alerts@skia.org>
To: test@example.com
Subject: An Alert!
Content-Type: text/html; charset=UTF-8
References: some-reference-id
In-Reply-To: some-reference-id

<html>
<body>

<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="https://example.com"/>
    <meta itemprop="name" content="Example"/>
  </div>
  <meta itemprop="description" content="Click the link"/>
</div>

<h1>Something happened</h1>
</body>
</html>
`
)

func createAppForTest(t *testing.T) *App {
	ret := &App{
		sendSucces:  metrics2.GetCounter("emailservice_send_success"),
		sendFailure: metrics2.GetCounter("emailservice_send_failure"),
	}
	ret.sendFailure.Reset()
	ret.sendSucces.Reset()

	return ret
}

func TestAppIncomingEmaiHandler_RequestBodyIsInvalidRFC2822Message_ReturnsHTTPError(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(""))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "Failed to parse RFC 2822 body.\n", w.Body.String())
	require.Equal(t, int64(1), app.sendFailure.Get())
	require.Equal(t, int64(0), app.sendSucces.Get())
}

func TestAppIncomingEmaiHandler_FromLineIsInvalid_ReturnsHTTPError(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(`From: me
To: you@example.org


`))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "Failed to parse From: address.\n", w.Body.String())
	require.Equal(t, int64(1), app.sendFailure.Get())
	require.Equal(t, int64(0), app.sendSucces.Get())
}

func TestAppIncomingEmaiHandler_ToLineIsInvalid_ReturnsHTTPError(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(`From: me@example.org
To: you


`))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "Failed to parse To: address.\n", w.Body.String())
	require.Equal(t, int64(1), app.sendFailure.Get())
	require.Equal(t, int64(0), app.sendSucces.Get())
}
