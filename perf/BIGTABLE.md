A BigTable backed tracedb for Perf
==================================

    TileKey = 2^22 - (tile number) (With 256 values per tile this let's us store 1 billion values per trace.)
        Formatted as %07d
    TraceKey = []byte from Ordered ParamSet
    TILE_SIZE = 256

Tables
------

traces:
   - index = TileKey:TraceKey
   - One table per corpus, e.g. 'skia', 'android', 'lottie-ci', etc.

ops:
   - index = TileKey
   - One table for OPS per corpus, e.g. 'skia-ops', 'android-ops', 'lottie-ci-ops', etc.
     Do a limit 1 search on this table to find the most recent tile.

Column Families and Columns
---------------------------

traces:

    V - float32 values
      - Columns: 0, 1, 2, ..., TILE_SIZE-1
    S - gs://... source locations
      - Columns: 0, 1, 2, ..., TILE_SIZE-1

ops:

    D - Ordered ParamSet
      - Columns: R   - Revision
                 OPS - The serialized Ordered ParamSet

Commands
--------

     cbt createtable skia-traces families=V:maxversions=1,S:maxversions=1
     cbt createtable skia-ops    families=D:maxversions=1
