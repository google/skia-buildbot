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
//
// N.B. The cookiesaltkey metadata value must be set on the GCE instance.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2_api "google.golang.org/api/oauth2/v2"
)

const (
	COOKIE_NAME         = "sktoken"
	SESSION_COOKIE_NAME = "sksession"
	DEFAULT_COOKIE_SALT = "notverysecret"

	// DEFAULT_REDIRECT_URL is the redirect URL to use if Init is called with
	// DEFAULT_ALLOWED_DOMAINS.
	DEFAULT_REDIRECT_URL = "https://skia.org/oauth2callback/"

	// DEFAULT_OAUTH2_CALLBACK is the default relative OAuth2 redirect URL.
	DEFAULT_OAUTH2_CALLBACK = "/oauth2callback/"

	// DEFAULT_ALLOWED_DOMAINS is a list of domains we use frequently.
	DEFAULT_ALLOWED_DOMAINS = "google.com chromium.org skia.org"

	// DEFAULT_ADMIN_LIST is list of users we consider to be admins as a
	// fallback when we can't retrieve the list from metadata.
	DEFAULT_ADMIN_LIST = "borenet@google.com jcgregorio@google.com kjlubick@google.com lovisolo@google.com rmistry@google.com westont@google.com"

	// COOKIE_DOMAIN_SKIA_ORG is the cookie domain for skia.org.
	COOKIE_DOMAIN_SKIA_ORG = "skia.org"

	// COOKIE_DOMAIN_SKIA_CORP is the cookie domain for skia*.corp.goog.
	COOKIE_DOMAIN_SKIA_CORP = "corp.goog"

	// LOGIN_CONFIG_FILE is the location of the login config when running in kubernetes.
	LOGIN_CONFIG_FILE = "/etc/skia.org/login.json"

	// DEFAULT_CLIENT_SECRET_FILE is the default path to the file used for OAuth2 login.
	DEFAULT_CLIENT_SECRET_FILE = "client_secret.json"
)

var (
	// cookieSalt is some entropy for our encoders.
	cookieSalt = ""

	secureCookie *securecookie.SecureCookie = nil

	// oauthConfig is the OAuth 2.0 client configuration.
	oauthConfig = &oauth2.Config{
		ClientID:     "not-a-valid-client-id",
		ClientSecret: "not-a-valid-client-secret",
		Scopes:       DEFAULT_SCOPE,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8000/oauth2callback/",
	}

	// activeUserDomainAllowList is the list of domains that are allowed to
	// log in.
	activeUserDomainAllowList map[string]bool

	// activeUserEmailAllowList is the list of email addresses that are
	// allowed to log in (even if the domain is not explicitly allowed).
	activeUserEmailAllowList map[string]bool

	// activeAdminEmailAllowList is the list of email addresses that are
	// allowed to perform admin tasks.
	activeAdminEmailAllowList map[string]bool

	// DEFAULT_SCOPE is the scope we request when logging in.
	DEFAULT_SCOPE = []string{"email"}

	// Auth groups which determine whether a given user has particular types
	// of access. If nil, fall back on domain and individual email allow lists.
	adminAllow allowed.Allow
	editAllow  allowed.Allow
	viewAllow  allowed.Allow
)

// Session is encrypted and serialized and stored in a user's cookie.
type Session struct {
	Email     string
	ID        string
	AuthScope string
	Token     *oauth2.Token
}

// SimpleInitMust initializes the login system for the default case, which uses
// DEFAULT_REDIRECT_URL in prod along with the DEFAULT_ALLOWED_DOMAINS and uses
// a localhost'port' redirect URL if 'local' is true.
//
// If an error occurs then the function fails fatally.
func SimpleInitMust(port string, local bool) {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = DEFAULT_REDIRECT_URL
	}
	if err := Init(redirectURL, DEFAULT_ALLOWED_DOMAINS, ""); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}
}

// SimpleInitWithAllow initializes the login system for the default case (see
// docs for SimpleInitMust) and sets the admin, editor, and viewer lists. These
// may be nil, in which case we fall back on the default settings. For editors
// we default to denying access to everyone, and for viewers we default to
// allowing access to everyone.
func SimpleInitWithAllow(port string, local bool, admin, edit, view allowed.Allow) {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = DEFAULT_REDIRECT_URL
	}
	InitWithAllow(redirectURL, admin, edit, view)
}

// InitWithAllow initializes the login system with the given redirect URL. Sets
// the admin, editor, and viewer lists as provided. These may be nil, in which
// case we fall back on the default settings. For editors we default to denying
// access to everyone, and for viewers we default to allowing access to
// everyone.
func InitWithAllow(redirectURL string, admin, edit, view allowed.Allow) {
	adminAllow = admin
	editAllow = edit
	viewAllow = view
	if err := Init(redirectURL, DEFAULT_ALLOWED_DOMAINS, ""); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}
	RestrictAdmin = RestrictWithMessage(adminAllow, "User is not an admin")
	RestrictEditor = RestrictWithMessage(editAllow, "User is not an editor")
	RestrictViewer = RestrictWithMessage(viewAllow, "User is not a viewer")
}

// Init must be called before any other login methods.
//
// The function first tries to load the cookie salt, client id, and client
// secret from GCE project level metadata. If that fails it looks for a
// "client_secret.json" file in the current directory to extract the client id
// and client secret from. If both of those fail then it returns an error.
//
// The authAllowList is the space separated list of domains and email addresses
// that are allowed to log in.
func Init(redirectURL string, authAllowList string, clientSecretFile string) error {
	cookieSalt, clientID, clientSecret := tryLoadingFromKnownLocations()
	if clientID == "" {
		if clientSecretFile == "" {
			clientSecretFile = DEFAULT_CLIENT_SECRET_FILE
		}

		b, err := ioutil.ReadFile(clientSecretFile)
		if err != nil {
			return skerr.Fmt("Failed to read from metadata and from %s. Got error: %s", clientSecretFile, err)
		}
		config, err := google.ConfigFromJSON(b)
		if err != nil {
			return skerr.Fmt("Failed to read from metadata and decode %s. Got error: %s", clientSecretFile, err)
		}
		sklog.Infof("Successfully read client secret from %s", clientSecretFile)
		clientID = config.ClientID
		clientSecret = config.ClientSecret
	}
	initLogin(clientID, clientSecret, redirectURL, cookieSalt, DEFAULT_SCOPE, authAllowList)
	return nil
}

// initLogin sets the params.  It should only be called directly for testing purposes.
// Clients should use Init().
func initLogin(clientID, clientSecret, redirectURL, cookieSalt string, scopes []string, authAllowList string) {
	secureCookie = securecookie.New([]byte(cookieSalt), nil)
	oauthConfig.ClientID = clientID
	oauthConfig.ClientSecret = clientSecret
	oauthConfig.RedirectURL = redirectURL
	oauthConfig.Scopes = scopes

	setActiveAllowLists(authAllowList)
}

// LoginURL returns a URL that the user is to be directed to for login.
func LoginURL(w http.ResponseWriter, r *http.Request) string {
	// Check for a session id, if not there then assign one, and add it to the redirect URL.
	session, err := r.Cookie(SESSION_COOKIE_NAME)
	state := ""
	if err != nil || session.Value == "" {
		state, err = generateID()
		if err != nil {
			sklog.Errorf("Failed to create a session token: %s", err)
			return ""
		}
		cookie := &http.Cookie{
			Name:     SESSION_COOKIE_NAME,
			Value:    state,
			Path:     "/",
			Domain:   domainFromHost(r.Host),
			HttpOnly: true,
			Expires:  time.Now().Add(365 * 24 * time.Hour),
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}
		http.SetCookie(w, cookie)
	} else {
		state = session.Value
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
	//   <sessionid>:<hash(salt + original url)>:<original url>
	//
	// Note that the sessionid and the hash are hex values and so won't contain
	// any colons.  To break this up when returned from the server just use
	// strings.SplitN(s, ":", 3) which will ignore any colons found in the
	// Referral URL.
	//
	// On the receiving side we need to recompute the hash and compare against
	// the hash passed in, and only if they match should the redirect URL be
	// trusted.
	state = fmt.Sprintf("%s:%x:%s", state, sha256.Sum256([]byte(cookieSalt+redirect)), redirect)

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
	cookie, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		return nil, err
	}
	var s Session
	if cookie != nil && len(cookie.String()) > 20 {
		sklog.Infof("Cookie is: %s", cookie.String()[:20])
	} else {
		// This is likely nil or invalid, so no need to elide.
		sklog.Infof("Cookie is: %v", cookie)
	}

	if err := secureCookie.Decode(COOKIE_NAME, cookie.Value, &s); err != nil {
		return nil, err
	}
	if s.AuthScope != strings.Join(oauthConfig.Scopes, " ") {
		return nil, fmt.Errorf("Stored auth scope differs from expected (%v vs %s)", oauthConfig.Scopes, s.AuthScope)
	}
	return &s, nil
}

// LoggedInAs returns the user's ID, i.e. their email address, if they are
// logged in, and "" if they are not logged in.
func LoggedInAs(r *http.Request) string {
	var email string
	if s, err := getSession(r); err == nil {
		email = s.Email
	} else if e, err := ViaBearerToken(r); err == nil {
		email = e
	}
	if isAuthorized(email) {
		// TODO(stephana): Uncomment the following line when Debugf is different from Infof.
		// sklog.Debugf("User %s is on the allowlist", email)
		return email
	}

	sklog.Debugf("User %s is logged in but not on the list of allowed users.", email)
	return ""
}

// ID returns the user's ID, i.e. their opaque identifier, if they are
// logged in, and "" if they are not logged in.
func ID(r *http.Request) string {
	s, err := getSession(r)
	if err != nil {
		return ""
	}
	return s.ID
}

// UserIdentifiers returns both the email and opaque user id of the logged in
// user, and will return two empty strings if they are not logged in.
func UserIdentifiers(r *http.Request) (string, string) {
	s, err := getSession(r)
	if err != nil {
		return "", ""
	}
	return s.Email, s.ID
}

// IsGoogler determines whether the user is logged in with an @google.com account.
func IsGoogler(r *http.Request) bool {
	return strings.HasSuffix(LoggedInAs(r), "@google.com")
}

// IsAdmin determines whether the user is logged in with an account on the admin
// allow list. If true, user is allowed to perform admin tasks.
func IsAdmin(r *http.Request) bool {
	email := LoggedInAs(r)
	if adminAllow != nil {
		return adminAllow.Member(email)
	}
	return activeAdminEmailAllowList[email]
}

// IsEditor determines whether the user is logged in with an account on the
// editor allow list. If true, user is allowed to perform edits. Defaults to
// false if no editor allow list is provided.
func IsEditor(r *http.Request) bool {
	email := LoggedInAs(r)
	if editAllow != nil {
		return editAllow.Member(email)
	}
	return false
}

// IsViewer determines whether the user is allowed to view this server. Defaults
// to true if no viewer allow list is provided.
func IsViewer(r *http.Request) bool {
	email := LoggedInAs(r)
	if viewAllow != nil {
		return viewAllow.Member(email)
	}
	return true
}

// A JSON Web Token can contain much info, such as 'iss' and 'sub'. We don't care about
// that, we only want one field which is 'email'.
//
// {
//   "iss":"accounts.google.com",
//   "sub":"110642259984599645813",
//   "email":"jcgregorio@google.com",
//   ...
// }
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
	} else if strings.HasSuffix(fullhost, "."+COOKIE_DOMAIN_SKIA_CORP) {
		return COOKIE_DOMAIN_SKIA_CORP
	} else if strings.HasSuffix(fullhost, "."+COOKIE_DOMAIN_SKIA_ORG) {
		return COOKIE_DOMAIN_SKIA_ORG
	} else {
		sklog.Errorf("Unknown domain for host: %s; falling back to %s", fullhost, COOKIE_DOMAIN_SKIA_ORG)
		return COOKIE_DOMAIN_SKIA_ORG
	}
}

// CookieFor creates an encoded Cookie for the given user id.
func CookieFor(value *Session, r *http.Request) (*http.Cookie, error) {
	encoded, err := secureCookie.Encode(COOKIE_NAME, value)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode cookie")
	}
	return &http.Cookie{
		Name:     COOKIE_NAME,
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
	cookie, err := CookieFor(value, r)
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
//   https://security.google.com/settings/security/permissions
//
// to revoke any grants they make.
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("LogoutHandler")
	setSkIDCookieValue(w, r, &Session{})
	http.Redirect(w, r, r.FormValue("redirect"), 302)
}

// OAuth2CallbackHandler must be attached at a handler that matches
// the callback URL registered in the APIs Console. In this case
// "/oauth2callback".
func OAuth2CallbackHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("OAuth2CallbackHandler")
	cookie, err := r.Cookie(SESSION_COOKIE_NAME)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Invalid session state.", 500)
		return
	}

	state := r.FormValue("state")
	stateParts := strings.SplitN(state, ":", 3)
	redirect := "/"
	// If the state contains a redirect URL.
	if len(stateParts) == 3 {
		// state has this form:   <sessionid>:<hash(salt + original url)>:<original url>
		// See LoginURL for more details.
		state = stateParts[0]
		hash := stateParts[1]
		url := stateParts[2]
		expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(cookieSalt+url)))
		if hash == expectedHash {
			redirect = url
		} else {
			sklog.Warningf("Got an invalid redirect: %s != %s", hash, expectedHash)
		}
	}
	if state != cookie.Value {
		http.Error(w, "Session state doesn't match callback state.", 500)
		return
	}

	code := r.FormValue("code")
	sklog.Infof("Code: %s ", code[:5])
	token, err := oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		sklog.Errorf("Failed to authenticate: %s", err)
		http.Error(w, "Failed to authenticate.", 500)
		return
	}
	// idToken is a JSON Web Token. We only need to decode the token, we do not
	// need to validate the token because it came to us over HTTPS directly from
	// Google's servers.
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token returned.", 500)
		return
	}
	// The id token is actually three base64 encoded parts that are "." separated.
	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		http.Error(w, "Invalid id_token.", 500)
		return
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
		http.Error(w, "Failed to base64 decode id_token.", 500)
		return
	}
	// Finally decode the JSON.
	decoded := &decodedIDToken{}
	if err := json.Unmarshal(b, decoded); err != nil {
		sklog.Errorf("Failed to JSON decode token: %s", string(b))
		http.Error(w, "Failed to JSON decode id_token.", 500)
		return
	}

	email := strings.ToLower(decoded.Email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		http.Error(w, "Invalid email address received.", 500)
		return
	}

	if !isAuthorized(email) {
		http.Error(w, "Accounts from your domain are not allowed or your email address is not on the allow list.", 500)
		return
	}
	s := Session{
		Email:     email,
		ID:        decoded.ID,
		AuthScope: strings.Join(oauthConfig.Scopes, " "),
		Token:     token,
	}
	setSkIDCookieValue(w, r, &s)
	http.Redirect(w, r, redirect, 302)
}

// isAuthorized returns true if the given email address matches either the
// domain or the user allow list.
func isAuthorized(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if viewAllow != nil {
		return viewAllow.Member(email)
	}
	if len(activeUserDomainAllowList) > 0 && !activeUserDomainAllowList[parts[1]] && !activeUserEmailAllowList[email] {
		return false
	}
	return true
}

// StatusHandler returns the login status of the user as JSON that looks like:
//
// {
//   "Email":     "fred@example.com",
//   "ID":        "12342...34324",
//   "LoginURL":  "https://..."
//   "IsAGoogler": false,
//   "IsViewer":   true,
//   "IsEditor":   true,
//   "IsAdmin:     false
// }
//
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("StatusHandler")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	email, id := UserIdentifiers(r)
	body := struct {
		Email      string
		ID         string
		LoginURL   string
		IsAGoogler bool
		IsAdmin    bool
		IsEditor   bool
		IsViewer   bool
	}{
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
		if strings.HasSuffix(u.Host, "."+COOKIE_DOMAIN_SKIA_ORG) ||
			strings.HasSuffix(u.Host, "."+COOKIE_DOMAIN_SKIA_CORP) ||
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

// ForceAuthMiddleware does ForceAuth by returning a func that can be used as
// middleware.
func ForceAuthMiddleware(oauthCallbackPath string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return ForceAuth(h, oauthCallbackPath)
	}
}

// ForceAuth is middleware that enforces authentication
// before the wrapped handler is called. oauthCallbackPath is the
// URL path that the user is redirected to at the end of the auth flow.
func ForceAuth(h http.Handler, oauthCallbackPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := LoggedInAs(r)
		if userId == "" {
			if strings.HasPrefix(r.URL.Path, oauthCallbackPath) {
				// If this is the oauth2 callback, run that handler.
				OAuth2CallbackHandler(w, r)
				return
			} else {
				// If this is not the oauth callback then redirect.
				redirectUrl := LoginURL(w, r)
				sklog.Infof("Redirect URL: %s", redirectUrl)
				if redirectUrl == "" {
					httputils.ReportError(w, fmt.Errorf("Unable to get redirect URL."), "Redirect to login failed:", http.StatusInternalServerError)
					return
				}
				http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// RestrictWithMessage returns a middleware func which enforces that the user
// is logged in with an allowed account before the wrapped handler is called. It
// uses the given message when a user is denied access.
func RestrictWithMessage(allow allowed.Allow, msg string) func(http.Handler) http.Handler {
	if allow == nil {
		return func(h http.Handler) http.Handler { return h }
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email := LoggedInAs(r)
			if !allow.Member(email) {
				sklog.Warningf("%s: %s", msg, email)
				http.Error(w, msg, 403)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}

// Restrict returns a middleware func which enforces that the user is logged
// in with an allowed account before the wrapped handler is called.
func Restrict(allow allowed.Allow) func(http.Handler) http.Handler {
	return RestrictWithMessage(allow, "User is not in allowed list")
}

// RestrictAdmin is middleware which enforces that the user is logged in as an
// admin before the wrapped handler is called.  Filled in during InitWithAllow.
var RestrictAdmin = func(h http.Handler) http.Handler {
	sklog.Fatal("RestrictAdmin called but not configured with InitWithAllow.")
	return h
}

// RestrictEditor is middleware which enforces that the user is logged in as an
// editor before the wrapped handler is called.  Filled in during InitWithAllow.
var RestrictEditor = func(h http.Handler) http.Handler {
	sklog.Fatal("RestrictEditor called but not configured with InitWithAllow.")
	return h
}

// RestrictViewer is middleware which enforces that the user is logged in as a
// viewer before the wrapped handler is called.  Filled in during InitWithAllow.
var RestrictViewer = func(h http.Handler) http.Handler {
	sklog.Fatal("RestrictViewer called but not configured with InitWithAllow.")
	return h
}

// RestrictFn wraps an http.HandlerFunc, restricting it to the given allowed list.
func RestrictFn(h http.HandlerFunc, allow allowed.Allow) http.HandlerFunc {
	return Restrict(allow)(h).(http.HandlerFunc)
}

// RestrictAdminFn wraps an http.HandlerFunc, restricting it to admins.
func RestrictAdminFn(h http.HandlerFunc) http.HandlerFunc {
	return RestrictAdmin(h).(http.HandlerFunc)
}

// RestrictEditorFn wraps an http.HandlerFunc, restricting it to editors.
func RestrictEditorFn(h http.HandlerFunc) http.HandlerFunc {
	return RestrictEditor(h).(http.HandlerFunc)
}

// RestrictViewerFn wraps an http.HandlerFunc, restricting it to viewers.
func RestrictViewerFn(h http.HandlerFunc) http.HandlerFunc {
	return RestrictViewer(h).(http.HandlerFunc)
}

// splitAuthAllowList splits the given allow list into a set of domains and a
// set of individual emails
func splitAuthAllowList(allowList string) (map[string]bool, map[string]bool) {
	domains := map[string]bool{}
	emails := map[string]bool{}

	for _, entry := range strings.Fields(allowList) {
		trimmed := strings.ToLower(strings.TrimSpace(entry))
		if strings.Contains(trimmed, "@") {
			emails[trimmed] = true
		} else {
			domains[trimmed] = true
		}
	}

	return domains, emails
}

// setActiveAllowLists initializes activeUserDomainAllowList and
// activeUserEmailAllowList from authAllowList.
func setActiveAllowLists(authAllowList string) {
	if adminAllow != nil || editAllow != nil || viewAllow != nil {
		return
	}
	activeUserDomainAllowList, activeUserEmailAllowList = splitAuthAllowList(authAllowList)
	_, activeAdminEmailAllowList = splitAuthAllowList(DEFAULT_ADMIN_LIST)
}

// loginInfo is the JSON file format that client info is stored in as a kubernetes secret.
type loginInfo struct {
	Salt         string `json:"salt"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// tryLoadingFromKnownLocations tries to load the cookie salt, client id, and
// client secret from a file in a known location. If it fails then it returns
// the salt it was passed and the client id and secret are the empty string.
//
// Returns salt, clientID, clientSecret.
func tryLoadingFromKnownLocations() (string, string, string) {
	cookieSalt := ""
	clientID := ""
	clientSecret := ""
	var info loginInfo
	err := util.WithReadFile(LOGIN_CONFIG_FILE, func(f io.Reader) error {
		if err := json.NewDecoder(f).Decode(&info); err != nil {
			return err
		}
		cookieSalt = info.Salt
		clientID = info.ClientID
		clientSecret = info.ClientSecret
		return nil
	})

	if err == nil {
		sklog.Infof("Successfully loaded login secrets from file %s.", LOGIN_CONFIG_FILE)
		return cookieSalt, clientID, clientSecret
	}
	sklog.Infof("Failed to load login secrets from file %s. Got error: %s", LOGIN_CONFIG_FILE, err)
	return DEFAULT_COOKIE_SALT, "", ""
}

// ViaBearerToken tries to load an OAuth 2.0 Bearer token from from the request
// and derives the login email address from it.
func ViaBearerToken(r *http.Request) (string, error) {
	tok := r.Header.Get("Authorization")
	if tok == "" {
		return "", errors.New("User is not authenticated.")
	}
	tok = strings.TrimPrefix(tok, "Bearer ")
	tokenInfo, err := ValidateBearerToken(tok)
	if err != nil {
		return "", err
	}
	return tokenInfo.Email, nil
}

// ValidateBearerToken takes an OAuth 2.0 Bearer token (e.g. The third part of
// Authorization: Bearer ya29.Elj...
// and polls a Google HTTP endpoint to see if is valid. This is fine in low-volumne
// situations, but another solution may be needed if this goes higher than a few QPS.
func ValidateBearerToken(token string) (*oauth2_api.Tokeninfo, error) {
	c, err := oauth2_api.New(httputils.NewTimeoutClient())
	if err != nil {
		return nil, fmt.Errorf("could not make oauth2 api client: %s", err)
	}
	ti, err := c.Tokeninfo().AccessToken(token).Do()
	if err != nil {
		return nil, err
	}
	if ti.ExpiresIn <= 0 {
		return nil, fmt.Errorf("Token is expired.")
	}
	if !ti.VerifiedEmail {
		return nil, fmt.Errorf("Email not verified.")
	}
	return ti, nil
}
