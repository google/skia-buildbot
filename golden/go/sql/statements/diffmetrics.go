package statements

import "fmt"

const rangeStatement = `
WITH
InitialRankings AS (
	SELECT DiffMetrics.*, rank() over (
		PARTITION BY left_digest
		ORDER BY combined_metric ASC, num_diff_pixels ASC
	) AS initialRank
	FROM DiffMetrics
	%s
),
DiffsWithLabels AS (
    SELECT DISTINCT InitialRankings.*, max(label) OVER(PARTITION BY right_digest) AS max_label
    FROM InitialRankings
    JOIN Expectations ON Expectations.digest = InitialRankings.right_digest
    WHERE initialRank < 100
),
RankedDiffs AS (
	SELECT DiffsWithLabels.*, rank() over (
		PARTITION BY left_digest, max_label
		ORDER BY combined_metric ASC, num_diff_pixels ASC
	) AS diffRank
	from DiffsWithLabels
),
TopOfEachLabel AS (
	SELECT RankedDiffs.*, rank() OVER (
		PARTITION BY left_digest
		ORDER BY combined_metric ASC, num_diff_pixels ASC
	) AS overallRank
	FROM RankedDiffs
	WHERE (RankedDiffs.max_label = 0 AND RankedDiffs.diffRank <= 3) OR
	      (RankedDiffs.max_label = 1 AND RankedDiffs.diffRank <= 5) OR
	      (RankedDiffs.max_label = 2 AND RankedDiffs.diffRank <= 2)
)
UPSERT INTO DiffMetricsClosestView
SELECT left_digest, overallRank, right_digest, num_diff_pixels, pixel_diff_percent,
	max_channel_diff, max_rgba_diff, combined_metric, dimensions_differ
FROM TopOfEachLabel;`

// CreateDiffMetricsClosestViewShard returns a SQL statement that requires no arguments which will
// perform one shard of the work needed to create the DiffMetricsClosestView table. These are
// sharded 255 ways, due to the fact the statement groups by the first byte.
func CreateDiffMetricsClosestViewShard(shard byte) string {
	if shard == 255 {
		return fmt.Sprintf(rangeStatement, `WHERE left_digest > x'ff'`)
	}
	r := fmt.Sprintf(`WHERE left_digest > x'%02x' and left_digest < x'%02x'`, shard, shard+1)
	return fmt.Sprintf(rangeStatement, r)
}

// CreateDiffMetricsClosestView is an unsharded version of CreateDiffMetricsClosestViewShard, meant
// for small databases/tests.
func CreateDiffMetricsClosestView() string {
	return fmt.Sprintf(rangeStatement, "")
}
