// login handles logging in users.
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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	COOKIE_NAME         = "sktoken"
	SESSION_COOKIE_NAME = "sksession"
	DEFAULT_COOKIE_SALT = "notverysecret"

	// DEFAULT_DOMAIN_WHITELIST is a white list of domains we use frequently.
	DEFAULT_DOMAIN_WHITELIST = "google.com chromium.org skia.org"

	// DEFAULT_ADMIN_WHITELIST is the white list of users we consider admins when we can't retrieve the whitelist from metadata.
	DEFAULT_ADMIN_WHITELIST = "benjaminwagner@google.com borenet@google.com jcgregorio@google.com kjlubick@google.com rmistry@google.com stephana@google.com"

	// COOKIE_DOMAIN is the domain that are cookies attached to.
	COOKIE_DOMAIN = "skia.org"
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

	// activeUserDomainWhiteList is the list of domains that are allowed to
	// log in.
	activeUserDomainWhiteList map[string]bool

	// activeUserEmailWhiteList is the list of email addresses that are
	// allowed to log in (even if the domain is not whitelisted).
	activeUserEmailWhiteList map[string]bool

	// activeAdminEmailWhiteList is the list of email addresses that are
	// allowed to perform admin tasks.
	activeAdminEmailWhiteList map[string]bool

	DEFAULT_SCOPE = []string{"email"}
)

// Session is encrypted and serialized and stored in a user's cookie.
type Session struct {
	Email     string
	ID        string
	AuthScope string
	Token     *oauth2.Token
}

// Init must be called before any other login methods.
//
// The function first tries to load the cookie salt, client id, and client
// secret from GCE project level metadata. If that fails it looks for a
// "client_secret.json" file in the current directory to extract the client id
// and client secret from. If both of those fail then it returns an error.
//
// The authWhiteList is the space separated list of domains and email addresses
// that are allowed to log in. The authWhiteList will be overwritten from
// GCE instance level metadata if present.
func Init(redirectURL string, scopes []string, authWhiteList string) error {
	cookieSalt, clientID, clientSecret := tryLoadingFromMetadata()
	if clientID == "" {
		b, err := ioutil.ReadFile("client_secret.json")
		if err != nil {
			return fmt.Errorf("Failed to read from metadata and from client_secret.json file: %s", err)
		}
		config, err := google.ConfigFromJSON(b)
		if err != nil {
			return fmt.Errorf("Failed to read from metadata and decode client_secret.json file: %s", err)
		}
		clientID = config.ClientID
		clientSecret = config.ClientSecret
	}
	initLogin(clientID, clientSecret, redirectURL, cookieSalt, scopes, authWhiteList)
	return nil
}

// initLogin sets the params.  It should only be called directly for testing purposes.
// Clients should use Init().
func initLogin(clientID, clientSecret, redirectURL, cookieSalt string, scopes []string, authWhiteList string) {
	secureCookie = securecookie.New([]byte(cookieSalt), nil)
	oauthConfig.ClientID = clientID
	oauthConfig.ClientSecret = clientSecret
	oauthConfig.RedirectURL = redirectURL
	oauthConfig.Scopes = scopes

	setActiveWhitelists(authWhiteList)
}

// LoginURL returns a URL that the user is to be directed to for login.
func LoginURL(w http.ResponseWriter, r *http.Request) string {
	// Check for a session id, if not there then assign one, and add it to the redirect URL.
	session, err := r.Cookie(SESSION_COOKIE_NAME)
	state := ""
	if err != nil || session.Value == "" {
		state, err = util.GenerateID()
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
		}
		http.SetCookie(w, cookie)
	} else {
		state = session.Value
	}

	redirect := r.Referer()
	if redirect == "" {
		redirect = "/"
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
	// once, unless they have a valid token but aren't in the whitelist,
	// in which case we want to use ApprovalForce so they get the chance
	// to pick a different account to log in with.
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline}
	s, err := getSession(r)
	if err == nil && !inWhitelist(s.Email) {
		opts = append(opts, oauth2.ApprovalForce)
	} else {
		opts = append(opts, oauth2.SetAuthURLParam("approval_prompt", "auto"))
	}
	return oauthConfig.AuthCodeURL(state, opts...)

}

func getSession(r *http.Request) (*Session, error) {
	cookie, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		return nil, err
	}
	var s Session
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
	s, err := getSession(r)
	if err != nil {
		return ""
	}
	if !inWhitelist(s.Email) {
		return ""
	}
	return s.Email
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
// whitelist. If true, user is allowed to perform admin tasks.
func IsAdmin(r *http.Request) bool {
	return activeAdminEmailWhiteList[LoggedInAs(r)]
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
	if host == "localhost" {
		return host
	}
	return COOKIE_DOMAIN
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
	sklog.Infof("LogoutHandler\n")
	setSkIDCookieValue(w, r, &Session{})
	http.Redirect(w, r, r.FormValue("redirect"), 302)
}

// OAuth2CallbackHandler must be attached at a handler that matches
// the callback URL registered in the APIs Console. In this case
// "/oauth2callback".
func OAuth2CallbackHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("OAuth2CallbackHandler\n")
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
			sklog.Warning("Got an invalid redirect: %s != %s", hash, expectedHash)
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

	if !inWhitelist(email) {
		http.Error(w, "Accounts from your domain are not allowed or your email address is not white listed.", 500)
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

// inWhitelist returns true if the given email address matches either the
// domain or the user whitelist.
func inWhitelist(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	if len(activeUserDomainWhiteList) > 0 && !activeUserDomainWhiteList[parts[1]] && !activeUserEmailWhiteList[email] {
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
// }
//
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("StatusHandler\n")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	email, id := UserIdentifiers(r)
	body := struct {
		Email      string
		ID         string
		LoginURL   string
		IsAGoogler bool
	}{
		Email:      email,
		ID:         id,
		LoginURL:   LoginURL(w, r),
		IsAGoogler: IsGoogler(r),
	}
	if err := enc.Encode(body); err != nil {
		sklog.Errorf("Failed to encode Login status to JSON: %s", err)
	}
}

// GetHttpClient returns a http.Client which performs authenticated requests as
// the logged-in user.
func GetHttpClient(r *http.Request) *http.Client {
	s, err := getSession(r)
	if err != nil {
		sklog.Errorf("Failed to get session state; falling back to default http client.")
		return &http.Client{}
	}
	return oauthConfig.Client(oauth2.NoContext, s.Token)
}

// ForceAuth is middleware that enforces authentication
// before the wrapped handler is called. oauthCallbackPath is the
// URL path that the user is redirected to at the end of the auth flow.
func ForceAuth(h http.Handler, oauthCallbackPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := LoggedInAs(r)
		if userId == "" {
			// If this is not the oauth callback then redirect.
			if !strings.HasPrefix(r.URL.Path, oauthCallbackPath) {
				redirectUrl := LoginURL(w, r)
				sklog.Infof("Redirect URL: %s", redirectUrl)
				if redirectUrl == "" {
					httputils.ReportError(w, r, fmt.Errorf("Unable to get redirect URL."), "Redirect to login failed:")
					return
				}
				http.Redirect(w, r, redirectUrl, http.StatusTemporaryRedirect)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func splitAuthWhiteList(whiteList string) (map[string]bool, map[string]bool) {
	domains := map[string]bool{}
	emails := map[string]bool{}

	for _, entry := range strings.Fields(whiteList) {
		trimmed := strings.ToLower(strings.TrimSpace(entry))
		if strings.Contains(trimmed, "@") {
			emails[trimmed] = true
		} else {
			domains[trimmed] = true
		}
	}

	return domains, emails
}

// setActiveWhitelists initializes activeDomainWhiteList and
// activeEmailWhiteList from instance metadata; or if metadata is not available,
// from authWhiteList.
func setActiveWhitelists(authWhiteList string) {
	authWhiteList = metadata.GetWithDefault(metadata.AUTH_WHITE_LIST, authWhiteList)
	activeUserDomainWhiteList, activeUserEmailWhiteList = splitAuthWhiteList(authWhiteList)
	adminWhiteList := metadata.ProjectGetWithDefault(metadata.ADMIN_WHITE_LIST, DEFAULT_ADMIN_WHITELIST)
	_, activeAdminEmailWhiteList = splitAuthWhiteList(adminWhiteList)
}

// tryLoadingFromMetadata tries to load the cookie salt, client id, and client
// secret from GCE project level metadata. If it fails then it returns the salt
// it was passed and the client id and secret are the empty string.
//
// Returns salt, clientID, clientSecret.
func tryLoadingFromMetadata() (string, string, string) {
	cookieSalt, err := metadata.ProjectGet(metadata.COOKIESALT)
	if err != nil {
		return DEFAULT_COOKIE_SALT, "", ""
	}
	clientID, err := metadata.ProjectGet(metadata.CLIENT_ID)
	if err != nil {
		return DEFAULT_COOKIE_SALT, "", ""
	}
	clientSecret, err := metadata.ProjectGet(metadata.CLIENT_SECRET)
	if err != nil {
		return DEFAULT_COOKIE_SALT, "", ""
	}
	return cookieSalt, clientID, clientSecret
}
