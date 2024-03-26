package schema

type SubscriptionSchema struct {
	// Unique name identifying subscription.
	Name string `sql:"name STRING UNIQUE NOT NULL"`

	// infra_internal Git hash on which a subscription is based on.
	Revision string `sql:"revision STRING NOT NULL"`

	// Labels to attach to bugs associated with a subscription.
	BugLabels []string `sql:"bug_labels STRING ARRAY"`

	// Hotlists to add to bugs associated with a subscription.
	Hotlists []string `sql:"hotlists STRING ARRAY"`

	// Component in which to file bugs associated with a subscription.
	BugComponent string `sql:"bug_component STRING"`

	// Emails to CC in bugs associated with a subscription.
	BugCCEmails []string `sql:"bug_cc_emails STRING ARRAY"`

	// Owner of subscription. Used for contact purposes.
	ContactEmail string `sql:"contact_email STRING"`

	// Name and revision are used to key a subscription.
	PrimaryKey struct{} `sql:"PRIMARY KEY(name, revision)"`
}
