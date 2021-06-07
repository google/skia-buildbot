package types

// StatusData is used in the response of the get_client_counts endpoint.
type StatusData struct {
	UntriagedCount int    `json:"untriaged_count"`
	Link           string `json:"link"`
}

type CQRecord struct {
	// The time the CQ first looked at this change.
	// Uses unix epoch time.
	StartTime int64
}
