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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"code.google.com/p/goauth2/oauth"
	"github.com/golang/glog"
	"github.com/gorilla/securecookie"
	"skia.googlesource.com/buildbot.git/perf/go/metadata"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

const (
	METADATA_KEY = "cookiesalt"

	LOCAL_COOKIE_SALT = "notverysecret"

	COOKIE_NAME = "skid"
)

var (
	// cookieSalt is some entropy for our encoders.
	cookieSalt = LOCAL_COOKIE_SALT

	secureCookie = securecookie.New([]byte(cookieSalt), nil)

	// oauthConfig is the OAuth 2.0 client configuration.
	//
	// The following information comes from:
	//
	//   https://console.developers.google.com/project/31977622648/apiui/credential
	//
	// That location is where you can change the accepted Redirect URLs.
	oauthConfig = &oauth.Config{
		ClientId:     "31977622648-cghipjk0o3g06gqogvrgohftesfs0t9q.apps.googleusercontent.com",
		ClientSecret: "F1DwOv0v0tiLBgm8ep5Umxof",
		Scope:        "email",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "http://localhost:8000/oauth2callback/",

		// We don't need a refresh token, we'll just go through the approval flow again.
		AccessType: "online",

		// And when we go through the approval flow again don't stop if they've already approved once.
		ApprovalPrompt: "auto",
	}

	// domainWhitelist is the list of domains that are allowed to log in to our site.
	domainWhitelist = []string{"google.com", "chromium.org", "skia.org"}
)

// Init must be called before any other methods. Pass in true for local if
// running locally, as opposed to in prod.
func Init(local bool) {
	if !local {
		// TODO(jcgregorio) Also read ClientSecret, and RedirectURL from metadata.
		cookieSalt, err := metadata.Get(METADATA_KEY)
		if err != nil {
			glog.Fatalf("Can't launch, unable to obtain %s from metadata server: %s. Remember to use --local when running locally.", METADATA_KEY, err)
		}
		secureCookie = securecookie.New([]byte(cookieSalt), nil)
		oauthConfig.RedirectURL = "http://skiaperf.com/oauth2callback/"
	}
}

// LoginURL returns a URL that the user is to be directed to for login.
func LoginURL() string {
	return oauthConfig.AuthCodeURL("")
}

// LoggedInAs returns the user's ID, i.e. their email address, if they are
// logged in, and "" if they are not logged in.
func LoggedInAs(r *http.Request) string {
	cookie, err := r.Cookie(COOKIE_NAME)
	if err != nil {
		return ""
	}
	var email string
	if err := secureCookie.Decode(COOKIE_NAME, cookie.Value, &email); err != nil {
		return ""
	}
	return email
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
}

func setSkIDCookieValue(w http.ResponseWriter, value string) {
	encoded, err := secureCookie.Encode(COOKIE_NAME, value)
	if err != nil {
		http.Error(w, "Failed to encode cookie.", 500)
		return
	}
	cookie := &http.Cookie{
		Name:     COOKIE_NAME,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
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
	glog.Infof("LogoutHandler\n")
	setSkIDCookieValue(w, "")
	http.Redirect(w, r, "/", 302)
}

// OAuth2CallbackHandler must be attached at a handler that matches
// the callback URL registered in the APIs Console. In this case
// "/oauth2callback".
func OAuth2CallbackHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("OAuth2CallbackHandler\n")
	code := r.FormValue("code")
	transport := &oauth.Transport{
		Config: oauthConfig,
		Transport: &http.Transport{
			Dial: util.DialTimeout,
		},
	}
	token, err := transport.Exchange(code)
	if err != nil {
		glog.Errorf("Failed to authenticate: %s", err)
		http.Error(w, "Failed to authenticate.", 500)
		return
	}
	// idToken is a JSON Web Token. We only need to decode the token, we do not
	// need to validate the token because it came to us over HTTPS directly from
	// Google's servers.
	idToken := token.Extra["id_token"]
	// The id token is actually three base64 encoded parts that are "." separated.
	segments := strings.Split(idToken, ".")
	if len(segments) != 3 {
		http.Error(w, "Invalid id_token.", 500)
		return
	}
	// Now base64 decode the middle segment, which decodes to JSON.
	padding := 4 - (len(segments[1]) % 4)
	b, err := base64.StdEncoding.DecodeString(segments[1] + strings.Repeat("=", padding))
	if err != nil {
		http.Error(w, "Failed to base64 decode id_token.", 500)
		return
	}
	// Finally decode the JSON.
	decoded := &decodedIDToken{}
	if err := json.Unmarshal(b, decoded); err != nil {
		http.Error(w, "Failed to JSON decode id_token.", 500)
		return
	}
	parts := strings.Split(decoded.Email, "@")
	if len(parts) != 2 {
		http.Error(w, "Invalid email address received.", 500)
		return
	}
	if !util.In(parts[1], domainWhitelist) {
		http.Error(w, "Accounts from your domain are not allowed.", 500)
		return
	}
	setSkIDCookieValue(w, decoded.Email)
	http.Redirect(w, r, "/", 302)
}

// StatusHandler returns the login status of the user as JSON that looks like:
//
// {
//   "Email": "fred@example.com",
//   "LoginURL": "https://..."
// }
//
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("StatusHandler\n")
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	body := map[string]string{
		"Email":    LoggedInAs(r),
		"LoginURL": LoginURL(),
	}
	if err := enc.Encode(body); err != nil {
		glog.Errorf("Failed to encode Login status to JSON", err)
	}
}
