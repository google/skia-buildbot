package login

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/stretchr/testify/assert"
)

var once sync.Once

func loginInit() {
	Init("id", "secret", "http://localhost", "salt", DEFAULT_SCOPE, DEFAULT_DOMAIN_WHITELIST, false)
}

func TestLoginURL(t *testing.T) {
	once.Do(loginInit)
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	LoginURL(w, r)
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), SESSION_COOKIE_NAME, "Session cookie should be set.")
	cookie := &http.Cookie{
		Name:  SESSION_COOKIE_NAME,
		Value: "some-random-state",
	}
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	url := LoginURL(w, r)
	assert.NotContains(t, w.HeaderMap.Get("Set-Cookie"), SESSION_COOKIE_NAME, "Session cookie should be set.")
	assert.Contains(t, url, "some-random-state", "Pass state in Login URL.")
}

func TestLoggedInAs(t *testing.T) {
	once.Do(loginInit)

	r, err := http.NewRequest("GET", "http://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, LoggedInAs(r), "", "No skid cookie means not logged in.")

	s := Session{
		Email:     "fred@example.com",
		AuthScope: DEFAULT_SCOPE,
		Token: &oauth.Token{
			AccessToken:  "dummy",
			RefreshToken: "",
			Expiry:       time.Now().Add(time.Hour),
		},
	}
	cookie, err := CookieFor(&s)
	if err != nil {
		t.Fatal(err)
	}
	r.AddCookie(cookie)
	assert.Equal(t, LoggedInAs(r), "fred@example.com", "Correctly get logged in email.")
}
