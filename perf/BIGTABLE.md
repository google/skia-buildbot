A BigTable backed tracedb for Perf
==================================

    TileKey = 2^22 - (tile number) (With 256 values per tile this let's us store 1 billion values per trace.)
        Tile 0 is 4194304
        Formatted as %07d
    TraceKey = string from Ordered ParamSet 0,1,1,4,2,3
    TILE_SIZE = 256
    Full TraceKey = TileKey:TraceKey, e.g. 4194304:0,1,1,4,2,3

Tables
------

traces:
   - row key = TileKey:TraceKey
   - One table per corpus, e.g. 'skia', 'android', 'lottie-ci', etc.

ops:
   - row key = TileKey
   - One table for OPS per corpus, e.g. 'skia-ops', 'android-ops', 'lottie-ci-ops', etc.
     Do a limit 1 search on this table to find the most recent tile.

Column Families and Columns
---------------------------

traces:

    V - Values
      - Column: V - []byte - Pairs of Varint encoded (offset, float32) pairs.
    S - Source locations
      - Column: 0, 1, 2, ..., TILE_SIZE-1

ops:

    D - Ordered ParamSet
      - Columns: H   - MD5 hash of the current GoB contents of OPS.
                 OPS - The GoB serialized Ordered ParamSet

Commands
--------

     cbt createtable skia-traces families=V:maxversions=1,S:maxversions=1
     cbt createtable skia-ops    families=D:maxversions=1
