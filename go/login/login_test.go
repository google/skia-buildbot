package login

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/securecookie"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	loginMocks "go.skia.org/infra/go/login/mocks"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/secret/mocks"
	"go.skia.org/infra/go/testutils"
	"golang.org/x/oauth2"
)

const (
	saltForTesting = "salt"

	sessionIDForTesting = "abcdef0123456"

	codeForTesting = "oauth2 code for testing"
)

var (
	errMockError = fmt.Errorf("error returned from mocks")
)

var once sync.Once

func loginInit() {
	initLogin("id", "secret", "http://localhost", saltForTesting, DEFAULT_ALLOWED_DOMAINS)
}

func TestLoginURL(t *testing.T) {
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
		AuthScope: emailScope,
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

func TestAuthorizedEmail(t *testing.T) {
	once.Do(loginInit)
	setActiveAllowLists(DEFAULT_ALLOWED_DOMAINS)
	// In place of SessionMiddleware function.
	middleware := func(r *http.Request) *http.Request {
		session, _ := getSession(r)
		ctx := context.WithValue(r.Context(), loginCtxKey, session)
		return r.WithContext(ctx)
	}

	r, err := http.NewRequest("GET", "http://www.skia.org/", nil)
	if err != nil {
		t.Fatal(err)
	}
	r = middleware(r)

	assert.Equal(t, AuthorizedEmail(r.Context()), "", "No skid cookie means not logged in.")

	s := Session{
		Email:     "fred@chromium.org",
		ID:        "12345",
		AuthScope: emailScope,
		Token:     nil,
	}
	cookie, err := CookieFor(&s, r)
	assert.NoError(t, err)
	assert.Equal(t, "skia.org", cookie.Domain)
	r.AddCookie(cookie)
	r = middleware(r)
	assert.Equal(t, AuthorizedEmail(r.Context()), "fred@chromium.org", "Correctly get logged in email.")
	w := httptest.NewRecorder()
	url := LoginURL(w, r)
	assert.Contains(t, url, "approval_prompt=auto", "Not forced into prompt.")

	delete(activeUserDomainAllowList, "chromium.org")
	assert.Equal(t, AuthorizedEmail(r.Context()), "", "Not in the domain allow list.")
	url = LoginURL(w, r)
	assert.Contains(t, url, "prompt=consent", "Force into prompt.")

	activeUserEmailAllowList["fred@chromium.org"] = true
	assert.Equal(t, AuthorizedEmail(r.Context()), "fred@chromium.org", "Found in the email allow list.")
}

func TestDomainFromHost(t *testing.T) {
	assert.Equal(t, "localhost", domainFromHost("localhost:10110"))
	assert.Equal(t, "localhost", domainFromHost("localhost"))
	assert.Equal(t, "skia.org", domainFromHost("skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org:443"))
	assert.Equal(t, "skia.org", domainFromHost("example.com:443"))
}

func TestIsAuthorized(t *testing.T) {
	once.Do(loginInit)
	setActiveAllowLists("google.com chromium.org skia.org service-account@proj.iam.gserviceaccount.com")

	assert.True(t, isAuthorized("fred@chromium.org"))
	assert.True(t, isAuthorized("service-account@proj.iam.gserviceaccount.com"))

	assert.False(t, isAuthorized("fred@example.com"))
	assert.False(t, isAuthorized("evil@proj.iam.gserviceaccount.com"))
}

func TestIsAuthorized_Gmail(t *testing.T) {
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
	assert.Equal(t, "example", normalizeGmailAddress(".ex.ampl.e."))
	assert.Equal(t, "example", normalizeGmailAddress("exa.mple"))
	assert.Equal(t, "example", normalizeGmailAddress("example+"))
	assert.Equal(t, "example", normalizeGmailAddress("example+decoration+more+plus"))
	assert.Equal(t, "example", normalizeGmailAddress("examp.le+.dec."))
}

func TestSessionMiddleware(t *testing.T) {

	// Setup.
	once.Do(loginInit)
	setActiveAllowLists(DEFAULT_ALLOWED_DOMAINS)

	// Helper function to set up a request with the given Session and test the
	// middleware, verifying that we get the session back via GetSession.
	test := func(t *testing.T, expect *Session) {
		// Create a request.
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)
		if expect != nil {
			cookie, err := CookieFor(expect, req)
			require.NoError(t, err)
			req.AddCookie(cookie)
		}

		// Create an http.Handler which uses LoginMiddleware and checks the
		// return value of GetSession against the expectation.
		handler := SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actual := GetSession(r.Context())
			assertdeep.Equal(t, expect, actual)
		}))

		// Run the test.
		handler.ServeHTTP(nil, req)
	}

	// Test cases.
	t.Run("not logged in", func(t *testing.T) {
		test(t, nil)
	})
	t.Run("logged in", func(t *testing.T) {
		test(t, &Session{
			Email:     "fred@chromium.org",
			ID:        "12345",
			AuthScope: emailScope,
			Token:     nil,
		})
	})
}

func TestTryLoadingFromGCPSecret_Success(t *testing.T) {

	ctx := context.Background()
	client := &mocks.Client{}
	secretValue := `{
  "salt": "fake-salt",
  "client_id": "fake-client-id",
  "client_secret": "fake-client-secret"
}`
	client.On("Get", ctx, LoginSecretProject, LoginSecretName, secret.VersionLatest).Return(secretValue, nil)
	cookieSalt, clientID, clientSecret, err := TryLoadingFromGCPSecret(ctx, client)
	require.NoError(t, err)
	require.Equal(t, "fake-salt", cookieSalt)
	require.Equal(t, "fake-client-id", clientID)
	require.Equal(t, "fake-client-secret", clientSecret)
}

func TestStateFromPartsAndPartsFromStateRoundTrip_Success(t *testing.T) {
	sessionIDSent := "sessionID"
	redirectURLSent := "https://example.org"
	state := stateFromParts(sessionIDSent, saltForTesting, redirectURLSent)
	sessionID, hash, redirectURL, err := partsFromState(state)
	require.NoError(t, err)
	require.Equal(t, sessionID, sessionIDSent)
	require.Equal(t, redirectURL, redirectURLSent)
	require.Equal(t, hashForURL(saltForTesting, redirectURL), hash)
}

func TestPartsFromState_MissingOnePart_ReturnsError(t *testing.T) {
	sessionIDSent := "sessionID"
	redirectURLSent := "https://example.org"
	state := stateFromParts(sessionIDSent, saltForTesting, redirectURLSent)
	state = strings.Join(strings.Split(state, ".")[1:], ".")
	_, _, _, err := partsFromState(state)
	require.ErrorIs(t, err, errMalformedState)
}

func TestOAuth2CallbackHandler_NoCookieSet_Returns500(t *testing.T) {
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http://example.com/", nil)
	require.NoError(t, err)
	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Missing session state")
}

func setupForOAuth2CallbackHandlerTest(t *testing.T, url string) (*httptest.ResponseRecorder, *http.Request) {
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	cookieSalt = saltForTesting
	secureCookie = securecookie.New([]byte(cookieSalt), nil)

	cookie := &http.Cookie{
		Name:  SESSION_COOKIE_NAME,
		Value: sessionIDForTesting,
	}
	r.AddCookie(cookie)
	setActiveAllowLists("")
	return w, r
}

func TestOAuth2CallbackHandler_CookieSetButStateNotSet_Returns500(t *testing.T) {
	w, r := setupForOAuth2CallbackHandlerTest(t, "https://skia.org/")

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Invalid session state")
}

func TestOAuth2CallbackHandler_CookieSetButSessionIDDoesNotMatchSessionIDInState_Returns500(t *testing.T) {
	state := stateFromParts("wrongSessionID", saltForTesting, "/foo")
	u := fmt.Sprintf("https://skia.org/?state=%s", state)
	w, r := setupForOAuth2CallbackHandlerTest(t, u)

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Session state doesn't match callback state.")
}

func TestOAuth2CallbackHandler_HashOfRedirectURLDoesNotMatch_Returns500(t *testing.T) {
	state := stateFromParts(sessionIDForTesting, "using the wrong salt here will change the hash", "/foo")
	u := fmt.Sprintf("https://skia.org/?state=%s", state)
	w, r := setupForOAuth2CallbackHandlerTest(t, u)

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Invalid redirect URL")
}

func TestOAuth2CallbackHandler_ExchangeReturnsError_Returns500(t *testing.T) {
	state := stateFromParts(sessionIDForTesting, saltForTesting, "/foo")
	u := fmt.Sprintf("https://skia.org/?state=%s&code=%s", state, codeForTesting)
	w, r := setupForOAuth2CallbackHandlerTest(t, u)

	oauthConfigMock := loginMocks.NewOAuthConfig(t)
	oauthConfigMock.On("Exchange", testutils.AnyContext, codeForTesting).Return(nil, errMockError)
	oauthConfig = oauthConfigMock

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Failed to authenticate")
}

func TestOAuth2CallbackHandler_ExtractEmailAndAccountIDFromTokenReturnsError_Returns500(t *testing.T) {
	state := stateFromParts(sessionIDForTesting, saltForTesting, "/foo")
	u := fmt.Sprintf("https://skia.org/?state=%s&code=%s", state, codeForTesting)
	w, r := setupForOAuth2CallbackHandlerTest(t, u)

	oauthConfigMock := loginMocks.NewOAuthConfig(t)
	token := &oauth2.Token{}
	token = token.WithExtra(map[string]string{})
	oauthConfigMock.On("Exchange", testutils.AnyContext, codeForTesting).Return(token, nil)
	oauthConfig = oauthConfigMock

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "No id_token returned")
}

func TestOAuth2CallbackHandler_HappyPath(t *testing.T) {
	state := stateFromParts(sessionIDForTesting, saltForTesting, "/foo")
	u := fmt.Sprintf("https://skia.org/?state=%s&code=%s", state, codeForTesting)
	w, r := setupForOAuth2CallbackHandlerTest(t, u)

	oauthConfigMock := loginMocks.NewOAuthConfig(t)

	middle := base64.URLEncoding.EncodeToString([]byte(`{
		"email": "somebody@example.org",
		"sub": "123"
		}`))
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a." + middle + ".c"})

	oauthConfigMock.On("Exchange", testutils.AnyContext, codeForTesting).Return(tokenWith, nil)
	oauthConfig = oauthConfigMock

	viewAllow = nil

	OAuth2CallbackHandler(w, r)
	require.Contains(t, w.Body.String(), "Found")
	require.Equal(t, w.Result().StatusCode, http.StatusFound)
	require.Equal(t, "/foo", w.Header().Get("Location"))
}

func TestExtractEmailAndAccountIDFromToken_InvalidForm_ReturnsFailureMessage(t *testing.T) {
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a.b"})
	_, _, msg := extractEmailAndAccountIDFromToken(tokenWith)
	require.Contains(t, msg, "Invalid id_token")
}

func TestExtractEmailAndAccountIDFromToken_InvalidBase64_ReturnsFailureMessage(t *testing.T) {
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a.??;;::not-valid-base64.c"})
	_, _, msg := extractEmailAndAccountIDFromToken(tokenWith)
	require.Contains(t, msg, "Failed to base64 decode id_token")
}

func TestExtractEmailAndAccountIDFromToken_DecodedBase64IsNotValidJSON_ReturnsFailureMessage(t *testing.T) {
	middle := base64.URLEncoding.EncodeToString([]byte("{not-valid-json"))
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a." + middle + ".c"})
	_, _, msg := extractEmailAndAccountIDFromToken(tokenWith)
	require.Contains(t, msg, "Failed to JSON decode id_token")
}

func TestExtractEmailAndAccountIDFromToken_EmailIsNotValidJSON_ReturnsFailureMessage(t *testing.T) {
	middle := base64.URLEncoding.EncodeToString([]byte(`{
"email": "not-a-valid-email-address",
"sub": "123"
}`))
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a." + middle + ".c"})
	_, _, msg := extractEmailAndAccountIDFromToken(tokenWith)
	require.Contains(t, msg, "Invalid email address received")
}

func TestExtractEmailAndAccountIDFromToken_HappyPath(t *testing.T) {
	middle := base64.URLEncoding.EncodeToString([]byte(`{
"email": "somebody@example.org",
"sub": "123"
}`))
	token := &oauth2.Token{}
	tokenWith := token.WithExtra(map[string]interface{}{idTokenKeyName: "a." + middle + ".c"})
	email, id, msg := extractEmailAndAccountIDFromToken(tokenWith)
	require.Empty(t, msg)
	require.Equal(t, "somebody@example.org", email)
	require.Equal(t, "123", id)
}
