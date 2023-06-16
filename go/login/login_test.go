package login

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/securecookie"
	ttlcache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	loginMocks "go.skia.org/infra/go/login/mocks"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/secret/mocks"
	"go.skia.org/infra/go/testutils"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	oauth2_api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (
	saltForTesting = "salt"

	sessionIDForTesting = "abcdef0123456"

	codeForTesting = "oauth2 code for testing"

	bearerToken = "fake-bearer-token"
)

var (
	errMockError = fmt.Errorf("error returned from mocks")
)

func initLoginForTests(t *testing.T) {
	ctx := context.Background()
	err := initLogin(ctx, "id", "secret", "http://localhost", saltForTesting, SkiaOrg)
	require.NoError(t, err)
}

func TestLoginURL(t *testing.T) {
	initLoginForTests(t)
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "http://example.com/", nil)
	require.NoError(t, err)
	r.Header.Set("Referer", "https://foo.org")

	url := LoginURL(w, r)
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), sessionCookieName, "Session cookie should be set.")
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), "SameSite=None", "SameSite should be set.")
	assert.Contains(t, w.HeaderMap.Get("Set-Cookie"), "Secure", "Secure should be set.")

	assert.Contains(t, url, "approval_prompt=auto", "Not forced into prompt.")
	cookie := &http.Cookie{
		Name:  sessionCookieName,
		Value: "some-random-state",
	}
	assert.Contains(t, url, "%3Ahttps%3A%2F%2Ffoo.org")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	url = LoginURL(w, r)
	assert.NotContains(t, w.HeaderMap.Get("Set-Cookie"), sessionCookieName, "Session cookie should be set.")
	assert.Contains(t, url, "some-random-state", "Pass state in Login URL.")
}

func TestLoggedInAs(t *testing.T) {
	initLoginForTests(t)
	for _, d := range AllDomainNames {
		t.Run(string(d), func(t *testing.T) {
			testLoggedInAs(t, d)
		})
	}
}

func testLoggedInAs(t *testing.T, domain DomainName) {
	err := setDomain(domain)
	require.NoError(t, err)

	r, err := http.NewRequest("GET", fmt.Sprintf("http://www.%s/", domain), nil)
	require.NoError(t, err)

	assert.Equal(t, LoggedInAs(r), "", "No skid cookie means not logged in.")

	s := Session{
		Email:     "fred@chromium.org",
		ID:        "12345",
		AuthScope: emailScope,
		Token:     nil,
	}
	cookie, err := cookieFor(&s, r)
	assert.NoError(t, err)
	assert.Equal(t, string(domain), cookie.Domain)
	r.AddCookie(cookie)
	assert.Equal(t, LoggedInAs(r), "fred@chromium.org", "Correctly get logged in email.")
	w := httptest.NewRecorder()
	url := LoginURL(w, r)
	assert.Contains(t, url, "approval_prompt=auto", "Not forced into prompt.")
}

func TestDomainFromHost(t *testing.T) {
	initLoginForTests(t)
	assert.Equal(t, "localhost", domainFromHost("localhost:10110"))
	assert.Equal(t, "localhost", domainFromHost("localhost"))
	assert.Equal(t, "skia.org", domainFromHost("skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org"))
	assert.Equal(t, "skia.org", domainFromHost("perf.skia.org:443"))
	assert.Equal(t, "skia.org", domainFromHost("example.com:443"))
}

func TestDomainFromHost_LuciApp(t *testing.T) {
	err := initLogin(context.Background(), "id", "secret", "", saltForTesting, LuciApp)
	require.NoError(t, err)
	assert.Equal(t, "localhost", domainFromHost("localhost:10110"))
	assert.Equal(t, "localhost", domainFromHost("localhost"))
	assert.Equal(t, "luci.app", domainFromHost("luci.app"))
	assert.Equal(t, "luci.app", domainFromHost("perf.luci.app"))
	assert.Equal(t, "luci.app", domainFromHost("perf.luci.app:443"))
	assert.Equal(t, "luci.app", domainFromHost("example.com:443"))
	assert.Equal(t, "https://luci.app/oauth2callback/", GetDefaultRedirectURL())
	assert.Equal(t, "https://luci.app/oauth2callback/",
		oauthConfig.(*oauth2.Config).RedirectURL)
}

func TestIsAuthorized(t *testing.T) {
	initLoginForTests(t)

	assert.True(t, isAuthorized("fred@chromium.org"))
	assert.True(t, isAuthorized("service-account@proj.iam.gserviceaccount.com"))
	assert.False(t, isAuthorized("this is not an email"))
}

func TestTryLoadingFromGCPSecret_Success(t *testing.T) {
	ctx := context.Background()
	client := &mocks.Client{}
	secretValue := `{
  "salt": "fake-salt",
  "client_id": "fake-client-id",
  "client_secret": "fake-client-secret"
}`
	client.On("Get", ctx, loginSecretProject, loginSecretName, secret.VersionLatest).Return(secretValue, nil)
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
		Name:  sessionCookieName,
		Value: sessionIDForTesting,
	}
	r.AddCookie(cookie)
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

func TestSetDomain_ValidDomainName_Success(t *testing.T) {
	for _, d := range AllDomainNames {
		t.Run(string(d), func(t *testing.T) {
			require.NoError(t, setDomain(d))
		})
	}
}

func TestSetDomain_UnknonwDomainName_ReturnsError(t *testing.T) {
	require.Error(t, setDomain(DomainName("this-in-not-a-known-domain.example.com")))
}

func setupForValidateBearerToken(t *testing.T, tokenInfo *oauth2_api.Tokeninfo) {
	// Create an HTTP server that emulates the Token Validation endpoint, that
	// takes in an access token and returns a Tokeninfo.
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, bearerToken, r.FormValue("access_token"))
		require.NoError(t, json.NewEncoder(w).Encode(tokenInfo))
	}))

	// Replace the default tokenValidatorService with one that points to the
	// emulation service built above.
	var err error
	tokenValidatorService, err = oauth2_api.NewService(context.Background(), option.WithHTTPClient(testServer.Client()),
		option.WithEndpoint(testServer.URL))

	// Create a fresh cache.
	validBearerTokenCache = ttlcache.New(validBearerTokenCacheLifetime, validBearerTokenCacheCleanup)
	require.NoError(t, err)
}

func TestValidateBearerToken_HappyPath(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     3600, // seconds
		VerifiedEmail: true,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	actual, err := validateBearerToken(context.Background(), bearerToken)
	require.NoError(t, err)

	// The TokenInfo should be identical modulo the ServerResponse.
	actual.ServerResponse = googleapi.ServerResponse{}
	assertdeep.Equal(t, expectedTokenInfo, actual)
}

func TestValidateBearerToken_ValidatedTokenExistsInCache_Success(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     3600, // seconds
		VerifiedEmail: true,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	// Add token to cache.
	validBearerTokenCache.Set(bearerToken, expectedTokenInfo, ttlcache.DefaultExpiration)

	// Nil out the tokenValidatorService, to prove we don't call it.
	tokenValidatorService = nil

	actual, err := validateBearerToken(context.Background(), bearerToken)
	require.NoError(t, err)
	assertdeep.Equal(t, expectedTokenInfo, actual)
}

func TestValidateBearerToken_FirstRequestAddsTokenToCache_SecondCallReturnsTokenFromCache(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     3600, // seconds
		VerifiedEmail: true,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	actual, err := validateBearerToken(context.Background(), bearerToken)
	require.NoError(t, err)

	// The TokenInfo should be identical modulo the ServerResponse.
	actual.ServerResponse = googleapi.ServerResponse{}
	assertdeep.Equal(t, expectedTokenInfo, actual)

	// Nil out the tokenValidatorService, to prove we don't call it.
	tokenValidatorService = nil

	// Call validateBearerToken again with the same bearer token.
	actual, err = validateBearerToken(context.Background(), bearerToken)
	require.NoError(t, err)
	assertdeep.Equal(t, expectedTokenInfo, actual)
}

func TestValidateBearerToken_EmailNotValidated_ReturnsError(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     3600, // seconds
		VerifiedEmail: false,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	_, err := validateBearerToken(context.Background(), bearerToken)
	require.Contains(t, err.Error(), "email not verified")
}

func TestValidateBearerToken_TokenExpired_ReturnsError(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     0, // seconds
		VerifiedEmail: true,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	_, err := validateBearerToken(context.Background(), bearerToken)
	require.Contains(t, err.Error(), "token is expired")
}

func TestViaBearerToken_HappyPath(t *testing.T) {
	expectedTokenInfo := &oauth2_api.Tokeninfo{
		Email:         "user@example.org",
		ExpiresIn:     3600, // seconds
		VerifiedEmail: true,
	}

	setupForValidateBearerToken(t, expectedTokenInfo)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))

	email, err := ViaBearerToken(r)
	require.NoError(t, err)
	require.Equal(t, "user@example.org", email)
}
