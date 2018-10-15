// Package activitylog implements utility for activity logging into database.
package activitylog

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/ds"
	"google.golang.org/api/iterator"
)

// Activity stores information on one user action activity. This corresponds to
// one record in the activity database table. See DESIGN.md for details.
type Activity struct {
	ID     int64 `datastore:",noindex"`
	TS     int64
	UserID string `datastore:",noindex"`
	Action string `datastore:",noindex"`
	URL    string `datastore:",noindex"`
}

// Date returns an RFC3339 string for the Activity's TS.
func (a *Activity) Date() string {
	return time.Unix(a.TS, 0).Format(time.RFC3339)
}

// Write writes a new activity record to the db table activitylog.
// Input is in types.Activity format, but ID and TS are ignored. Instead, always
// use autoincrement ID and the current timestamp for the new record.
func Write(r *Activity) error {
	if r.TS == 0 {
		r.TS = time.Now().Unix()
	}
	key := ds.NewKey(ds.ACTIVITY)
	key.ID = r.ID
	if _, err := ds.DS.Put(context.TODO(), key, r); err != nil {
		return fmt.Errorf("Failed to store activity: %s", err)
	}
	return nil
}

// GetRecent returns the most recent n activity records in Activity struct format.
func GetRecent(n int) ([]*Activity, error) {
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
}
