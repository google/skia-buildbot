package login

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/sklog"
)

const (
	// DEFAULT_ADMIN_LIST is list of users we consider to be admins as a
	// fallback when we can't retrieve the list from metadata.
	DEFAULT_ADMIN_LIST = "borenet@google.com jcgregorio@google.com kjlubick@google.com lovisolo@google.com rmistry@google.com cmumford@google.com"
)

var (
	// activeUserDomainAllowList is the list of domains that are allowed to
	// log in.
	activeUserDomainAllowList map[string]bool

	// activeUserEmailAllowList is the list of email addresses that are
	// allowed to log in (even if the domain is not explicitly allowed).
	activeUserEmailAllowList map[string]bool

	// activeAdminEmailAllowList is the list of email addresses that are
	// allowed to perform admin tasks.
	activeAdminEmailAllowList map[string]bool

	// Auth groups which determine whether a given user has particular types
	// of access. If nil, fall back on domain and individual email allow lists.
	adminAllow allowed.Allow
	editAllow  allowed.Allow
	viewAllow  allowed.Allow
)

// SimpleInitWithAllow initializes the login system for the default case (see
// docs for SimpleInitMust) and sets the admin, editor, and viewer lists. These
// may be nil, in which case we fall back on the default settings. For editors
// we default to denying access to everyone, and for viewers we default to
// allowing access to everyone.
func SimpleInitWithAllow(ctx context.Context, port string, local bool, admin, edit, view allowed.Allow) {
	redirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", port)
	if !local {
		redirectURL = GetDefaultRedirectURL()
	}
	InitWithAllow(ctx, redirectURL, admin, edit, view)
}

// InitWithAllow initializes the login system with the given redirect URL. Sets
// the admin, editor, and viewer lists as provided. These may be nil, in which
// case we fall back on the default settings. For editors we default to denying
// access to everyone, and for viewers we default to allowing access to
// everyone.
func InitWithAllow(ctx context.Context, redirectURL string, admin, edit, view allowed.Allow) {
	adminAllow = admin
	editAllow = edit
	viewAllow = view
	if err := Init(ctx, redirectURL, defaultAllowedDomains, ""); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}
	RestrictAdmin = RestrictWithMessage(adminAllow, "User is not an admin")
	RestrictEditor = RestrictWithMessage(editAllow, "User is not an editor")
	RestrictViewer = RestrictWithMessage(viewAllow, "User is not a viewer")
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

// IsEditorEmail returns true if the passed in email is on the edit Allowed. If none was configured
// (e.g. login.InitWithAllow was not used), it returns false to err on the side of failing safe.
func IsEditorEmail(email string) bool {
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

// FakeAllows is to be used by unit tests to set the auth groups
func FakeAllows(admin, edit, view allowed.Allow) {
	adminAllow = admin
	editAllow = edit
	viewAllow = view
}
