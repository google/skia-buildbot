Storing digest failures in Firestore
====================================

This store caches diff.DigestFailure structs, so the schema is an 1:1 mapping of said struct.

Schema
------

We should have a Firestore collection "failurestore_failures" that will store documents with the
fields below.

	  Digest string  # ID of the digest that failed.
	  Reason string  # See type diff.DiffErr. The only value currently in use is "http_error".
	  TS     int     # Failure timestamp in milliseconds since the epoch.

Documents in this collection are keyed by digest ID and timestamp, e.g. "<digest>-<ts>".

Indexing
--------
No additional indexes are necessary.

Usage
-----
When digest failures are read, the entire collection is retrieved and kept in memory. We assume the
number of stored failures to be fairly small.

Purging is done on a per-digest basis. Given a list of digest IDs, the entire contents of the
collection are retrieved, and any documents that match the given digest IDs are deleted.

No need to update digest failures as they are immutable.
