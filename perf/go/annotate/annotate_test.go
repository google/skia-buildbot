package annotate

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.skia.org/infra/go/login"
)

var once sync.Once

func loginInit() {
	login.Init("id", "secret", "http://localhost", "salt", login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST)
}

func TestMissingLogin(t *testing.T) {
	once.Do(loginInit)
	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "http://skiaperf.com/annotate", nil)
	if err != nil {
		t.Fatal(err)
	}
	Handler(w, r)
	if got, want := w.Code, 500; got != want {
		t.Errorf("Failed to reject missing login: Got %v Want %v", got, want)
	}
	if !strings.Contains(w.Body.String(), "logged in") {
		t.Errorf("Failed to reject for the reason of a missing login.")
	}
}

func TestGoodLogin(t *testing.T) {
	once.Do(loginInit)
	w := httptest.NewRecorder()
	r, err := http.NewRequest("POST", "http://skiaperf.com/annotate", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &login.Session{
		Email:     "fred@example.com",
		AuthScope: login.DEFAULT_SCOPE,
		Token:     nil,
	}
	cookie, err := login.CookieFor(s)
	if err != nil {
		t.Fatal(err)
	}
	r.AddCookie(cookie)
	Handler(w, r)
	if got, want := w.Code, 500; got != want {
		t.Errorf("Failed to reject missing body: Got %v Want %v", got, want)
	}
	if strings.Contains(w.Body.String(), "logged in") {
		t.Errorf("Failed to accept a good login.")
	}
}
