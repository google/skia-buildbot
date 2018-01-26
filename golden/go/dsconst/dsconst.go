package dsconst

import "go.skia.org/infra/go/ds"

// One const for each Datastore Kind.
const (
	TILE      ds.Kind = "Tile"
	TEST_NAME ds.Kind = "TestName"
	TRACE     ds.Kind = "Trace"

	ISSUE             ds.Kind = "Issue"
	TRYJOB            ds.Kind = "TryJob"
	TRYJOB_RESULT     ds.Kind = "TryJobResult"
	TRYJOB_EXP_CHANGE ds.Kind = "TryJobExpChange"
	TEST_DIGEST_EXP   ds.Kind = "TestDigestExp"
)
