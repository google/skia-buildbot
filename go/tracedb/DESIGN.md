tracedb
=======

The tracedb package is designed to replace the current storage system for
traces, tiles, with a new BoltDB backend that allows for much more flexibility
and an increase in the size of data that can be stored.  The new system needs
to support both branches and trybots (note that in the future there may be no
difference between the two), while still supporting the current capabilities
of looking at master.

The current structure for a Tile looks like:

    type GoldenTrace struct {
      Params_ map[string]string
      Values  []string
    }

    type PerfTrace struct {
      Values  []float64         `json:"values"`
      Params_ map[string]string `json:"params"`
    }

    // Commit is information about each Git commit.
    type Commit struct {
      CommitTime int64  `json:"commit_time" bq:"timestamp" db:"ts"`
      Hash       string `json:"hash"        bq:"gitHash"   db:"githash"`
      Author     string `json:"author"                     db:"author"`
    }

    // Tile is a config.TILE_SIZE commit slice of data.
    //
    // The length of the Commits array is the same length as all of the Values
    // arrays in all of the Traces.
    type Tile struct {
      Traces   map[string]Trace    `json:"traces"`
      ParamSet map[string][]string `json:"param_set"`
      Commits  []*Commit           `json:"commits"`

      // What is the scale of this Tile, i.e. it contains every Nth point, where
      // N=const.TILE_SCALE^Scale.
      Scale     int `json:"scale"`
      TileIndex int `json:"tileIndex"`
    }

Where `PerfTrace` and `GoldenTrace` implement the `Trace` interface.

Requirements
============

In the following list you may substitute 'branch' for 'trybot'.

1. Build a tile of the last N commits from master. (Our only usage today.)
2. Build a Tile for a trybot.
3. Build a Tile for a single trybot result vs a specific commit.
4. Build a Tile for all commits to master in a given time range. (Be able to go back in time for either Gold or Perf.)
5. Build a Tile for all commits to all branches in a given time range. (Show how all branches compare against main.)
6. Build a Tile for all commits to main and a given branch for a given time range. (See how a single branch compares to main.)

Assumptions
===========

1. We will use queries to the BoltDB to build in-memory Tiles.
2. We can extract a timestamp from Reitveld for each patch.

Design
======

To actually handle this in BoltDB we will need to create two buckets, one
for the per-commit values in each trace, and another for the trace-level
information, such as the params for each trace.

commit bucket
-------------

The keys for the commit bucket are structured as:

    [timestamp]:[git hash]:[branch name]:[trace_key]

and the keys map to a single value []byte, that is either the Gold digest or
the Perf float64 measurement value.

Note that to search through a time range for a specific branch name we'll need
to do the filtering inside the closure we pass to BoltDB.

trace bucket
------------

The keys for the trace bucket are just the trace keys.

    [trace_key]

The values are structs serialized as JSON that contain the params for each
trace. We are using JSON over GOB since these are relatively small structs.

Interface
---------

The interface to tracedb looks like:

    // DB represents the interface to any datastore for perf and gold results.
    //
    // Notes:
    // 1. If 'sources' is an empty slice it will match all sources.
    // 2. The Commits in the Tile will only contain the commit id and
    //    the timestamp, the Author will not be populated.
    // 3. The Tile's Scale and TileIndex will be set to 0.
    //
    type DB interface {
        // Add new information to the datastore.
        //
        // source - Either a branch name or a Rietveld issue id.
        // values - maps the trace id to a DBEntry.
        //
        // Note that only allowing adding data for a single commit at a time
        // should work well with ingestion while still breaking up writes into
        // shorter actions.
        Add(commitID *CommitID, source string, values map[string]*DBEntry) error

        // Create a Tile based on the given query parameters.
        //
        // If 'sources' is an empty slice it will match all sources.
        //
        // Note that the Commits in the Tile will only contain the commit id and
        // the timestamp, the Author will not be populated.
        TileFromRangeAndSources(begin, end time.Time, sources []string) (*tiling.Tile, error)

        // Create a Tile for the given commit ids. Commits should be provided in
        // time order.
        //
        // Note that the Commits in the Tile will only contain the commit id and
        // the timestamp, the Author will not be populated.
        TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, error)
    }

The above interface depends on the CommitID struct, which is:

    // CommitID represents the time of a particular commit, where a commit could either be
    // a real commit into the repo, or an event like running a trybot.
    type CommitID struct {
      Timestamp time.Time
      ID        string // Normally a git hash, but could also be Rietveld issue id + patch id.
    }

    func (c *CommitID) String() string {
      return fmt.Sprintf("%s%s", c.Timestamp.Format(time.RFC3339), c.ID)
    }

And DBEntry, which is:

    // DBEntry holds the params and a value for single measurement.
    type DBEntry struct {
      Params map[string]string

      // Value is the value of the measurement.
      //
      // It should be the digest string converted to a []byte, or a float64
      // converted to a little endian []byte. I.e. tiling.Trace.SetAt
      // should be able to consume this value.
      Value []byte
    }

Note that this will require adding a new method to the Trace interface:

    // Sets the value of the measurement at index.
    //
    // Each specialization will convert []byte to the correct type.
    SetAt(index int, value []byte) error

Usage
=====

Here is how the single TileFromRangeAndSources can be used to satisfy all the above requirements:

1. Build a tile of the last N commits from master.
  * Find the ~Nth commit via gitinfo, along with its timestamp. Then call

      TileFromRangeAndSources(nth.Timestamp, head.Timestamp, []string{"master"})

2. Build a Tile for a trybot.
  * Find the Reitveld issue id and created time of each patchset. Use the
    patchset ids and created timestamps to create a slice of CommitID's to use
    in:

      TileFromCommits(commits)

    or if you know the timestamp when the issue was created:

      TileFromRangeAndSources(created.Timestamp, time.Now(), []string{"[codereview id]"})

3. Build a Tile for a single trybot result vs a specific commit.
  * Find the Reitveld issue id and created time of the patchset. Find the
    commitid of the target commit:

      TileFromCommits([]*CommitID{trybot, commit})

4. Build a Tile for all commits to master in a given time range. (Be able to go back in time for either Gold or Perf).
  * Given the time range:

      TileFromRangeAndSources(beginTimestamp, endTimestamp, []string{"master"})

5. Build a Tile for all commits to all branches in a given time range. (Show how all branches compare against main).
  * Given the time range, the empty slice for source means include all sources:

      TileFromRangeAndSources(beginTimestamp, endTimestamp, []string{})

6. Build a Tile for all commits to main and a given branch for a given time range. (See how a single branch compares to main).
  * Find the ~Nth commit via gitinfo. Then call:

      TileFromRangeAndSources(nth.Timestamp, head.Timestamp, []string{"master", "[codereview id]"})

    Note that this might return multiple tries, i.e. one for each patchset.
