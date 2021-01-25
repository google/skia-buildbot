package types

import "go.skia.org/infra/am/go/incident"


// RecentIncidentsResponse - response of the "recent_incidents" endpoint.
type RecentIncidentsResponse struct {
	Incidents              []incident.Incident `json:"incidents"`
	Flaky                  bool                `json:"flaky"`
	RecentlyExpiredSilence bool                `json:"recently_expired_silence"`
}