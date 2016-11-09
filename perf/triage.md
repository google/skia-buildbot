Triage
======

The new perf needs a new triaging page, one that tracks regressions per
commit, and allows multiple different queries, such as "source_type=skp" to be
run.

Requirements
------------

Right now alerting is run only on the last 50 commits as one monolithic query
for skps only. The new triaging page should:

  * Run clustering with a window of +/- 5 commits on either side for each
    commit.
  * Only steps that occur at the selected commit will count as a Regression.
  * Low and high clusters are stored per commit, per query. (As opposed to by
    cluster only in the old system).
  * Both low and high cluster regressions are stored, if present.
  * Each query and low/high cluster needs to be triaged.

Design
------

The UI will look roughly like this:

    +----------------------------------+------------------------------------------+
    |            Commit                |               Queries                    |
    +-------------------------------------------------------+---------------------+
    |                                  | source_type=skp    |   source_type=svg   |
    |                                  | sub_result=min_ms  |   sub_result=min_ms |
    |                                  +--------+--------------------+------------+
    |                                  |   High | Low       |  High  | Low        |
    |                                  +--------+--------------------+------------+
    | 5166651 Fix FreePageCount alert. |      N | N         |     N  | N          |
    | 669f2da [task scheduler] Get the |      N | N         |     N  | N          |
    | a6ebcd2 Make Status categorize I |        |           |        |            |
    | 501bc48 [CQ Watcher] Log when a  |      I |           |        | ↓          |
    | 1b31bb5 [CT] Reduce number of pa |        |           |        |            |
    | ea80add Add ability to specify c |      ↑ | ↓         |        |            |
    | c844730 Fix alert queries due to |        |           |        | B          |
    +----------------------------------+--------+-----------+--------+------------+

  * In the above sketch:
    * 'N' means insufficient data to do clustering.
    * '↑' means an untriaged High regression was found, i.e. a step up.
    * '↓' means an untriaged Low regression was found, i.e. a step down.
    * 'I' means Ignore.
    * 'B' means Bug.
    * ' ' means sufficient data to cluster, but no regressions found.
  * One column for each query, two sub-cols for low and high regressions.
  * Queries can be added/removed semi-easily (flags or metadata).
  * The queries need to be stored with per-commit data, since they may change
    over time, i.e. we may add new queries or stop running old queries.
  * Each non-empty query/commit/(low|high) cell has:
    * Status:
      * Nsf - insufficient data to do clustering around that commit.
      * New - untriaged - analysis found regression, but not yet triaged.
      * Bug
      * Ignore - An expected regression, or a noisy cluster.
    * A text comment.
  * Each low/high cell pops up a cluster-summary2-sk that allows inspecting
    the centroid of the regression, the members of the cluster, and triaging
    the cluster.

The data stored for each commit of the triage page analysis will be:

    // map[query]Regression.
    map[string]Regression

    type Regression struct {
        Low        *cluster2.ClusterSummary, // Can be nil.
        High       *cluster2.ClusterSummary, // Can be nil.
        Frame      dataframe.FrameResponse,
        LowStatus  TriageStatus
        HighStatus TriageStatus
    }

    type TriageStatus struct {
        Status   string
        Message  string
    }

    Status is "New", Bug", "Ignore", or "Nsf".


The alerting analysis will run the following analysis continuously:

    The alert system will define a range [last 50 commits].
    For each commit:
      For each query:
        * Do clustering.
        * Save results into database for regressions that show up
          for that commit.

DB Model
--------

A new table will need to be added to store the Regressions, which will be
indexed by cid.CommitID, and will also have a column for timestamp to make
time range queries easy. The Regression will be stored as serialized JSON in a
text field.

 Column:     index       ts          text
              cid     timestamp  [serialized map[string]Regression as JSON]

UI Model
--------

The map[string]Regression is delivered to the browser as the following
struct to make it easier to use Polymer repeat templates:


    {
      header: [ "query1", "query2", "query3", ...],
      table: [
        { id: cid1, cols: [ Regression, Regression, Regression, ...], },
        { id: cid2, cols: [ Regression, null,       Regression, ...], },
        { id: cid3, cols: [ Regression, Regression, Regression, ...], },
      ]
    }

Note that the list of queries is the union of the current queries being run, and
all the queries that appear in all the map[string]Regression's.

The UI for Regression must be able to handle null for a value, which signifies
a cell for which no data exists.

