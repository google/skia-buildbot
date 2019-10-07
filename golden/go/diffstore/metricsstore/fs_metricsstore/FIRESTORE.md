Storing diff metrics in Firestore
=================================

Computing diff metrics is an expensive task and often requires retrieving images from GCS. For this
reason, we cache them in Firestore.

This store essentially caches diff.DiffMetrics structs, so the schema is an almost 1:1 mapping of
said struct.

Schema
------

We should have a Firestore collection "metricsstore_diffmetrics" that will store documents with the
fields below.

  	LeftAndRightDigests []string  # Only needed to purge metrics by digest IDs. This is an array
  	                              # instead of two fields LeftDigest and RightDigest to simplify
  	                              # querying (i.e. allows using "array-contains").
  	NumDiffPixels       int
  	PercentDiffPixels   float32   # Maps to diff.DiffMetrics.PixelDiffPercent.
  	MaxRGBADiffs        []int
  	DimensionsDiffer    bool      # Maps to diff.DiffMetrics.DimDiffer.

Field diff.DiffMetrics.Diffs, which is a map[string]float32, is not represented in the schema above.
This is because it's always populated with the same three entries ("percent", "pixel", "combined"),
and these can be computed from the other fields in the schema.

Documents in the "diffmetrics" collection are keyed by diff ID, e.g. "<left-digest>-<right-digest>".

Indexing
--------
The automatically created single-field index for LeftAndRightDigests should suffice.

All other fields can be excluded from indexing.

Usage
-----
Diff metrics are queried by diff ID, which in Firestore terms means querying by document ID.

Purging is done on a per-digest basis. Given a digest ID, we find and then delete all documents such
that field LeftAndRightDigests contains the digest ID.

No need to update diff metrics as they are immutable.
