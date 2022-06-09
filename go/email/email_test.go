package email

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/api/gmail/v1"
)

const subject = "An Alert!"

const expectedMessage = `From: Alert Service <alerts@skia.org>
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

// Mock out the http.Client by supplying our own http.RoundTripper.
type myTransport struct {
	t *testing.T
}

// RoundTrip implements http.RoundTripper.
func (m *myTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Check the URL.
	require.Equal(m.t, "https://gmail.googleapis.com/gmail/v1/users/alerts%40skia.org/messages/send?alt=json&prettyPrint=false", r.URL.String())

	// Confirm the body, which is a JSON encoded gmail.Message, is correct.
	var msg gmail.Message
	err := json.NewDecoder(r.Body).Decode(&msg)
	require.NoError(m.t, err)
	require.Equal(m.t, subject, msg.Snippet)
	require.Equal(m.t, int64(559), msg.SizeEstimate)

	// The actual RFC2822 message is base64 encoded in the .Raw member.
	b, err := base64.URLEncoding.DecodeString(msg.Raw)
	require.NoError(m.t, err)
	require.Equal(m.t, expectedMessage, string(b))

	// Now supply a response that the GMail client library will accept.
	buf := bytes.NewBufferString(`HTTP/1.1 200 OK
Date: Mon, 27 Jul 2020 12:28:53 GMT
Last-Modified: Wed, 22 Jul 2020 19:15:56 GMT
Content-Type: text/html

{
	"error": {
		"code": 200
	}
}`)
	return http.ReadResponse(bufio.NewReader(buf), r)
}

var _ http.RoundTripper = (*myTransport)(nil)

func TestGMailSendWithMarkup(t *testing.T) {
	unittest.SmallTest(t)
	c := httputils.DefaultClientConfig().Client()
	// Swap in our mock transport.
	c.Transport = &myTransport{t: t}

	// Construct a new *GMail instance.
	service, err := gmail.New(c)
	require.NoError(t, err)
	gm := &GMail{
		service: service,
		from:    "alerts@skia.org",
	}
	markup, err := GetViewActionMarkup("https://example.com", "Example", "Click the link")
	require.NoError(t, err)
	ref := "some-reference-id"

	// Send email, the validation happens in myTransport.
	_, err = gm.SendWithMarkup("Alert Service", []string{"test@example.com"}, subject, "<h1>Something happened</h1>", markup, ref)
	require.NoError(t, err)
}
