Storing Gold traces in BigTable
===============================

This implementation is based on the original btts.go made for Perf.
See perf/BIGTABLE.md for an overview of that schema.

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

It's tempting to just put the commit hashes as columns, the trace ids as the row keys and store the
commits in the cells, but this isn't ideal for 5 performance reasons:

  1. traceIDs get really long with lots of params - BT has a limit of 4kb per row key.
  2. digests are long strings that are repeated a lot - if we stored them as int64s, it would save
     a lot of data per request (32 bytes per cell -> 8 bytes per cell).
  3. BT has a soft limit of 100Mb of data per row, and when it does a fetch, it fetches all columns
     in a row. We only really want to fetch the last few hundred commits (the last tile or two), so
     we need to wrap the data for a trace across many rows.
  4. We want to split our rows up further into shards, so we can execute multiple queries at once
     (one query per shard).

To address these performance issues, we need to store our traces and some auxiliary data.

  1. An OrderedParamSet that can convert tiling.TraceId (long string) <->
     EncodedTraceId (short string).
  2. A DigestMap that can convert types.Digest (string) <-> DigestID (int64).
  3. A monotonically increasing counter to derive more DigestIDs.

These 3 "tables" along with the traces will be stored using 4 Column Families and can
logically thought of being stored as independent "tables" or "spreadsheets" even
though they are all stored in the "one big table".

Row naming scheme
-----------------
There is a row naming scheme (for all 4 tables) as follows:

    [shard]:[namespace]:[type]:[tile]:[key]

Where shard and key are optional (can be "").
namespace is constant for all tracestore data: "ts". Reminder that there's one table per Gold
instance, so if we store more data to BT (e.g. expectations, tryjobs, etc) we need to have such
a namespace.
tile is a 0 padded 32 bit number, which for tile 0 (the oldest commits), would
manifest as "2147483646". (tile 0 => key 2^32-1 => 2147483646)

BigTable can easily fetch rows starting with a given prefix, so this naming schema
is set up to request things of a type for one or more tiles, with optional sharding.

Note that sharding is a thing we have enabled by our choice of row names, not something
given out for free by BigTable.

Gold (and perf) shards its data based on the based on the key (essentially key % 32)
This makes the workload be spread more evenly, even when fetching only one tile.
The shards come first in the naming convention to try to spread the rows across multiple
tablets for better performance (rows on BT are stored sorted by row key).

Storing the OrderedParamSet for traceIDs
----------------------------------------
As mentioned above, traceIDs are derived from the paramtools.Params map of key-value pairs.
We compress these maps using a paramstool.OrderedParamSet which concretely look like:

    ,0=1,4=2,3=1,

To do this compression/decompression, we need to store the OrderedParamSet (OPS).
Conceptually, this is stored like:
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
let "shard" and "key" both be "".
Thus, the actual row for tile 0 would be

    :ts:o:2147483646:


Storing the Digest Map
----------------------
We need to store a (potentially large) map of types.Digest (string) <-> DigestID (int64).
This map is global across tiles. Conceptually, it is stored like:
```
        | [digest1] | [digest2] | [digest3] | ...
====================================================
map_000 |  [blank]  |  [int64]  |  [blank]  |
map_001 |  [int64]  |  [blank]  |  [blank]  |
...
map_ffe |  [blank]  |  [blank]  |  [blank]  |
map_fff |  [blank]  |  [blank]  |  [int64]  |
map_000 |  [blank]  |  [int64]  |  [blank]  |
...
```

Basically, we take a digest, chop off the first three characters, use those as
the "key" in the row so we can make sure our rows don't exceed the maximum size.
Given the soft row limit of 100Mb, a digest column + a cell is at most 40 bytes,
which means a soft limit of 2.5 million digests per row. Splitting on 3 hex
characters means we have 4096-way splitting, so a soft limit of 10 billion digests.

Given that we define "type" for the digest map to be "d", an
example row for digest "92eb5ffee6ae2fec3ad71c777531578f"
(assume this key is tied to shard 19) would be:

    19:ts:d:0000000000:92e

The ids of digests out start at 0 (for MISSING_DIGEST aka "") and increase monotonically.
We manage these ids ourselves because using the autogenerated ids can lead to issues when
migrating data from one table to another.

Storing the Digest ID counter
-----------------------------
We assign newly seen digests an id that is simply an increasing int64. To store
this int64 in BT, we essentially have a single cell dedicated to this:

```
               |    idc   |
====================================================
digest_counter |  [int64] |
```
Even though the digest maps are tile-by-tile, we have one global id counter
to make sure that if we merge two tiles together, we don't have any id conflicts.

There's only one row for the counter, which is:

    :ts:i:0000000000:

Storing the traces
------------------

With all the auxiliary data set, we can look at how the traces themselves are stored.
Going with the default tile size of 256, the data would be like:

```
             | offset0 | offset1 | ... | offset255
====================================================
tile0_trace0 | [dig_1] | [dig_1] | ... | [dig_2] |
tile0_trace1 | [dig_1] | [dig_2] | ... | [dig_2] |
...
tile0_traceN | [dig_8] | [blank] | ... | [dig_6] |
tile1_trace0 | [blank] | [dig_2] | ... | [blank] |
...
```

The columns are the offset into a tile of a commit. For example, the third commit in a repo would
end up in tile 0, offset 3. The 1000th commit with a tile of 256 would be in tile 3 (1000 / 256),
offset 232 (1000 % 256). This has the effect of wrapping a given trace across many tiles.
The row keys are based off the encoded trace ids (which correspond to a ParamSet).
The cells are the int64 ids of the digest that were drawn according to that ParamSet.

The rows follow the standard naming scheme, using "t" as "type", and making use of the shards
(32 by default). The value for "key" is the encoded ParamSet (and from this key a shard is derived).
An example row for encoded ParamSet ",0=1,1=3,3=0," on tile 0 (assume shard calculates
to be 7) would be:

    07:ts:t:2147483646:,0=1,1=3,3=0,

Blank cells will be read as id 0, which is hard-coded to belong to MISSING_DIGEST ("").
