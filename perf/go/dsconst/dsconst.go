package dsconst

import "go.skia.org/infra/go/ds"

// One const for each Datastore Kind.
const (
	SHORTCUT   ds.Kind = "Shortcut"
	ACTIVITY   ds.Kind = "Activity"
	REGRESSION ds.Kind = "Regression"
	ALERT      ds.Kind = "Alert"
)
