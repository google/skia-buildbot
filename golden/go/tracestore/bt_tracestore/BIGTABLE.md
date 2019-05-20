Storing Gold traces in BigTable
===============================

This implementation is based on the original btts.go made for Perf.
See perf/BIGTABLE.md for an overview of that schema.

Data we want to store
---------------------

We need to store traces, which are arranged in a table.
Traces have commits as their rows, a tiling.traceID as their columns, and
a digest as their cells. These traceIDs are derived from the key:value pairs
associated with the digest, e.g. {"os":"Android", "gpu": "NVidia1080"}

Gold has an idea of a tile, which is a collection of N of these trace rows (i.e. commits).


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

It's tempting to just put the commit hashes as rows, the trace ids as the columns and store the
commits in the cells, but this isn't ideal for 4 performance reasons:

  1. traceIDs get really long with lots of params - BigTable recommends keeping column qualifiers
     short to reduce data transferred per request.
  2. digests are long strings that are repeated a lot - if we stored them as ints, it would save
     a lot of data per request.
  3. We want to make querying by tile easier, so we need to group commits into tiles, which we
     do using the commit index (i.e. this hash is the Nth commit in the repo).
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
We need to store a (potentially large) map of types.Digest (string) <-> DigestID (int64)
with there being one map per tile. Conceptually, this is stored like:
```
            | [digest1] | [digest2] | [digest3] | ...
====================================================
map_tile0_0 |  [blank]  |  [int64]  |  [blank]  |
map_tile0_1 |  [int64]  |  [blank]  |  [blank]  |
...
map_tile0_e |  [blank]  |  [blank]  |  [blank]  |
map_tile0_f |  [blank]  |  [blank]  |  [int64]  |
map_tile1_0 |  [blank]  |  [int64]  |  [blank]  |
...
```

Basically, we take a digest, chop off the first character, use that as the "key"
in the row so we can pull the table from BT using 16-way parallelization. The
remaining 31 characters of the digest are used as the column name and the
cell value is the int of the id associated with that digest.

Given that we define "type" for the digest map to be "d" and shards are "", an
example row for tile 0 and digest "92eb5ffee6ae2fec3ad71c777531578f" would be

    :ts:d:2147483646:9


Storing the Digest ID counter
-----------------------------
We assign newly seen digests an id that is simply an increasing int. To store this int in BT, we essentially have a single cell dedicated to this:

```
               |    idc   |
====================================================
digest_counter |  [int64] |
```
Even though the digest maps are tile-by-tile, we have one global id counter
to make sure that if we merge two tiles together, we don't have any id conflicts.