package dsconst

import "go.skia.org/infra/go/ds"

// One const for each Datastore Kind.
const (
	TRACE ds.Kind = "FlatTrace"

	ISSUE             ds.Kind = "Issue"
	TRYJOB            ds.Kind = "TryJob"
	TRYJOB_RESULT     ds.Kind = "TryJobResult"
	TRYJOB_EXP_CHANGE ds.Kind = "TryJobExpChange"
	TEST_DIGEST_EXP   ds.Kind = "TestDigestExp"
)
