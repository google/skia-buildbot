package schema

// CulpritSchema represents the SQL schema of the Culprits table.
type CulpritSchema struct {
	// Repo this change belongs to e.g. chromium.googlesource.com
	Host string `sql:"host STRING"`

	// Project inside the repo e.g. chromium/src
	Project string `sql:"project STRING"`

	// Repo Ref e.g. refs/heads/main
	Ref string `sql:"ref STRING"`

	// Commit hash of the culprit change
	Revision string `sql:"revision STRING"`

	// Stored as a Unit timestamp.
	LastModified int `sql:"last_modified INT"`

	// List of Anomaly Group IDs  where this commit has been identified
	// as a culprit.
	AnomalyGroupIDs []int `sql:"anomaly_group_ids INT ARRAY"`

	// List of Issue Ids associated with this culprit
	IssueIds []int `sql:"issue_ids INT ARRAY"`

	primaryKey struct{} `sql:"PRIMARY KEY (host, project, ref, revision)"`
}
