A BigTable backed tracedb for Perf
==================================

Table
-----

There is a single table per Perf instance. Each table that contains Traces, OrderedParamSets, and the Source locations.

Values used in row names:

    TileKey = 2^22 - (tile number)
       - With 256 values per tile this let's us store 1 billion values per trace.
       - Formatted as %07d
       - Note that this reverses the order of the tiles, i.e. new tiles have
         smaller numbers, so that we can do a simple query to find the newest tile.
    TraceKey = OrderedParamSet.EncodeParamsAsString()
       - A structured key using just the offsets, e.g. ",0=1,2=102,3=1,"
    Shard = A number, calculated from the TraceKey, that places
         the trace in one of the shards. The total number of shards
         is set per table.

Rows, Column Families, and Columns
----------------------------------

traces:
   - row name = Shard:TileKey:TraceKey

    V - Column family stores float32 values
      - Columns: 0, 1, 2, ..., TILE_SIZE-1
    S - Column family stores md5 sum of the source location, written as []byte.
        Look up the actual value of the source under the H column family.
      - Columns: 0, 1, 2, ..., TILE_SIZE-1

ops:
   - row name = '@' + TileKey

    D - Column family stores OrderedParamSets.
      - Columns: R   - Revision (hash of stored OPS to avoid the lost update problem).
                 OPS - The serialized Ordered ParamSet

hashes:
   - row name = '&' + md5('gs://...')
     The md5 name of the full source file location.

    H - Column family stores md5 hash of source file name written as hex string.
      - Columns: S   - Source (The full name of the source file, gs://....)

Commands
--------

     cbt createtable skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1

Read all the OPS hashes from the android table in the perf-bt instance.

     cbt --instance perf-bt read android prefix=@ columns=D:H

