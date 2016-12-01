package types

import "time"

// Activity stores information on one user action activity. This corresponds to
// one record in the activity database table. See DESIGN.md for details.
type Activity struct {
	ID     int
	TS     int64
	UserID string
	Action string
	URL    string
}

// Date returns an RFC3339 string for the Activity's TS.
func (a *Activity) Date() string {
	return time.Unix(a.TS, 0).Format(time.RFC3339)
}
