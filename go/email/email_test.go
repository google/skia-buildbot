package email

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const subject = "An Alert!"

const expectedMessage = `From: Alert Service <alerts@skia.org>
To: test@example.com
Subject: An Alert!
Content-Type: text/html; charset=UTF-8
References: <some-reference-id>
In-Reply-To: <some-reference-id>

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

func TestParseRFC2822Message_HappyPath(t *testing.T) {
	from, to, subject, body, err := ParseRFC2822Message([]byte(`From: Alerts <alerts@skia.org>
To:  A Display Name <a@example.com>, B <b@example.org>,,
Subject: My Stuff

Hi!
`))
	require.NoError(t, err)
	require.Equal(t, "Alerts <alerts@skia.org>", from)
	require.Equal(t, []string{"A Display Name <a@example.com>", "B <b@example.org>"}, to)
	require.Equal(t, "My Stuff", subject)
	require.Equal(t, "Hi!\n", body)
}

func TestParseRFC2822Message_EmptyInput_ReturnsError(t *testing.T) {
	_, _, _, _, err := ParseRFC2822Message([]byte(``))
	require.Contains(t, err.Error(), "Failed to find a From: line")
}

func TestParseRFC2822Message_EmptyToLine_ReturnsError(t *testing.T) {
	_, _, _, _, err := ParseRFC2822Message([]byte(`From: Alerts <alerts@skia.org>
To:  ,,,
Subject: My Stuff

Hi!
`))
	require.Contains(t, err.Error(), "Failed to find any To: addresses")
}

func TestParseRFC2822Message_MissingSubject_DefaultSubjectIsReturned(t *testing.T) {
	from, to, subject, body, err := ParseRFC2822Message([]byte(`From: Alerts <alerts@skia.org>
To: someone@example.org

Hi!
`))
	require.NoError(t, err)
	require.Equal(t, "Alerts <alerts@skia.org>", from)
	require.Equal(t, []string{"someone@example.org"}, to)
	require.Equal(t, "(no subject)", subject)
	require.Equal(t, "Hi!\n", body)
}

func TestFormatAsRFC2822_HappyPath(t *testing.T) {
	body := `<h1>Testing</h1>`
	ref := "<some-reference-id>"
	messageID := "<foo-bar-baz@skia.org>"
	markup, err := GetViewActionMarkup("https://example.com", "Example", "Click the link")
	require.NoError(t, err)
	actual, err := FormatAsRFC2822("Alerts", "alerts@skia.org", []string{"someone@example.org"}, "Your Alert", body, markup, ref, messageID)
	require.NoError(t, err)
	expected := `From: Alerts <alerts@skia.org>
To: someone@example.org
Subject: Your Alert
Content-Type: text/html; charset=UTF-8
References: <some-reference-id>
In-Reply-To: <some-reference-id>
Message-ID: <foo-bar-baz@skia.org>

<html>
<body>

<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="https://example.com"/>
    <meta itemprop="name" content="Example"/>
  </div>
  <meta itemprop="description" content="Click the link"/>
</div>

<h1>Testing</h1>
</body>
</html>
`
	require.Equal(t, expected, actual.String())
}

func TestFormatAsRFC2822_NoThreadID_MessageDoesNotContainInReplyToOrReferencesHeaders(t *testing.T) {
	body := `<h1>Testing</h1>`
	ref := ""
	messageID := "<foo-bar-baz@skia.org>"
	markup, err := GetViewActionMarkup("https://example.com", "Example", "Click the link")
	require.NoError(t, err)
	actual, err := FormatAsRFC2822("Alerts", "alerts@skia.org", []string{"someone@example.org"}, "Your Alert", body, markup, ref, messageID)
	require.NoError(t, err)
	expected := `From: Alerts <alerts@skia.org>
To: someone@example.org
Subject: Your Alert
Content-Type: text/html; charset=UTF-8
Message-ID: <foo-bar-baz@skia.org>

<html>
<body>

<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="https://example.com"/>
    <meta itemprop="name" content="Example"/>
  </div>
  <meta itemprop="description" content="Click the link"/>
</div>

<h1>Testing</h1>
</body>
</html>
`
	require.Equal(t, expected, actual.String())
}

func TestFormatAsRFC2822_NoThreadIDOrMessageID_MessageDoesNotContainInReplyToOrReferenceOrMessageIDHeaders(t *testing.T) {
	body := `<h1>Testing</h1>`
	ref := ""
	messageID := ""
	markup, err := GetViewActionMarkup("https://example.com", "Example", "Click the link")
	require.NoError(t, err)
	actual, err := FormatAsRFC2822("Alerts", "alerts@skia.org", []string{"someone@example.org"}, "Your Alert", body, markup, ref, messageID)
	require.NoError(t, err)
	expected := `From: Alerts <alerts@skia.org>
To: someone@example.org
Subject: Your Alert
Content-Type: text/html; charset=UTF-8

<html>
<body>

<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="https://example.com"/>
    <meta itemprop="name" content="Example"/>
  </div>
  <meta itemprop="description" content="Click the link"/>
</div>

<h1>Testing</h1>
</body>
</html>
`
	require.Equal(t, expected, actual.String())
}

func TestFormatAsRFC2822_NoMessageID_MessageDoesNotContainMessageIDHeader(t *testing.T) {
	body := `<h1>Testing</h1>`
	ref := "<some-reference-id>"
	messageID := ""
	markup, err := GetViewActionMarkup("https://example.com", "Example", "Click the link")
	require.NoError(t, err)
	actual, err := FormatAsRFC2822("Alerts", "alerts@skia.org", []string{"someone@example.org"}, "Your Alert", body, markup, ref, messageID)
	require.NoError(t, err)
	expected := `From: Alerts <alerts@skia.org>
To: someone@example.org
Subject: Your Alert
Content-Type: text/html; charset=UTF-8
References: <some-reference-id>
In-Reply-To: <some-reference-id>

<html>
<body>

<div itemscope itemtype="http://schema.org/EmailMessage">
  <div itemprop="potentialAction" itemscope itemtype="http://schema.org/ViewAction">
    <link itemprop="target" href="https://example.com"/>
    <meta itemprop="name" content="Example"/>
  </div>
  <meta itemprop="description" content="Click the link"/>
</div>

<h1>Testing</h1>
</body>
</html>
`
	require.Equal(t, expected, actual.String())
}
