package caching

const (
	// This query collects untriaged image digests within the specified commit window for the given
	// corpus where an ignore rule is not applied. This data is used when the user wants to see
	// a list of untriaged digests for the specific corpus in the UI.
	ByBlameQuery = `WITH
UntriagedDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = 'u'
),
UnignoredDataAtHead AS (
	SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	WHERE most_recent_commit_id >= $1 AND matches_any_ignore_rule = FALSE AND corpus = $2
)
SELECT UnignoredDataAtHead.trace_id, UnignoredDataAtHead.grouping_id, UnignoredDataAtHead.digest FROM
UntriagedDigests
JOIN UnignoredDataAtHead ON UntriagedDigests.grouping_id = UnignoredDataAtHead.grouping_id AND
	 UntriagedDigests.digest = UnignoredDataAtHead.digest`
)
