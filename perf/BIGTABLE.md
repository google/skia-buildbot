# A BigTable backed tracedb for Perf

## Table

There is a single table per Perf instance. Each table that contains Traces, OrderedParamSets, and the Source locations.

## Commands

     cbt createtable skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1,I:maxversions=1

Read all the OPS hashes from the android table in the perf-bt instance.

     cbt --instance perf-bt read android prefix=@ columns=D:H


## Design

Traces are stored by breaking them across Tiles. The size of each Tile will
vary depending on considerations like how sparse the data is, for example they
may vary from 50 to 8000 commits per Tile.


~~~~
              Tile 1                  Tile 2          
                                                      
      +---------------------+ +---------------------+ 
      |                     | |                     | 
      |                     | |                     | 
      | ===== trace 1 ===== | | ===== trace 1 ===== | 
      |                     | |                     | 
      | ===== trace 2 ===== | | ===== trace 2 ===== | 
      |                     | |                     | 
      | ===== trace 3 ===== | | ===== trace 3 ===== | 
      |                     | |                     | 
      |       ...           | |       ...           | 
      |                     | |                     | 
      |                     | |                     | 
      +---------------------+ +---------------------+ 
~~~~

Caveats our design has to take into account:

  * Traces may come and go.
  * The set of params, and the number of them, can vary across traces.
  * The Params used to specify tests are set outside of Perf and can change at any time.
  * Data may be sparse, i.e. a sample point may only arrive every 100 commits.
  * Key and values for params can be very long. E.g.

     name=AndroidCodec_122224874ic_lockscreen_emergencycall_pressed.png_SampleSize2

Traces are identified by their params, i.e. a trace key looks like:

     ,config=8888,cpu=x86,

The number of keys and values may change across tests. See infra/go/query for
more information on structured keys.

Since each Tile contains traces for a limited time range, i.e. for only a
finite set of commits, then we can track the full set of all keys and params
seen across all traces in that tile. That data is stored in an
OrderedParamSet. See infra/go/paramtools for more info on OrderedParamSets.

Since we have an OrderedParamSet (OPS) for each Tile we can use that to compress
trace ids, storing just the offsets in the OPS. I.e. these keys:

     ,config=8888,cpu=x86,name=AndroidCodec_122224874ic_lockscreen_emergencycall_pressed.png_SampleSize2,
     ,config=8888,cpu=arm,source_type=skp,name=Clear-Complex

Could be stored as:

     ,0=1,2=0,3=7,
     ,0=1,2=1,3=18,5=1,

Note that since each OPS for each Tile is different the encoding of the same
key may change from Tile to Tile.

To make querying faster we store indices for each Tile. They are reverse
lookup tables that allow finding every trace id that matches a given
param (key=value) pair.

     tile1:config:8888  => ,0=1,2=0,3=7,
                           ,0=1,2=1,3=18,5=1,

To query for config=8888 we just load the indices for tile1:config:8888 and
then load the traces for all the trace ids we find there. More complex queries
require loading more indices then taking intersections and unions of the sets
of trace ids they contain.

Finally when we write data we also store the Google Cloud Storage filename
where the data came from for each point.

To store all of this in a single BigTable table we have different kinds of
rows we store and they use different column families.

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

### Rows, Column Families, and Columns

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

indices
   - row name = 'i' + TileKey:ParamKey:ParamValue

       I - Column family stores row names of traceIds that match the given ParamKey:ParamValue.
         - Columns: The column names are the trace row names. They have the empty byte slice as a value.

hashes:
   - row name = '&' + md5('gs://...')
     The md5 name of the full source file location.

    H - Column family stores md5 hash of source file name written as hex string.
      - Columns: S   - Source (The full name of the source file, gs://....)

