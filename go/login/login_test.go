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
	initLogin("id", "secret", "http://localhost", "salt", DEFAULT_SCOPE, DEFAULT_DOMAIN_WHITELIST)
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
	setActiveWhitelists(DEFAULT_DOMAIN_WHITELIST)

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

	delete(activeUserDomainWhiteList, "chromium.org")
	assert.Equal(t, LoggedInAs(r), "", "Not in the domain whitelist.")
	url = LoginURL(w, r)
	assert.Contains(t, url, "prompt=consent", "Force into prompt.")

	activeUserEmailWhiteList["fred@chromium.org"] = true
	assert.Equal(t, LoggedInAs(r), "fred@chromium.org", "Found in the email whitelist.")
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

func TestSplitAuthWhiteList(t *testing.T) {
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
		d, e := splitAuthWhiteList(tc.Input)
		assert.Equal(t, tc.ExpectedDomains, d)
		assert.Equal(t, tc.ExpectedEmails, e)
	}
}

func TestInWhitelist(t *testing.T) {
	unittest.SmallTest(t)
	once.Do(loginInit)
	setActiveWhitelists("google.com chromium.org skia.org service-account@proj.iam.gserviceaccount.com")

	assert.True(t, inWhitelist("fred@chromium.org"))
	assert.True(t, inWhitelist("service-account@proj.iam.gserviceaccount.com"))

	assert.False(t, inWhitelist("fred@example.com"))
	assert.False(t, inWhitelist("evil@proj.iam.gserviceaccount.com"))
}
