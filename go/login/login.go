// Package login handles logging in users.
package login

// Theory of operation.
//
// We use OAuth 2.0 handle authentication. We are essentially doing OpenID
// Connect, but vastly simplified since we are hardcoded to Google's endpoints.
//
// We do a simple OAuth 2.0 flow where the user is asked to grant permission to
// the 'email' scope. See https://developers.google.com/+/api/oauth#email for
// details on that scope. Note that you need to have the Google Plus API turned
// on in your project for this to work, but note that the 'email' scope will
// still work for people w/o Google Plus accounts.
//
// Now in theory once we are authorized and have an Access Token we could call
// https://developers.google.com/+/api/openidconnect/getOpenIdConnect and get the
// users email address. But here we can cheat, as we know it's Google and that for
// the 'email' scope an ID Token will be returned along with the Access Token.
// If we decode the ID Token we can get the email address out of that w/o the
// need for the second round trip. This is all clearly *ahem* explained here:
//
//   https://developers.google.com/accounts/docs/OAuth2Login#exchangecode
//
// Once we get the users email address we put it in a cookie for later
// retrieval. The cookie value is validated using HMAC to stop spoofing.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	ttlcache "github.com/patrickmn/go-cache"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2_api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

const (

	// DefaultOAuth2Callback is the default relative OAuth2 redirect URL.
	DefaultOAuth2Callback = "/oauth2callback/"

	// LoginPath is the path to use for login on the root domain.
	LoginPath = "/login/"

	// LogoutPath is the path to use for logout on the root domain.
	LogoutPath = "/logout/"

	// emailScope is the scope we request when logging in.
	emailScope = "email"

	// The name of the cookie that stores the login info.
	cookieName = "sktoken"

	// The name of the session cookie.
	sessionCookieName = "sksession"

	// Default cookie salt used only for testing.
	defaultCookieSalt = "notverysecret"

	// defaultAllowedDomains is a list of domains we use frequently.
	defaultAllowedDomains = "google.com chromium.org skia.org"

	// cookieDomainSkiaCorp is the cookie domain for skia*.corp.goog.
	cookieDomainSkiaCorp = "corp.goog"

	// loginConfigFile is the location of the login config when running in kubernetes.
	loginConfigFile = "/etc/skia.org/login.json"

	// defaultClientSecretFile is the default path to the file used for OAuth2 login.
	defaultClientSecretFile = "client_secret.json"

	// loginSecretProject is the GCP project containing the login secrets.
	loginSecretProject = "skia-infra-public"

	// idTokenKeyName is the key of the JWT stored in oauth2.Token.Extra that
	// contains the authenticated users email address.
	idTokenKeyName = "id_token"

	// validBearerTokenCacheLifetime is how long are valid bearer tokens cached
	// before requiring they be validated again.
	//
	// OAuth2 access tokens expire after an hour, so we'll cache them for the
	// same duration.
	validBearerTokenCacheLifetime = time.Hour

	// validBearerTokenCacheCleanup is how often the cache is cleared of expired
	// bearer tokens.
	validBearerTokenCacheCleanup = 5 * time.Minute
)

var (
	// DefaultRedirectURL is the redirect URL to use if Init is called with
	// DEFAULT_ALLOWED_DOMAINS.
	DefaultRedirectURL = "https://skia.org/oauth2callback/"

	// cookieDomain is the domain to use when setting Cookies.
	cookieDomain = "skia.org"

	// loginSecretName is the name of the GCP secret for login.
	loginSecretName = "login-oauth2-secrets"

	errMalformedState = errors.New("malformed state value")
)

// InitOption are options passed to Init. Note that DomainName implements
// InitOption allowing the selection of the login domain.
type InitOption interface {
	Apply() error
}

// SkipLoadingSecrets should only be used when calling Init during tests. It
// skips trying to load secrets.
type SkipLoadingSecrets struct{}

// Apply implements InitOption.
func (s SkipLoadingSecrets) Apply() error {
	return nil
}

// DomainName represents a domain name that can be used for login.
type DomainName string

// Apply implements InitOption for DomainName selection.
func (d DomainName) Apply() error {
	return setDomain(d)
}

const (
	// SkiaOrg selects the configuration for the skia.org domain.
	SkiaOrg DomainName = "skia.org"

	// LuciApp selects the configuration for the luci.app domain.
	LuciApp DomainName = "luci.app"
)

// AllDomainNames contains all the allowed domain names.
var AllDomainNames = []DomainName{SkiaOrg, LuciApp}

// domainConfig contains the configuration to process logins for a domain.
type domainConfig struct {
	CookieDomain    string
	LoginSecretName string
}

var domainConfigurations = map[DomainName]domainConfig{
	SkiaOrg: {
		CookieDomain:    "skia.org",
		LoginSecretName: "login-oauth2-secrets",
	},
	LuciApp: {
		CookieDomain:    "luci.app",
		LoginSecretName: "luci-app-login-oauth2-secrets",
	},
}

// setDomain sets the domain used for authentication.
func setDomain(d DomainName) error {
	cfg, ok := domainConfigurations[d]
	if !ok {
		return skerr.Fmt("unknown domain: %q", d)
	}
	DefaultRedirectURL = fmt.Sprintf("https://%s%s", cfg.CookieDomain, DefaultOAuth2Callback)
	cookieDomain = cfg.CookieDomain
	loginSecretName = cfg.LoginSecretName
	return nil
}

// OAuthConfigConstructor allows choosing OAuthConfig implementations.
type OAuthConfigConstructor func(clientID, clientSecret, redirectURL string) OAuthConfig

// OAuthConfig is an interface with the subset of the functionality we use of oauth2.Config, used for tests/mocking.
type OAuthConfig interface {
	// See oauth2.Config.
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string

	// See oauth2.Config.
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
}

// oauth2Config implements OAuthConfigConstructor for *oauth2.Config objects.
func configConstructor(clientID, clientSecret, redirectURL string) OAuthConfig {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{emailScope},
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}
}

var (
	// cookieSalt is some entropy for our encoders.
	cookieSalt = ""

	secureCookie *securecookie.SecureCookie = nil

	tokenValidatorService *oauth2_api.Service = nil

	// oauthConfig is the OAuth 2.0 client configuration.
	oauthConfig = configConstructor("not-a-valid-client-id", "not-a-valid-client-secret", "http://localhost:8000/oauth2callback/")

	// loginCtxKey is used to store login information in the request context.
	loginCtxKey = &struct{}{}

	// activeOAuth2ConfigConstructor can be replaced with a func that returns a
	// mock OAuthConfig for testing.
	activeOAuth2ConfigConstructor OAuthConfigConstructor = configConstructor

	// validBearerTokenCache is a TTL cache for bearer tokens that have been
	// validated, which saves an HTTP round trip for validation for every
	// request.
	validBearerTokenCache *ttlcache.Cache
)

// Session is encrypted and serialized and stored in a user's cookie.
type Session struct {
	Email     string
	ID        string
	AuthScope string
	Token     *oauth2.Token
}

// Returns true if SkipLoadingSecrets has been passed in as an option.
func skipLoadingSecrets(opts ...InitOption) bool {
	for _, opt := range opts {
		if _, ok := opt.(SkipLoadingSecrets); ok {
			return true
		}
	}
	return false
}

// Init must be called before any other login methods.
//
// The function first tries to load the cookie salt, client id, and client
// secret from a file provided by Kubernetes secrets. If that fails, it tries to
// load them from GCP secret manager, and if that also fails it looks for a
// "client_secret.json" file in the current directory to extract the client id
// and client secret from. If all three of those fail then it returns an error.
//
// The authAllowList is the space separated list of domains and email addresses
// that are allowed to log in.
//
// InitOptions include setting the DomainName to be used for authentication.
func Init(ctx context.Context, redirectURL, authAllowList string, clientSecretFile string, opts ...InitOption) error {
	for _, opt := range opts {
		if err := opt.Apply(); err != nil {
			return skerr.Wrapf(err, "applying option")
		}
	}

	// Must be done after applying opts, since an opt may change
	// DefaultRedirectURL.
	if redirectURL == "" {
		redirectURL = DefaultRedirectURL
	}

	cookieSalt = defaultCookieSalt
	var clientID string
	var clientSecret string
	var err error
	if !skipLoadingSecrets(opts...) {
		cookieSalt, clientID, clientSecret, err = TryLoadingFromAllSources(ctx, clientSecretFile)
		if err != nil {
			return skerr.Wrap(err)
		}
	}

	secureCookie = securecookie.New([]byte(cookieSalt), nil)
	oauthConfig = activeOAuth2ConfigConstructor(clientID, clientSecret, redirectURL)

	setActiveAllowLists(authAllowList)
	tokenValidatorService, err = oauth2_api.NewService(ctx, option.WithHTTPClient(httputils.NewTimeoutClient()))
	if err != nil {
		return skerr.Wrapf(err, "create oauth2 service client")
	}

	// Create the valid bearer token cache.
	validBearerTokenCache = ttlcache.New(validBearerTokenCacheLifetime, validBearerTokenCacheCleanup)

	// Report metrics on the cache size.
	validBearerTokens := metrics2.GetInt64Metric("login_valid_bearer_tokens_in_cache")
	go func() {
		for range time.Tick(time.Minute) {
			validBearerTokens.Update(int64(validBearerTokenCache.ItemCount()))
		}
	}()

	return nil
}

func writeNewSessionCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	sessionID, err := generateID()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Domain:   domainFromHost(r.Host),
		HttpOnly: true,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	http.SetCookie(w, cookie)
	return sessionID, nil
}

// LoginURL returns a URL that the user is to be directed to for login.
func LoginURL(w http.ResponseWriter, r *http.Request) string {
	// Check for a session id, if not there then assign one, and add it to the redirect URL.
	session, err := r.Cookie(sessionCookieName)
	sessionID := ""
	if err != nil || session.Value == "" {
		sessionID, err = writeNewSessionCookie(w, r)
		if err != nil {
			sklog.Errorf("Failed to create a session token: %s", err)
			return ""
		}
	} else {
		sessionID = session.Value
	}

	redirect := r.Referer()
	if redirect == "" {
		// If we don't have a referrer then we need to construct the URL to
		// bounce back to. This only works if r.Host is set correctly, which
		// it should be as long as 'proxy_set_header Host $host;' is set for
		// the nginx server config.
		redirect = "https://" + r.Host + r.RequestURI
	}
	// Append the current URL to the state, in a way that's safe from tampering,
	// so that we can use it on the rebound. So the state we pass in has the
	// form:
	//
	//	<sessionid>:<hash(salt + original url)>:<original url>
	//
	// Note that the sessionid and the hash are hex values and so won't contain
	// any colons.  To break this up when returned from the server just use
	// strings.SplitN(s, ":", 3) which will ignore any colons found in the
	// Referral URL.
	//
	// On the receiving side we need to recompute the hash and compare against
	// the hash passed in, and only if they match should the redirect URL be
	// trusted.
	state := stateFromParts(sessionID, cookieSalt, redirect)

	// Only retrieve an online access token, i.e. no refresh token. And when we
	// go through the approval flow again don't stop if they've already approved
	// once, unless they have a valid token but aren't in the allow list, in
	// which case we want to use ApprovalForce so they get the chance to pick a
	// different account to log in with.
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline}
	s, err := getSession(r)
	if err == nil && !isAuthorized(s.Email) {
		opts = append(opts, oauth2.ApprovalForce)
	} else {
		opts = append(opts, oauth2.SetAuthURLParam("approval_prompt", "auto"))
	}
	return oauthConfig.AuthCodeURL(state, opts...)
}

// stateFromParts constructs a state value. The state value has the form:
//
//	<sessionid>:<hash(salt + original url)>:<original url>
//
// Note that the sessionid and the hash are hex values and so won't contain
// any colons.  To break this up when returned from the server just use
// strings.SplitN(s, ":", 3) which will ignore any colons found in the
// Referral URL.
//
// On the receiving side we need to recompute the hash and compare against
// the hash passed in, and only if they match should the redirect URL be
// trusted.
func stateFromParts(sessionsID, salt, redirect string) string {
	return fmt.Sprintf("%s:%s:%s", sessionsID, hashForURL(salt, redirect), redirect)
}

// partsFromState breaks up the state, which has the form:
//
//	<sessionid>:<hash(salt + original url)>:<original url>
//
// and returns each part, or an error if the number of parts is wrong.
func partsFromState(state string) (string, string, string, error) {
	stateParts := strings.SplitN(state, ":", 3)
	if len(stateParts) == 3 {
		return stateParts[0], stateParts[1], stateParts[2], nil
	}
	return "", "", "", errMalformedState
}

// hashForURL computes hash(salt+url) and returns it as a hex string.
func hashForURL(salt, url string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(salt+url)))
}

// generate a 16-byte random ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", b), nil
}

func getSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var s Session
	if err := secureCookie.Decode(cookieName, cookie.Value, &s); err != nil {
		return nil, skerr.Wrap(err)
	}
	if s.AuthScope != emailScope {
		return nil, skerr.Fmt("Stored auth scope differs from expected (%q vs %q)", emailScope, s.AuthScope)
	}
	return &s, nil
}

// LoggedInAs returns the user's email address, if they are logged in, and "" if
// they are not logged in.
func LoggedInAs(r *http.Request) string {
	var email string
	if s, err := getSession(r); err == nil {
		email = s.Email
	} else {
		if e, err := ViaBearerToken(r); err == nil {
			email = e
		}
	}
	if isAuthorized(email) {
		return email
	}

	return ""
}

// LoggedInAsFromContext returns the email from the session cookie if it is
// present and matches either the domain or the user allow list. The passed-in
// Context must be from a request whose http.Handler was wrapped using
// SessionMiddleware. This differs from LoggedInAs in that it doesn't fall back
// to checking the OAuth 2 Bearer token.
func LoggedInAsFromContext(ctx context.Context) string {
	if session := getSessionFromContext(ctx); session != nil {
		email := session.Email
		if isAuthorized(email) {
			return email
		}
	}
	return ""
}

// userIdentifiers returns both the email and opaque user id of the logged in
// user, and will return two empty strings if they are not logged in.
func userIdentifiers(r *http.Request) (string, string) {
	s, err := getSession(r)
	if err != nil {
		return "", ""
	}
	return s.Email, s.ID
}

// A JSON Web Token can contain much info, such as 'iss'. We don't care about
// that, we only want two fields, 'email' and 'sub'.
//
//	{
//	  "iss":"accounts.google.com",
//	  "sub":"110642259984599645813",
//	  "email":"jcgregorio@google.com",
//	  ...
//	}
type decodedIDToken struct {
	Email string `json:"email"`
	ID    string `json:"sub"`
}

// domainFromHost returns the value to use in the cookie Domain field based on
// the requests Host value.
func domainFromHost(fullhost string) string {
	// Split host and port.
	parts := strings.Split(fullhost, ":")
	host := parts[0]
	if strings.HasPrefix(fullhost, "localhost") {
		return host
	} else if strings.HasSuffix(fullhost, "."+cookieDomainSkiaCorp) {
		return cookieDomainSkiaCorp
	} else if strings.HasSuffix(fullhost, "."+cookieDomain) || fullhost == cookieDomain {
		return cookieDomain
	} else {
		sklog.Errorf("Unknown domain for host: %s; falling back to %s", fullhost, cookieDomain)
		return cookieDomain
	}
}

// cookieFor creates an encoded Cookie for the given user id.
func cookieFor(value *Session, r *http.Request) (*http.Cookie, error) {
	encoded, err := secureCookie.Encode(cookieName, value)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode cookie")
	}
	return &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		Domain:   domainFromHost(r.Host),
		HttpOnly: true,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}, nil
}

func setSkIDCookieValue(w http.ResponseWriter, r *http.Request, value *Session) {
	cookie, err := cookieFor(value, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s", err), 500)
		return
	}
	http.SetCookie(w, cookie)
}

// LogoutHandler logs the user out by overwriting the cookie with a blank email
// address.
//
// Note that this doesn't revoke the 'email' grant, so logging in later will
// still be fast. Users can always visit
//
//	https://security.google.com/settings/security/permissions
//
// to revoke any grants they make.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("LogoutHandler")
	setSkIDCookieValue(w, r, &Session{})
	redirect := r.FormValue("redirect")
	// The empty string for the redirect will just redirect back to the
	// LogoutHandler in an infinite loop, so fallback to "/".
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// LoginHandler kicks off the authentication flow.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("LoginHandler")
	http.Redirect(w, r, LoginURL(w, r), http.StatusFound)
}

// OAuth2CallbackHandler must be attached at a handler that matches
// the callback URL registered in the APIs Console. In this case
// "/oauth2callback".
func OAuth2CallbackHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Missing session state.", 500)
		return
	}

	// Validate the session state.
	sessionID, hash, redirectURL, err := partsFromState(r.FormValue("state"))
	if err != nil {
		http.Error(w, "Invalid session state", 500)
		return
	}
	if sessionID != cookie.Value {
		http.Error(w, "Session state doesn't match callback state.", 500)
		return
	}
	expectedHash := hashForURL(cookieSalt, redirectURL)
	if hash != expectedHash {
		sklog.Errorf("Got an invalid redirect: %s != %s", hash, expectedHash)
		http.Error(w, "Invalid redirect URL", 500)
		return
	}

	// Exchange code for JWT.
	code := r.FormValue("code")
	token, err := oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		sklog.Errorf("Failed to authenticate: %s", err)
		http.Error(w, "Failed to authenticate.", 500)
		return
	}

	// Extract email address and account ID from token.
	email, accountID, errorMessage := extractEmailAndAccountIDFromToken(token)
	if errorMessage != "" {
		http.Error(w, errorMessage, 500)
		return
	}

	if !isAuthorized(email) {
		http.Error(w, "Accounts from your domain are not allowed or your email address is not on the allow list.", 500)
		return
	}
	s := Session{
		Email:     email,
		ID:        accountID,
		AuthScope: emailScope,
		Token:     token,
	}
	setSkIDCookieValue(w, r, &s)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Returns only an error message instead of an error because the results are
// sent back to the caller via http.Error() and we don't want to accidentally
// leak internal data that an an error type might accumulate.
func extractEmailAndAccountIDFromToken(token *oauth2.Token) (string, string, string) {
	// idToken is a JSON Web Token. We only need to decode the token, we do not
	// need to validate the token because it came to us over HTTPS directly from
	// Google's servers.
	idToken, ok := token.Extra(idTokenKeyName).(string)
	if !ok {
		return "", "", "No id_token returned."
	}
	// The id token is actually three base64 encoded parts that are "." separated.
	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		return "", "", "Invalid id_token."
	}
	// Now base64 decode the middle segment, which decodes to JSON.
	padding := 4 - (len(segments[1]) % 4)
	if padding == 4 {
		padding = 0
	}
	middle := segments[1] + strings.Repeat("=", padding)
	b, err := base64.URLEncoding.DecodeString(middle)
	if err != nil {
		sklog.Errorf("Failed to base64 decode middle part of token: %s From: %#v", middle, segments)
		return "", "", "Failed to base64 decode id_token."
	}
	// Finally decode the JSON.
	decoded := &decodedIDToken{}
	if err := json.Unmarshal(b, decoded); err != nil {
		sklog.Errorf("Failed to JSON decode token: %s", string(b))
		return "", "", "Failed to JSON decode id_token."
	}

	email := strings.ToLower(decoded.Email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", "", "Invalid email address received."
	}
	return email, decoded.ID, ""
}

// isAuthorized returns true if the given email address matches either the
// domain or the user allow list.
func isAuthorized(email string) bool {
	if email == "" {
		return false
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		sklog.Errorf("Email %q was not in 2 parts", email)
		return false
	}

	user, domain := parts[0], parts[1]
	if domain == "gmail.com" {
		user = normalizeGmailAddress(user)
	}
	normalizedEmail := user + "@" + domain

	if viewAllow != nil {
		sklog.Infof("viewAllow = %v", viewAllow)
		return viewAllow.Member(normalizedEmail)
	}

	if len(activeUserDomainAllowList) == 0 {
		return true // if the list is empty, everybody is allowed
	}
	return activeUserDomainAllowList[domain] || activeUserEmailAllowList[normalizedEmail]
}

// normalizeGmailAddress removes periods and text after a plus sign.
// See https://stackoverflow.com/a/15499627 for more.
func normalizeGmailAddress(user string) string {
	user = strings.ReplaceAll(user, ".", "")
	plusIdx := strings.Index(user, "+")
	if plusIdx >= 0 {
		return user[:plusIdx]
	}
	return user
}

// LoginStatus is the status returned by the StatusHandler endpoint, serialized
// as JSON.
type LoginStatus struct {
	Email      string `json:"Email"`
	ID         string `json:"ID"`
	LoginURL   string `json:"LoginURL"`
	IsAGoogler bool   `json:"IsAGoogler"`
	IsAdmin    bool   `json:"IsAdmin"`
	IsEditor   bool   `json:"IsEditor"`
	IsViewer   bool   `json:"IsViewer"`
}

// StatusHandler returns the login status of the user as JSON that looks like:
//
//	{
//	  "Email":     "fred@example.com",
//	  "ID":        "12342...34324",
//	  "LoginURL":  "https://..."
//	  "IsAGoogler": false,
//	  "IsViewer":   true,
//	  "IsEditor":   true,
//	  "IsAdmin:     false
//	}
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("StatusHandler")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	email, id := userIdentifiers(r)
	body := LoginStatus{
		Email:      email,
		ID:         id,
		LoginURL:   LoginURL(w, r),
		IsAGoogler: IsGoogler(r),
		IsAdmin:    IsAdmin(r),
		IsEditor:   IsEditor(r),
		IsViewer:   IsViewer(r),
	}

	sklog.Infof("Origin: %s", r.Header.Get("Origin"))
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			httputils.ReportError(w, err, "Invalid Origin", http.StatusInternalServerError)
			return
		}
		if strings.HasSuffix(u.Host, "."+cookieDomain) ||
			strings.HasSuffix(u.Host, "."+cookieDomainSkiaCorp) ||
			strings.HasPrefix(u.Host, "localhost:") {
			prefix := "https://"
			if strings.HasPrefix(u.Host, "localhost:") {
				prefix = "http://"
			}
			w.Header().Add("Access-Control-Allow-Origin", prefix+u.Host)
			w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Add("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}
	}

	if r.Method == "OPTIONS" {
		return
	}

	if err := enc.Encode(body); err != nil {
		sklog.Errorf("Failed to encode Login status to JSON: %s", err)
	}
}

// ForceAuth is middleware that enforces authentication
// before the wrapped handler is called. oauthCallbackPath is the
// URL path that the user is redirected to at the end of the auth flow.
func ForceAuth(h http.Handler, oauthCallbackPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := LoggedInAs(r)
		if userID == "" {
			if strings.HasPrefix(r.URL.Path, oauthCallbackPath) {
				// If this is the oauth2 callback, run that handler.
				OAuth2CallbackHandler(w, r)
				return
			} else {
				// If this is not the oauth callback then redirect.
				redirectURL := LoginURL(w, r)
				sklog.Infof("Redirect URL: %s", redirectURL)
				if redirectURL == "" {
					httputils.ReportError(w, fmt.Errorf("unable to get redirect URL"), "Redirect to login failed:", http.StatusInternalServerError)
					return
				}
				http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// loginInfo is the JSON file format that client info is stored in as a kubernetes secret.
type loginInfo struct {
	Salt         string `json:"salt"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// TryLoadingFromAllSources tries to load the cookie salt, client id, and client
// secret from GCP secrets, and a local file.  Returns an error if all of the
// above fail.
//
// Returns salt, clientID, clientSecret.
func TryLoadingFromAllSources(ctx context.Context, clientSecretFile string) (string, string, string, error) {
	// GCP secret.
	secretClient, err1 := secret.NewClient(ctx)
	if err1 == nil {
		cookieSalt, clientID, clientSecret, err2 := TryLoadingFromGCPSecret(ctx, secretClient)
		if err2 == nil {
			return cookieSalt, clientID, clientSecret, nil
		}
	} else {
		err1 = skerr.Wrapf(err1, "failed loading login secrets from GCP secret manager; failed to create client")
	}

	// Local file, this is only used for testing.
	cookieSalt, clientID, clientSecret, err2 := TryLoadingFromFile(clientSecretFile)
	if err2 == nil {
		return cookieSalt, clientID, clientSecret, nil
	}
	return "", "", "", skerr.Fmt("Failed loading from metadata, GCP secrets, and from %s: %s | %s", clientSecretFile, err1, err2)
}

// TryLoadingFromFile tries to load the client id and client secret from the
// given file.  If not specified, it tries to load from client_secret.json in
// the current working directory.
//
// Returns DEFAULT_COOKIE_SALT, clientID, clientSecret.
func TryLoadingFromFile(clientSecretFile string) (string, string, string, error) {
	if clientSecretFile == "" {
		clientSecretFile = defaultClientSecretFile
	}
	b, err := ioutil.ReadFile(clientSecretFile)
	if err != nil {
		return "", "", "", skerr.Wrapf(err, "failed loading login secrets from %s", clientSecretFile)
	}
	config, err := google.ConfigFromJSON(b)
	if err != nil {
		return "", "", "", skerr.Wrapf(err, "failed decoding login secrets from %s", clientSecretFile)
	}
	sklog.Infof("Successfully read client secret from %s", clientSecretFile)
	return defaultCookieSalt, config.ClientID, config.ClientSecret, nil
}

// TryLoadingFromGCPSecret tries to load the cookie salt, client id, and client
// secret from GCP secrets.  If it fails, it returns the default cookie salt and
// the client id and secret are the empty string.
//
// Returns salt, clientID, clientSecret.
func TryLoadingFromGCPSecret(ctx context.Context, secretClient secret.Client) (string, string, string, error) {
	contents, err := secretClient.Get(ctx, loginSecretProject, loginSecretName, secret.VersionLatest)
	if err != nil {
		return "", "", "", skerr.Wrapf(err, "failed loading login secrets from GCP secret manager; failed to retrieve secret %q", loginSecretName)
	}
	var info loginInfo
	if err := json.Unmarshal([]byte(contents), &info); err != nil {
		return "", "", "", skerr.Wrapf(err, "successfully retrieved login secret from GCP secrets but failed to decode it as JSON")
	}
	return info.Salt, info.ClientID, info.ClientSecret, nil
}

// ViaBearerToken tries to load an OAuth 2.0 Bearer token from the request and
// derives the login email address from it.
func ViaBearerToken(r *http.Request) (string, error) {
	tok := r.Header.Get("Authorization")
	if tok == "" {
		return "", skerr.Fmt("User is not authenticated. No Authorization header.")
	}
	tok = strings.TrimPrefix(tok, "Bearer ")
	tokenInfo, err := validateBearerToken(r.Context(), tok)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return tokenInfo.Email, nil
}

// validateBearerToken takes an OAuth 2.0 Bearer token (e.g. The third part of
// `Authorization: Bearer <value>“) and polls a Google HTTP endpoint to see if
// is valid. Valid tokens are cached for one hour.
func validateBearerToken(ctx context.Context, token string) (*oauth2_api.Tokeninfo, error) {
	iTokenInfo, ok := validBearerTokenCache.Get(token)
	if ok {
		return iTokenInfo.(*oauth2_api.Tokeninfo), nil
	}

	ti, err := tokenValidatorService.Tokeninfo().AccessToken(token).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	if ti.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token is expired")
	}
	if !ti.VerifiedEmail {
		return nil, fmt.Errorf("email not verified")
	}
	validBearerTokenCache.Set(token, ti, ttlcache.DefaultExpiration)

	return ti, nil
}

// SessionMiddleware is middleware which attaches login info to the request
// context. This allows the passed-in handler to use GetSession().
func SessionMiddleware(sub http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ignore the error. If it matters that the user is not logged in, the
		// request will fail at some other level.
		session, _ := getSession(r)
		ctx := context.WithValue(r.Context(), loginCtxKey, session)
		sub.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getSessionFromContext returns the current user's Session, or nil if the user is not
// logged in. The passed-in Context must be from a request whose http.Handler
// was wrapped using SessionMiddleware.
func getSessionFromContext(ctx context.Context) *Session {
	session := ctx.Value(loginCtxKey)
	if session != nil {
		return session.(*Session)
	}
	return nil
}

// FakeLoggedInAs is to be used by unit tests which want to fake that a user is logged in.
func FakeLoggedInAs(ctx context.Context, userEmail string) context.Context {
	s := Session{
		Email: userEmail,
	}
	return context.WithValue(ctx, loginCtxKey, &s)
}
