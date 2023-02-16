package schema

// ShortcutSchema represents the SQL schema of the Shortcuts table.
type ShortcutSchema struct {
	ID string `sql:"id TEXT UNIQUE NOT NULL PRIMARY KEY"`

	// TraceIDs is a shortcut.Shortcut serialized as JSON.
	TraceIDs string `sql:"trace_ids TEXT"`
}
