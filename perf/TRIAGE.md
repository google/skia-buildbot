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
    | 5166651 Fix FreePageCount alert. |        |           |        |            |
    | 669f2da [task scheduler] Get the |        |           |        |            |
    | a6ebcd2 Make Status categorize I |        |           |        |            |
    | 501bc48 [CQ Watcher] Log when a  |      ✓ |           |        | ?          |
    | 1b31bb5 [CT] Reduce number of pa |        |           |        |            |
    | ea80add Add ability to specify c |      ✓ | ✗         |        |            |
    | c844730 Fix alert queries due to |        |           |        | ✗          |
    +----------------------------------+--------+-----------+--------+------------+

  * In the above sketch:
    * '?' means an untriaged regression was found.
    * '✓' means this change is acceptable.
    * '✗' means bug.
    * ' ' means sufficient data to cluster, but no regressions found.
  * One column for each query, two sub-cols for low and high regressions.
  * Queries can be added/removed semi-easily (flags or metadata).
  * The queries need to be stored with per-commit data, since they may change
    over time, i.e. we may add new queries or stop running old queries.
  * Each non-empty query/commit/(low|high) cell has:
    * Status:
      * untriaged - Analysis found regression, but not yet triaged.
      * negative - This is a regression.
      * positive - An expected change in behavior, or a noisy cluster.
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

    Status is "untriaged", "positive", or "negative".


The alerting analysis will run the following analysis continuously:

    The alert system will define a range [last 50 commits].
    For each commit:
      For each query:
        * Do clustering.
        * Save results into database for regressions that show up
          for that commit.

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

