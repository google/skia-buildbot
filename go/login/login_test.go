package login

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

var once sync.Once

func loginInit() {
	initLogin("id", "secret", "http://localhost", "salt", DEFAULT_SCOPE, DEFAULT_ALLOWED_DOMAINS)
}

func TestLoginURL(t *testing.T) {
	unittest.SmallTest(t)
	once.Do(loginInit)
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http://example.com/", nil)
	r.Header.Set("Referer", "https://foo.org")
	if err != nil {
		t.Fatal(err)
	}
	url := LoginURL(w, r)
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), SESSION_COOKIE_NAME, "Session cookie should be set.")
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), "SameSite=None", "SameSite should be set.")
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), "Secure", "Secure should be set.")

	assert.Contains(t, url, "approval_prompt=auto", "Not forced into prompt.")
	cookie := &http.Cookie{
		Name:  SESSION_COOKIE_NAME,
		Value: "some-random-state",
	}
	assert.Contains(t, url, "%3Ahttps%3A%2F%2Ffoo.org")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	url = LoginURL(w, r)
	assert.NotContains(t, w.HeaderMap.Get("Set-Cookie"), SESSION_COOKIE_NAME, "Session cookie should be set.")
	assert.Contains(t, url, "some-random-state", "Pass state in Login URL.")
}

func TestLoggedInAs(t *testing.T) {
	unittest.SmallTest(t)
	once.Do(loginInit)
	setActiveAllowLists(DEFAULT_ALLOWED_DOMAINS)

	r, err := http.NewRequest("GET", "http://www.skia.org/", nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, LoggedInAs(r), "", "No skid cookie means not logged in.")

	s := Session{
		Email:     "fred@chromium.org",
		ID:        "12345",
		AuthScope: DEFAULT_SCOPE[0],
		Token:     nil,
	}
	cookie, err := CookieFor(&s, r)
	assert.NoError(t, err)
	assert.Equal(t, "skia.org", cookie.Domain)
	r.AddCookie(cookie)
	assert.Equal(t, LoggedInAs(r), "fred@chromium.org", "Correctly get logged in email.")
	w := httptest.NewRecorder()
	url := LoginURL(w, r)
	assert.Contains(t, url, "approval_prompt=auto", "Not forced into prompt.")

	delete(activeUserDomainAllowList, "chromium.org")
	assert.Equal(t, LoggedInAs(r), "", "Not in the domain allow list.")
	url = LoginURL(w, r)
	assert.Contains(t, url, "prompt=consent", "Force into prompt.")

	activeUserEmailAllowList["fred@chromium.org"] = true
	assert.Equal(t, LoggedInAs(r), "fred@chromium.org", "Found in the email allow list.")
}

func TestDomainFromHost(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "localhost", domainFromHost("localhost:10110"))
	assert.Equal(t, "localhost", domainFromHost("localhost"))
	assert.Equal(t, "skia.org", domainFromHost("skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org:443"))
	assert.Equal(t, "skia.org", domainFromHost("example.com:443"))
}

func TestSplitAuthAllowList(t *testing.T) {
	unittest.SmallTest(t)

	type testCase struct {
		Input           string
		ExpectedDomains map[string]bool
		ExpectedEmails  map[string]bool
	}

	tests := []testCase{
		{
			Input: "google.com chromium.org skia.org",
			ExpectedDomains: map[string]bool{
				"google.com":   true,
				"chromium.org": true,
				"skia.org":     true,
			},
			ExpectedEmails: map[string]bool{},
		},
		{
			Input: "google.com chromium.org skia.org service-account@proj.iam.gserviceaccount.com",
			ExpectedDomains: map[string]bool{
				"google.com":   true,
				"chromium.org": true,
				"skia.org":     true,
			},
			ExpectedEmails: map[string]bool{
				"service-account@proj.iam.gserviceaccount.com": true,
			},
		},
		{
			Input:           "user@example.com service-account@proj.iam.gserviceaccount.com",
			ExpectedDomains: map[string]bool{},
			ExpectedEmails: map[string]bool{
				"user@example.com": true,
				"service-account@proj.iam.gserviceaccount.com": true,
			},
		},
	}

	for _, tc := range tests {
		d, e := splitAuthAllowList(tc.Input)
		assert.Equal(t, tc.ExpectedDomains, d)
		assert.Equal(t, tc.ExpectedEmails, e)
	}
}

func TestIsAuthorized(t *testing.T) {
	unittest.SmallTest(t)
	once.Do(loginInit)
	setActiveAllowLists("google.com chromium.org skia.org service-account@proj.iam.gserviceaccount.com")

	assert.True(t, isAuthorized("fred@chromium.org"))
	assert.True(t, isAuthorized("service-account@proj.iam.gserviceaccount.com"))

	assert.False(t, isAuthorized("fred@example.com"))
	assert.False(t, isAuthorized("evil@proj.iam.gserviceaccount.com"))
}

func TestIsAuthorized_Gmail(t *testing.T) {
	unittest.SmallTest(t)
	once.Do(loginInit)
	setActiveAllowLists("google.com example@gmail.com")

	assert.True(t, isAuthorized("example@gmail.com"))
	assert.True(t, isAuthorized("ex.amp.le@gmail.com"))
	assert.True(t, isAuthorized("example+somethi.ng@gmail.com"))
	assert.True(t, isAuthorized("ex.amp.le+something@gmail.com"))

	assert.False(t, isAuthorized("fred@gmail.com"))
	assert.False(t, isAuthorized("example@g.mail.com"))
}

func TestNormalizeGmailAddress(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "example", normalizeGmailAddress(".ex.ampl.e."))
	assert.Equal(t, "example", normalizeGmailAddress("exa.mple"))
	assert.Equal(t, "example", normalizeGmailAddress("example+"))
	assert.Equal(t, "example", normalizeGmailAddress("example+decoration+more+plus"))
	assert.Equal(t, "example", normalizeGmailAddress("examp.le+.dec."))
}
