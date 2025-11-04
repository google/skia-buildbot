package schema

// CulpritSchema represents the SQL schema of the Culprits table.
// TODO(wenbinzhang): remove anomaly group ids and issue ids as we have
// the info needed the group issue map
type CulpritSchema struct {
	Id string `sql:"id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

	// Repo this change belongs to e.g. chromium.googlesource.com
	Host string `sql:"host STRING"`

	// Project inside the repo e.g. chromium/src
	Project string `sql:"project STRING"`

	// Repo Ref e.g. refs/heads/main
	Ref string `sql:"ref STRING"`

	// Commit hash of the culprit change
	Revision string `sql:"revision STRING"`

	// Stored as a Unit timestamp.
	LastModified int64 `sql:"last_modified INT"`

	// List of Anomaly Group IDs  where this commit has been identified
	// as a culprit.
	AnomalyGroupIDs []string `sql:"anomaly_group_ids STRING ARRAY"`

	// List of Issue Ids associated with this culprit
	IssueIds []string `sql:"issue_ids STRING ARRAY"`

	// JSON map from anomaly group id to the issue id.
	GroupIssueMap interface{} `sql:"group_issue_map JSONB"`

	// Index by (host, project, ref, revision). Revision is kept first to
	// reduce hotspots
	byRevisionIndex struct{} `sql:"INDEX by_revision (revision, host, project, ref)"`
}
