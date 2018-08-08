package note

// Note is one note attached to an Incident or Silence.
type Note struct {
	Text   string `json:"text" datastore:"text"`
	Author string `json:"author" datastore:"author"`
	TS     int64  `json:"ts" datastore:"ts"` // Time in seconds since the epoch.
}
