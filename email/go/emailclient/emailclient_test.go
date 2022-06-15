package emailclient

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestClientSendWithMarkup_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	const expected = `From: Alert Manager <alerts@skia.org>
To: someone@example.org
Subject: Alert!
Content-Type: text/html; charset=UTF-8
References: some-thread-reference
In-Reply-To: some-thread-reference

<html>
<body>
<h2>Hi!</h2>

</body>
</html>
`
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, expected, string(b))
		w.WriteHeader(http.StatusOK)
	}))
	c := New()
	c.emailServiceURL = s.URL

	err := c.SendWithMarkup("Alert Manager", "alerts@skia.org", []string{"someone@example.org"}, "Alert!", "", "<h2>Hi!</h2>", "some-thread-reference")
	require.NoError(t, err)
}

func TestClientSendWithMarkup_HTTPRequestFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	c := New()
	c.emailServiceURL = s.URL

	err := c.SendWithMarkup("Alert Manager", "alerts@skia.org", []string{"someone@example.org"}, "Alert!", "", "<h2>Hi!</h2>", "some-thread-reference")
	require.Contains(t, err.Error(), "Failed to send")
}
