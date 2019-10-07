Storing diff metrics in Firestore
=================================

Computing diff metrics is an expensive task and often requires retrieving images from GCS. For this
reason, we cache them in Firestore.

This store essentially caches diff.DiffMetrics structs, so the schema is an almost 1:1 mapping of
said struct.

Schema
------

We should have a Firestore collection "diffmetrics" that will store documents with the fields below.

  	LeftAndRightDigests []string  # Only needed to purge metrics by digest IDs.
  	NumDiffPixels       int       # Maps to diff.DiffMetrics.NumDiffPixels.
  	PercentDiffPixels   float32   # Maps to diff.DiffMetrics.PixelDiffPercent.
  	MaxDiffR            int       # Maps to diff.DiffMetrics.MaxRGBADiffs[0].
  	MaxDiffG            int       # Maps to diff.DiffMetrics.MaxRGBADiffs[1].
  	MaxDiffB            int       # Maps to diff.DiffMetrics.MaxRGBADiffs[2].
  	MaxDiffA            int       # Maps to diff.DiffMetrics.MaxRGBADiffs[3].
  	DimensionsDiffer    bool      # Maps to diff.DiffMetrics.DimDiffer.

Field diff.DiffMetrics.Diffs, which is a map[string]float32, is not represented in the schema above.
This is because it's always populated with the same three entries ("percent", "pixel", "combined"),
and these can be computed from the other fields in the schema.

Documents in the "diffmetrics" collection are keyed by diff ID, e.g. "<left-digest>-<right-digest>".

Indexing
--------
The automatically created single-field indices should suffice.

Usage
-----
Diff metrics are queried by diff ID, which in Firestore terms means querying by document ID.

Purging is done on a per-digest basis. Given a digest ID, we find and then delete all documents such
that field LeftAndRightDigests contains the digest ID.
