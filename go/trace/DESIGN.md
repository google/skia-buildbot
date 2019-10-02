tracedb
=======

The tracedb package is designed to replace the current storage system for
traces, tiles, with a new backend that allows for much more flexibility
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

1. We will use queries to the interface to build in-memory Tiles.

Design
======

The design will actually be done in two layers, tracedb.DB, which is the Go
interface for talking to the data store, and then a separate service that
implements a gRPC interface and stores the data in BoltDB.


                 +-------------+
                 | tracedb.DB  |
                 | interface   |
                 +-------------+
                        |
                        |
                        |
                 +------v------+
                 | gRPC Server |
                 | BoltDB      |
                 +-------------+


tracedb.DB Interface
--------------------

This is the Go interface to the storage for traces. The interface to tracedb looks like:

    // DB represents the interface to any datastore for perf and gold results.
    //
    // Notes:
    // 1. The Commits in the Tile will only contain the commit id and
    //    the timestamp, the Author will not be populated.
    // 2. The Tile's Scale and TileIndex will be set to 0.
    //
    type DB interface {
      // Add new information to the datastore.
      //
      // The values maps a trace id to a Entry.
      //
      // Note that only allowing adding data for a single commit at a time
      // should work well with ingestion while still breaking up writes into
      // shorter actions.
      Add(commitID *CommitID, values map[string]*Entry) error

      // List returns all the CommitID's between begin and end.
      List(begin, end time.Time) ([]*CommitID, error)

      // Create a Tile for the given commit ids. Will build the Tile using the
      // commits in the order they are provided.
      //
      // Note that the Commits in the Tile will only contain the commit id and
      // the timestamp, the Author will not be populated.
      TileFromCommits(commitIDs []*CommitID) (*tiling.Tile, error)

      // Close the datastore.
      Close() error
    }

The above interface depends on the CommitID struct, which is:

    // CommitID represents the time of a particular commit, where a commit could either be
    // a real commit into the repo, or an event like running a trybot.
    type CommitID struct {
      Timestamp time.Time
      ID        string // Normally a git hash.
      Source    string // The branch name, e.g. "master".
    }

And Entry, which is:

    // Entry holds the params and a value for single measurement.
    type Entry struct {
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


BoltDB Implementation
=====================

For local testing the Go interface above will be implemented in terms of the
gRPC interface defined below with a BoltDB store. I.e. there will be a
standalone server that implements the following gRPC interface.

The gRPC interface is similar to the Go interface, with Add and List operating
exactly the same. The only difference is in retrieving data, which means that
TileForCommits is broken down into two different calls, GetValues, and
GetParams, which the caller can use to build a Tile from.

    // TraceDB stores trace information for both Gold and Perf.
    service TraceDB {
      // Returns a list of traceids that don't have Params stored in the datastore.
      rpc MissingParams(MissingParamsRequest) returns (MissingParamsResponse) {}

      // Adds Params for a set of traceids.
      rpc AddParams(AddParamsRequest) returns (EmptyResponse) {}

      // Adds data for a set of traces for a particular commitid.
      rpc Add(AddRequest) returns (AddResponse) {}

      // Removes data for a particular commitid.
      rpc Remove(RemoveRequest) returns (EmptyResponse) {}

      // List returns all the CommitIDs that exist in the given time range.
      rpc List(ListRequest) return (ListResponse) {}

      // GetValues returns all the trace values stored for the given CommitID.
      rpc GetValues(GetValuesRequest) (GetValuesResponse)

      // GetParams returns the Params for all of the given traces.
      rpc GetParams(GetParamsRequest) (GetParamsResponse)
    }

See `go/tracedb/proto/tracestore.proto` for more details.


To actually handle this in BoltDB we will need to create three buckets, one for
the per-commit values in each trace, and another for the trace-level
information, such as the params for each trace, and a third for mapping
traceids to much shorter int64 values.

traceid bucket
--------------

To reduce the amount of data stored, we'll map traceids to 64 bit ints
and use the 64 bit ints as the keys to the maps stored in the commit
bucket. The traceid bucket maps traceids to trace64id, and vice versa.

There is a special key, "the largest trace64id", which isn't a valid traceid, which
contains the largest trace64id seen, and defaults to 0 if not set.

commit bucket
-------------

The keys for the commit bucket are structured as:

    [timestamp]![git hash]![branch name]

The key maps to a serialized values and their trace64ids. I.e. a serialized
map[uint64][]byte, where the uint64 is the trace64id.

trace bucket
------------

The keys for the trace bucket are traceids.

    [traceid]

The values are structs serialized Protocol Buffers that contain the params for
each trace and the original traceid.

constructor
-----------

    func NewTraceStoreDB(conn *grpc.ClientConn, tb tiling.TraceBuilder) (DB, error) {

Usage
=====

Here is how the single TileFromCommits can be used to satisfy all the above requirements:

1. Build a tile of the last N commits from master.
  * Find the last N commits via gitinfo, construct CommitIDs for each one, then call:

      TileFromCommits(commits)

2. Build a Tile for all commits to master in a given time range. (Be able to go back in time for either Gold or Perf).
  * Given the time range, build CommitIDs from gitinfo, then call:

      TileFromCommits(commits)

3. Build a Tile for all commits to all branches in a given time range. (Show how all branches compare against main).
  * Given the time range, call List, then TileFromCommits:

      commits, err := List(beginTimestamp, endTimestamp)
      TileFromCommits(commits)

4. Build a Tile for all commits to main and a given branch for a given time range. (See how a single branch compares to main).
  * Find the ~Nth commit via gitinfo. Then call List, filter the results, then call TileFromCommits.

      commits, err := List(beginTimestamp, endTimestamp)
      // Filter commits to only include values from the desired branches.
      TileFromCommits(commits)
