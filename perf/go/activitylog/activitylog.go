// Package activitylog implements utility for activity logging into database.
package activitylog

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/ds"
)

var (
	useCloudDatastore bool
)

// Init initializes activitylog.
//
// useDS - Is true if activities should be store in Google Cloud Datastore, otherwise they're stored in Cloud SQL.
func Init(useDS bool) {
	useCloudDatastore = useDS
}

// Activity stores information on one user action activity. This corresponds to
// one record in the activity database table. See DESIGN.md for details.
type Activity struct {
	ID     int64
	TS     int64
	UserID string
	Action string
	URL    string
}

// Date returns an RFC3339 string for the Activity's TS.
func (a *Activity) Date() string {
	return time.Unix(a.TS, 0).Format(time.RFC3339)
}

// Write writes a new activity record to the db table activitylog.
// Input is in types.Activity format, but ID and TS are ignored. Instead, always
// use autoincrement ID and the current timestamp for the new record.
func Write(r *Activity) error {
	if useCloudDatastore {
		r.TS = time.Now().Unix()
		key := ds.NewKey(ds.ACTIVITY)
		if _, err := ds.DS.Put(context.TODO(), key, r); err != nil {
			return fmt.Errorf("Failed to store activity: %s", err)
		}
		return nil
	} else {
		// TODO(jcgregorio) Delete this code after the migration away from MySQL.
		sklog.Infof("Write activity: %s\n", r)
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
}

// GetRecent returns the most recent n activity records in Activity struct format.
func GetRecent(n int) ([]*Activity, error) {
	if useCloudDatastore {
		ret := []*Activity{}
		q := ds.NewQuery(ds.ACTIVITY).EventualConsistency().Limit(n).Order("-TS")
		it := ds.DS.Run(context.TODO(), q)
		for {
			a := &Activity{}
			k, err := it.Next(a)
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, fmt.Errorf("Failed retrieving activity list: %s", err)
			}
			a.ID = k.ID
			ret = append(ret, a)
		}
		return ret, nil
	} else {
		// TODO(jcgregorio) Delete this code after the migration away from MySQL.
		ret := []*Activity{}
		rows, err := db.DB.Query("SELECT id, timestamp, userid, action, url FROM activitylog ORDER BY id DESC LIMIT ?", n)
		if err != nil {
			return nil, fmt.Errorf("Failed to read from database: %s", err)
		}
		defer util.Close(rows)
		sklog.Infoln("Processing activity rows.")
		for rows.Next() {
			var id int
			var timestamp int64
			var userid string
			var action string
			var url string
			if err := rows.Scan(&id, &timestamp, &userid, &action, &url); err != nil {
				return nil, fmt.Errorf("Failed to read row from database: %s", err)
			}
			r := &Activity{
				ID:     int64(id),
				TS:     timestamp,
				UserID: userid,
				Action: action,
				URL:    url,
			}
			ret = append(ret, r)
		}

		return ret, nil
	}
}
