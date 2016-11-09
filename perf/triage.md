triage
======

The new perf needs a new triaging page, one that tracks regressions per
commit, and allows multiple different queries to be run.

  * data stored per commit.
  * both low and high clusters stored, if present.
  * one column for each query. two sub-cols for low and high.
  * queries can be added/removed semi-easily (flags or metadata).
  * the queries need to be stored with per-commit data, since they may change
    over time.
  * Each query/commit/(low|high) cell has a status:
    * Nfs - insufficient data.
    * New - untriaged - analysis found regression, but not yet triaged.
    * Bug
    * Ignore
    * A text comment.
    * Each low/high cell pops up a cluster-summary2-sk.

Each query and low/high needs to be ackowledged.

Do we need a page where we can trigger this analysis across a range of commits
for a given query?

If we re-run and find a better fit does it replace original? Yes.

DB Model
--------

 index   ts          text
 cid timestamp [serialized blob of JSON]

Also need to store the current set of queries? Or is that just part of
metadata/flags? Not stored in DB.

Need to search by a time range. possibly differentiating by commit id?

May also want to search by status? No. We can do that after a range search.

blob of JSON is a serialized

  // map[query]Regression.
  map[string]Regression

  type Regression struct {
      Low      *cluster2.ClusterSummary, // Can be nil.
      High     *cluster2.ClusterSummary, // Can be nil.
      Frame    dataframe.FrameResponse,
      Status   triage.Status
  }


  type triage.Status struct {
      Status   string
      Message  string
  }

  Status is "New", Bug", "Ignore", "Nsf".

  UI also recognizes Status of "" meaning
  don't display triage info.

  No permalink for individual cluster, but permalink
  for ranges on this display.

Delivered to browser as *
  [ "query1", "query2", "query3", ...],
  [
    { id: cid1, cols: [ Regression, Regression, Regression, ...], },
    { id: cid2, cols: [ Regression, null,       Regression, ...], },
    { id: cid3, cols: [ Regression, Regression, Regression, ...], },
  ]

*The list of queries is the union of the current queries being run, and
 all the queries that appear in all the map[string]Regression.

The UI for Regression must be able to handle null for a value.

The alert system will define a range [last 50 commits].
For each commit,
  For each query,
    Do clustering.
    Save results into datastore (SetStatus vs. SetLowSummary,SetHighSummary).
