Storing ChangeLists and PatchSets in Firestore
==============================================

We need to keep track of the ChangeLists and PatchSets that have been associated
with TryJob data that we have ingested.

See https://docs.google.com/document/d/1d0tOhgx51QOGiSXqTxiwNSlgm1pYHTUSBK3agysX6Iw/edit
for more context if desired.

Schema
------

We should have two Firestore Collections (i.e. tables), one for ChangeList
and one for PatchSet.

	ChangeList
		ID         string  # SystemID + System
		SystemID   string  # The id of the CL in, for example, Gerrit
		System     string  # "gerrit", "github", etc
		Status     int     # corresponds to code_review.CLStatus
		Owner      string  # email address
		Updated    time.Time
		Subject    string

	PatchSet
		ID            string  # SystemID + System + ChangeListID
		SystemID      string  # The id of the PS in, for example, Gerrit
		System        string  # "gerrit", "github", etc
		ChangeListID  string  # SystemID from ChangeList
		Order         int     # number of this PS
		GitHash       string

Indexing
--------
We'll need some complex indices because we are adding a "System matches" addendum
to all queries (to avoid the small possibility being conflicts between two Systems
with the same ID) and because we care about most recent ChangeLists.

We'll need the following composite indexes:
TODO(kjlubick): once we try running this on a real database, we'll see what
indices we need.


We should mark ChangeList.Subject as no-index, to save some index space.
<https://cloud.google.com/firestore/docs/query-data/indexing#exemptions>

Usage
-----
We'll be querying:
 - ChangeLists by SystemID, System
 - PatchSets by SystemID, System, and ChangeListID

We have to use all these keys to make sure we don't have any collisions by CLs or
PSs with the same ID from different systems.

Growth Opportunities
-------------------

We could open up the searching to include "get only Open CLs" or get CLs by
a given (the logged in?) user.  This should just entail adding some more
indices and a few new functions to the API.