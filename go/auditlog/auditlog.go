package auditlog

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/sklog"
)

type auditLog struct {
	Type   string      `json:"type"`
	Action string      `json:"action"`
	User   string      `json:"user"`
	Body   interface{} `json:"body"`
}

// LogWithUser is used to create structured logs for auditable actions.
//
// This is only intended to be used on GKE, since by default GKE is
// configured to handle structured logs emitted on stdout and stderr.
//
// See: https://cloud.google.com/logging/docs/structured-logging
//
// user should be an identifier for the user from the login system.
func LogWithUser(r *http.Request, user, action string, body interface{}) {
	a := auditLog{
		Type:   "audit",
		Action: action,
		User:   user,
		Body:   body,
	}
	b, err := json.Marshal(a)
	if err != nil {
		sklog.Errorf("Failed to marshall audit log entry: %s", err)
	}
	fmt.Println(string(b))
}
