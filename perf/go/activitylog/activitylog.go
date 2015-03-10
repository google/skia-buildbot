// Package activitylog implements utility for activity logging into database.
package activitylog

import (
	"fmt"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/types"
)

// Write writes a new activity record to the db table activitylog.
// Input is in types.Activity format, but ID and TS are ignored. Instead, always
// use autoincrement ID and the current timestamp for the new record.
func Write(r *types.Activity) error {
	glog.Infof("Write activity: %s\n", r)
	if r.UserID == "" || r.Action == "" {
		return fmt.Errorf("Activity UserID and Action cannot be empty: %v\n", r)
	}
	_, err := db.DB.Exec(
		"INSERT INTO activitylog (timestamp, userid, action, url) VALUES (?, ?, ?, ?)",
		time.Now().Unix(), r.UserID, r.Action, r.URL)
	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// GetRecent returns the most recent n activity records in types.Activity struct format.
func GetRecent(n int) ([]*types.Activity, error) {
	ret := []*types.Activity{}
	rows, err := db.DB.Query("SELECT id, timestamp, userid, action, url FROM activitylog ORDER BY id DESC LIMIT ?", n)
	if err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	defer util.Close(rows)
	glog.Infoln("Processing activity rows.")
	for rows.Next() {
		var id int
		var timestamp int64
		var userid string
		var action string
		var url string
		if err := rows.Scan(&id, &timestamp, &userid, &action, &url); err != nil {
			return nil, fmt.Errorf("Failed to read row from database: %s", err)
		}
		r := &types.Activity{
			ID:     id,
			TS:     timestamp,
			UserID: userid,
			Action: action,
			URL:    url,
		}
		ret = append(ret, r)
	}

	return ret, nil
}
