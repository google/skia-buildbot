Storing Gold traces in BigTable
===============================

This implementation is based on the original btts.go made for Perf.
See perf/BIGTABLE.md for an overview of that schema.

Note that we assume our Big Table instance is strongly consistent, which is
only valid if there is no replication (which we shouldn't need for now).

Data we want to store
---------------------

We need to store traces, which are arranged in a table.
Traces have commits as their columns, a tiling.traceID as their rows, and
a digest as their cells. These traceIDs are derived from the key:value pairs
associated with the digest, e.g. {"os":"Android", "gpu": "NVidia1080"}

Gold has an idea of a tile, which is a collection of N of these trace columns (i.e. commits).


Mapping the data to BigTable
----------------------------

BigTable (BT) stores its data as one big table. Concretely, we have

```
     | col_1  | col_2  | col_3  | ...
==================================
row_1| [data] | [data] | [data] | ...
row_2| [data] | [data] | [data] | ...
...
```

For performance, BT provides [Column Families](https://cloud.google.com/bigtable/docs/schema-design#column_families_and_column_qualifiers)
which group columns together so you can query some cells from a row belonging
to that family w/o getting everything. As an analogy, if the whole table is a Sheets
workbook, a column family can be thought of as a single spreadsheet in that workbook (at least,
for our purposes).

BT uses the term "Column Qualifier", which essentially just means "column name", because they
can be arbitrary strings.

It's tempting to just put the commit hashes as columns, the trace ids as the row names and store the
digests in the cells, but this isn't ideal for these reasons:

  1. traceIDs get really long with lots of params - BT has a limit of 4kb per
     row name (aka a row key).
  2. BT has a soft limit of 100Mb of data per row.
  3. BT will perform better if each row contains only the columns we are interested in
    reading. Since we only really want to fetch the last few hundred commits, we would
    like to limit each row to contain enough data for a single tile.
    See also https://cloud.google.com/bigtable/docs/performance#slower-perf
  4. We want to split our rows up further into shards, so we can execute multiple queries at once
     (one query per shard).

To address these performance issues, we need to store our traces and some auxiliary data:
  - an OrderedParamSet (OPS for short) that can convert tiling.TraceId (long string)
    to/from EncodedTraceId (short string).
  - The Options key/values that are a part of the trace, but don't affect the traceID.

These data, along with the traces, will be stored using 3 Column Families and can
logically thought of being stored as independent "tables" or "spreadsheets" even
though they are all stored in the "one big table".

Row naming scheme
-----------------
There is a row naming scheme (for all 4 tables) as follows:

    [shard]:[namespace]:[type]:[tile]:[subkey]

Where shard and subkey are optional (can be ""). Some tables have tile with a constant value.
"namespace" is constant for all tracestore data: "ts". Reminder that there's one table per Gold
instance, so if we store other data to BT (e.g. expectations, tryjobs, etc) we need to have
several unique namespaces.

tile is a 0 padded 32 bit number (2^32-1) - [tile number].
For example, tile 0 (the oldest commits) is number `2147483646`.
Note that this reverses the order of the tiles, i.e. new tiles have
smaller numbers, so that we can do a simple query to find the newest tile.

BigTable can easily fetch rows starting with a given prefix, so this naming schema
is set up to request things of a type for one or more tiles, with optional sharding.

Note that sharding is a thing we have enabled by our choice of row names, not something
given out for free by BigTable.

Gold (and Perf) shards its data based on the subkey (conceptually subkey % 32)
This makes the workload be spread more evenly, even when fetching only one tile.
The shards come first in the naming convention to try to spread the rows across multiple
tablets for better performance (rows on BT are stored on tablets sorted by the entire row name).

Storing the OrderedParamSet for traceIDs
----------------------------------------
As mentioned above, traceIDs are derived from the paramtools.Params map of key-value pairs.
We compress these maps using a paramstool.OrderedParamSet which concretely look like:

    ,0=1,4=2,3=1,

To do this compression/decompression, we need to store the OrderedParamSet (OPS).
There is one OPS per tile. Conceptually, an OPS is stored like:
```
          |   OPS   |    H   |
==============================
ops_tile0 | [bytes] | [hash] |
ops_tile1 | [bytes] | [hash] |
...
```

The bytes stored in under the "OPS" column are just a gob encoded OrderedParamSet and
the hash stored under the "H" column is the hash of the OPS, used to query the row when updating.

As mentioned before, ops_tile0 is a conceptual simplification of the actual
row name. Given the row naming schema, we define "type" for OPS to be "o" and
let "shard" and "subkey" both be "".
Thus, the actual row for tile 0 would be

    :ts:o:2147483646:

Storing the traces
------------------

With all the auxiliary data set, we can look at how the traces themselves are stored.
Going with the default tile size of 256, the data would be like:

```
             | offset000 | offset001 | ... | offset255
====================================================
tile0_trace0 |  [dig_1]  |  [dig_1]  | ... | [dig_2]
tile0_trace1 |  [dig_1]  |  [dig_2]  | ... | [dig_2]
...
tile0_traceN |  [dig_8]  |  [blank]  | ... | [dig_6]
tile1_trace0 |  [blank]  |  [dig_2]  | ... | [blank]
...
```

The columns are the offset into a tile of a commit. For example, the third commit in a repo would
end up in tile 0, offset 3. The 1000th commit with a tile size of 256 would be in
tile 3 (1000 / 256), offset 232 (1000 % 256). This has the effect of wrapping a given
trace across many tiles. The column values are zero padded so they sort lexicographically.

The rows follow the standard naming scheme, using "t" as "type", and making use of the shards
(32 by default). The value for "subkey" is the encoded ParamSet (and from this subkey a shard
is derived). An example row for encoded ParamSet ",0=1,1=3,3=0," on tile 0 (assume shard
calculates to be 7) would be:

    07:ts:t:2147483646:,0=1,1=3,3=0,

The cells are the 16 bytes of the digest that were drawn according to that ParamSet.
Blank cells will be decoded to MISSING_DIGEST ("") [We will sometimes represent blank cells with
a single null byte].

Storing the trace Options
----------------------------------------
Options are small map[string]string. Since Options are submitted per entry
(i.e. alongside a digest), we store them, but only care about the latest one (most recent commit).

```
             |     B
==========================
tile0_trace0 |  [bytes]
tile0_trace1 |  [blank]
...
tile0_traceN |  [bytes]
tile1_trace0 |  [blank]
...
```

We store the maps as strings, encoded like trace keys (`,key1=value1,key2=value2,`) which can use
less disk (since GOB needs to specify the type of the encoded object) and allows us to intern the
option maps, using less RAM on decoding.

The rows have the same names as their trace counterparts, with the exception of the type
(and column family). We only read the most recent cell value, which corresponds to the latest
commit. There is a garbage collection policy that will clean up older cell values over time.

An example row for encoded ParamSet ",0=1,1=3,3=0," on tile 0 (assume shard
calculates to be 7) would be:

    07:ts:p:2147483646:,0=1,1=3,3=0,