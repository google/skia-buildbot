package audit

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/am/go/types"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/auditlog"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const getLogsLimit = 200

// Log outputs the action/user/body to stdout and persists it in datastore.
func Log(r *http.Request, action string, body interface{}, alogin *proxylogin.ProxyLogin) {
	// Log to stdout.
	user := alogin.LoggedInAs(r)
	auditlog.LogWithUser(r, string(user), action, body)

	// Add the log to datastore to display in UI. Doing this in a Go routine
	// to avoid introducing latency in the UI.
	go func() {
		a := types.AuditLog{
			Action:    action,
			User:      string(user),
			Body:      fmt.Sprintf("%+v", body),
			Timestamp: time.Now().Unix(),
		}
		key := ds.NewKey(ds.AUDITLOG_AM)
		if _, err := ds.DS.Put(context.Background(), key, &a); err != nil {
			sklog.Errorf("Could not persist auditlog into DS: %s", err)
		}
	}()
}

func GetLogs(ctx context.Context) ([]*types.AuditLog, error) {
	logs := []*types.AuditLog{}
	q := ds.NewQuery(ds.AUDITLOG_AM).Order("-timestamp").Limit(getLogsLimit)
	if _, err := ds.DS.GetAll(ctx, q, &logs); err != nil {
		return nil, skerr.Wrap(err)
	}
	return logs, nil
}
