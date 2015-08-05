/*
	Utility functions used by all of ctfe.
*/

package util

import (
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"go.skia.org/infra/go/login"
	skutil "go.skia.org/infra/go/util"
)

// Function to run before executing a template.
var PreExecuteTemplateHook = func() {}

func UserHasEditRights(r *http.Request) bool {
	return strings.HasSuffix(login.LoggedInAs(r), "@google.com") || strings.HasSuffix(login.LoggedInAs(r), "@chromium.org")
}

func UserHasAdminRights(r *http.Request) bool {
	// TODO(benjaminwagner): Add this list to GCE project level metadata and retrieve from there.
	admins := map[string]bool{
		"benjaminwagner@google.com": true,
		"borenet@google.com":        true,
		"jcgregorio@google.com":     true,
		"rmistry@google.com":        true,
		"stephana@google.com":       true,
	}
	return UserHasEditRights(r) && admins[login.LoggedInAs(r)]
}

func ExecuteSimpleTemplate(template *template.Template, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	PreExecuteTemplateHook()
	if err := template.Execute(w, struct{}{}); err != nil {
		skutil.ReportError(w, r, err, fmt.Sprintf("Failed to expand template: %v", err))
		return
	}
}
