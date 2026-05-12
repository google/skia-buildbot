package schema

type RegressionsShortcutSchema struct {
	// The id for the shortcut.
	// Changed such fields from UUID to Text in https://b.corp.google.com/issues/492077374
	SID string `sql:"sid TEXT PRIMARY KEY"`

	// IDs of regressions this shortcut is leading to
	AnomalyIDs []string `sql:"anomaly_ids TEXT ARRAY"`

	IsLegacy bool `sql:"is_legacy BOOLEAN DEFAULT FALSE"`
}
