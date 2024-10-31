package caching

const (
	// This query collects untriaged image digests within the specified commit window for the given
	// corpus where an ignore rule is not applied. This data is used when the user wants to see
	// a list of untriaged digests for the specific corpus in the UI.
	ByBlameQuery = `WITH
BeginningOfWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC
	OFFSET $1 LIMIT 1
),
UntriagedDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = 'u'
),
UnignoredDataAtHead AS (
	SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	JOIN BeginningOfWindow ON ValuesAtHead.most_recent_commit_id >= BeginningOfWindow.commit_id
	WHERE matches_any_ignore_rule = FALSE AND corpus = $2
)
SELECT UnignoredDataAtHead.trace_id, UnignoredDataAtHead.grouping_id, UnignoredDataAtHead.digest FROM
UntriagedDigests
JOIN UnignoredDataAtHead ON UntriagedDigests.grouping_id = UnignoredDataAtHead.grouping_id AND
	 UntriagedDigests.digest = UnignoredDataAtHead.digest`

	// This query returns all traces for the given corpus which do not have any corresponding ignore rule.
	UnignoredQuery = `WITH
BeginningOfWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC
	OFFSET $1 LIMIT 1
)
SELECT trace_id, grouping_id, digest FROM ValuesAtHead
JOIN BeginningOfWindow ON ValuesAtHead.most_recent_commit_id >= BeginningOfWindow.commit_id
WHERE corpus = $2 AND matches_any_ignore_rule = FALSE`
)
