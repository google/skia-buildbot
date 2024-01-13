package schema

type GraphsShortcutSchema struct {
	ID string `sql:"id TEXT UNIQUE NOT NULL PRIMARY KEY"`

	Graphs string `sql:"graphs TEXT"`
}
