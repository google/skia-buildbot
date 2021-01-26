package types

import "go.skia.org/infra/am/go/incident"

// RecentIncidentsResponse - response of the "recent_incidents" endpoint.
type RecentIncidentsResponse struct {
	Incidents              []incident.Incident `json:"incidents"`
	Flaky                  bool                `json:"flaky"`
	RecentlyExpiredSilence bool                `json:"recently_expired_silence"`
}

// Stat - contains statistics of an incident.
type Stat struct {
	Num      int               `json:"num"`
	Incident incident.Incident `json:"incident"`
}

// StatsRequest - request of the "stats" endpoint.
type StatsRequest struct {
	Range string `json:"range"`
}

// StatsResponse - response of the "stats" endpoint.
type StatsResponse []*Stat

// IncidentsResponse - response of the "incidents" endpoint.
type IncidentsResponse struct {
	Incidents                    []incident.Incident `json:"incidents"`
	IdsToRecentlyExpiredSilences map[string]bool     `json:"ids_to_recently_expired_silences"`
}

// IncidentsInRangeRequest - request of the "incidents_in_range" endpoint.
type IncidentsInRangeRequest struct {
	Range    string            `json:"range"`
	Incident incident.Incident `json:"incident"`
}
