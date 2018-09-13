A BigTable backed tracedb for Perf
==================================

    TileKey = 2^22 - (tile number) (With 256 values per tile this let's us store 1 billion values per trace.)
        Formatted as %07d
    TraceKey = []byte from Ordered ParamSet
    TILE_SIZE = 256

Table
-----

There is a single table that contains both traces and ops.

traces:
   - row name = TileKey:TraceKey
   - One table per corpus, e.g. 'skia', 'android', 'lottie-ci', etc.

ops:
   - row name = '@' +  TileKey

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

     cbt createtable skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1


Notes
-----

cbt bug: cbt tries to shell out to gcloud config config-helper, which tries to
refresh the token, which fails if the token is expired and you have not
internet connectivity, all of this is done before BIGTABLE_EMULATOR_HOST
is checked to figure out that creds aren't needed.

A workaround is to supply a phony -creds command line flag value.


