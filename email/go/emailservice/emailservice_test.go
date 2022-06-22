package emailservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils/unittest"
)

const myMesageID = "<abcdef>"

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

func createAppForTest(t *testing.T, handler http.Handler) *App {
	ret := &App{
		sendSuccess: metrics2.GetCounter("emailservice_send_success"),
		sendFailure: metrics2.GetCounter("emailservice_send_failure"),
	}
	ret.sendFailure.Reset()
	ret.sendSuccess.Reset()

	if handler != nil {
		s := httptest.NewServer(handler)
		t.Cleanup(s.Close)
		ret.sendgridClient = &sendgrid.Client{
			Request: sendgrid.GetRequest("my key", "/v3/mail/send", s.URL),
		}
	}

	return ret
}

func TestAppIncomingEmaiHandler_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Message-Id", myMesageID)
		resp := Response{
			// No errors in response means the request was a success.
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(validMessage))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, myMesageID, w.Header().Get("X-Message-Id"))
	require.Equal(t, int64(0), app.sendFailure.Get())
	require.Equal(t, int64(1), app.sendSuccess.Get())
}

func TestAppIncomingEmaiHandler_RequestBodyIsInvalidRFC2822Message_ReturnsHTTPError(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(""))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "Failed to convert RFC2822 body to SendGrid API format\n", w.Body.String())
	require.Equal(t, int64(1), app.sendFailure.Get())
	require.Equal(t, int64(0), app.sendSuccess.Get())
}

func TestAppIncomingEmaiHandler_ServerReturnsError_ReturnsHTTPError(t *testing.T) {
	unittest.SmallTest(t)
	app := createAppForTest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Errors: []Error{
				{
					Message: "something failed on the server",
				},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/send", bytes.NewBufferString(validMessage))

	app.incomingEmaiHandler(w, r)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "something failed on the server")
	require.Equal(t, int64(1), app.sendFailure.Get())
	require.Equal(t, int64(0), app.sendSuccess.Get())
}

func TestConvertRFC2822ToSendGrid_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	body := bytes.NewBufferString(`From: Alert Service <alerts@skia.org>
To: test@example.com, B <b@example.com>
Subject: An Alert!
Content-Type: text/html; charset=UTF-8
References: some-reference-id
In-Reply-To: some-reference-id

Hi!
`)
	m, err := convertRFC2822ToSendGrid(body)
	require.NoError(t, err)
	require.Equal(t, "{\"from\":{\"name\":\"Alert Service\",\"email\":\"alerts@skia.org\"},\"subject\":\"An Alert!\",\"personalizations\":[{\"to\":[{\"email\":\"test@example.com\"},{\"name\":\"B\",\"email\":\"b@example.com\"}]}],\"content\":[{\"type\":\"text/html\",\"value\":\"Hi!\\n\"}]}", string(mail.GetRequestBody(m)))
}

func TestConvertRFC2822ToSendGrid_ToLineIsInvalid_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	body := bytes.NewBufferString(`From: Alert Service <alerts@skia.org>
To: you
Subject: An Alert!
Content-Type: text/html; charset=UTF-8

Hi!
`)
	_, err := convertRFC2822ToSendGrid(body)
	require.Contains(t, err.Error(), "Failed to parse To: address")
}

func TestConvertRFC2822ToSendGrid_FromLineIsInvalid_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	body := bytes.NewBufferString(`From: me
To: you@example.com
Subject: An Alert!

Hi!
`)
	_, err := convertRFC2822ToSendGrid(body)
	require.Contains(t, err.Error(), "Failed to parse From: address")
}
