/*
 The ptracestore module handles storing Perf data in a fast and efficient
 manner.

 Data storage is broken up into tiles, i.e. the data for every 50 commits are
 stored in their own BoltDB database. The database for each tile is structured
 as:

    Bucket     | Key              | Value
   ------------+------------------+-----------------------
    traces     | traceid          | [index, float32]*
   ------------+------------------+-----------------------
    sources    | traceid          | [index, sourceIndex]*
   ------------+------------------+-----------------------
    sourceList | sourceIndex      | sourceFullname
   ------------+------------------+-----------------------

  The keys for 'traces' and 'sources' are structured keys, see the go/query package
  for more details.

  For trace data we store each point as a pair, the index of the point
  and then the value of the trace at that point. That is, if the tile
  size is 50 then each point in a trace is at an index in [0, 49]. So
  the values stored for a trace might look like:

    [0, 1.23], [1, 3.21], [2, 5.67], ...

  Note that the points may not arrive in order, so they could actually be stored
  as:

    [2, 5.67], [0, 1.23], [1, 3.21], ...

  Also note that points are only appended, and the last value for a point
  is the one that's used, so duplicate data may exist in the trace:

    [2, 5.67], [0, 1.23], [1, 3.21], [2, 5.50], ...

  This can happen if a test is re-run, we always use the latter value, so the
  value at index 2 of this trace will be 5.50, not 5.67.

  The storage in 'sources' is the same, but the pairs are an index and then
  the sourceIndex, which is an int.

  Instead of storing the optional params in the tile they will be ignored,
  instead the URL of the source of the data, i.e. the Google Storage URL to the
  ingested JSON file, will be stored in the tile, that way all of the
  information about a specific point can be retrieved, including build #.

  To make storing the source name more efficient, it will be stored in
  sourceList and map to an incrementing integer identifier for that bucket, see
  Bucket.NextSequence(). It's that unique identifier that will be stored in the
  sources bucket.

  So to get the details about commit #3 in the ',arch=x86,config=565,' trace we
  load:

     sources[',arch=x86,config=565,']

  And then decode all the [index, sourceIndex] values stored there, finding the final
  pair with an index of 2. For example:

    [2, 132]

  Then we look up the source name in the 'sourceList' bucket:

    sourceList(132)

  The largest sourceIndex used is stored at the key 'lastSourceIndex' and is incremented
  when new sourceFullname's are added.
*/
package ptracestore
