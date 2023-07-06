package notify

import (
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/notify/mocks"
)

const (
	newMessage = "<b>Alert</b><br><br>\n<p>\n\tA Perf Regression (High) has been found at:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\">https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n  For:\n</p>\n<p style=\"padding: 1em;\">\n  <a href=\"https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664\">https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tWith 10 matching traces.\n</p>\n<p>\n   And direction High.\n</p>\n<p>\n\tFrom Alert <a href=\"https://perf.skia.org/a/?123\">MyAlert</a>\n</p>\n"
	newSubject = "MyAlert - Regression found for d261e10 -  2y 40w - An example commit use for testing."

	missingMessage = "<b>Alert</b><br><br>\n<p>\n\tA Perf Regression (High) can no longer be found at:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\">https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tFor:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664\">https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tWith 10 matching traces.\n</p>\n<p>\n\tAnd direction High.\n</p>\n<p>\n\tFrom Alert <a href=\"https://perf.skia.org/a/?123\">MyAlert</a>\n</p>\n"
	missingSubject = "MyAlert - Regression no longer found for d261e10 -  2y 40w - An example commit use for testing."
)

const (
	mockThreadingID = "123456789"
	instanceURL     = "https://perf.skia.org"
)

var (
	alertForTest = &alerts.Alert{
		IDAsString:  "123",
		Alert:       "someone@example.org, someother@example.com ",
		DisplayName: "MyAlert",
	}

	errMock = errors.New("my mock error")
)

func TestExampleSendWithHTMLFormatter_HappyPath(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newMessage, newSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingMessage, missingSubject).Return(nil)

	n := new(NewHTMLFormatter(), tr, instanceURL)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestExampleSendWithHTMLFormatter_SendRegressionMissingReturnsError_ReturnsError(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newMessage, newSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingMessage, missingSubject).Return(errMock)

	n := new(NewHTMLFormatter(), tr, instanceURL)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.ErrorIs(t, err, errMock)
	require.Contains(t, err.Error(), "sending regression missing message")
}

func TestExampleSendWithHTMLFormatter_SendNewRegressionReturnsError_ReturnsError(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newMessage, newSubject).Return("", errMock)

	n := new(NewHTMLFormatter(), tr, instanceURL)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.ErrorIs(t, err, errMock)
	require.Contains(t, err.Error(), "sending new regression message")
}

func TestExampleSendWithHTMLFormatterAndEMailTransport_HappyPath(t *testing.T) {
	const expectedMessageID = "<the-actual-message-id>"

	subjects := []string{newSubject, missingSubject}
	subjectIndex := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(b), "<b>Alert</b>")
		require.Contains(t, string(b), subjects[subjectIndex])
		subjectIndex++

		w.Header().Add("x-message-id", mockThreadingID)
		w.WriteHeader(http.StatusOK)
	}))
	tr := NewEmailTransport()
	emailClient := emailclient.NewAt(s.URL)
	tr.client = emailClient

	n := new(NewHTMLFormatter(), tr, instanceURL)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}
