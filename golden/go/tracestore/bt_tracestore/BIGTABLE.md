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

Gold has an idea of a tile, which is a collection of N of these trace rows.

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
workbook, a column family can be thought of as a singe spreadsheet in that workbook (at least,
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

As mentioned above, traceIDs are derived from the paramtools.Params map of key-value pairs.
We compress these maps using a paramstool.OrderedParamSet which concretely look like:
`,0=1,4=2,3=1,`

To do this compression/decompression, we need to store the OrderedParamSet (OPS).
Conceptually, this is stored like:
```
           |   OPS   |    H   |
==============================
ops_tile_0 | [bytes] | [hash] |
ops_tile_1 | [bytes] | [hash] |
...
```

The bytes stored in under the "OPS" column are just a gob encoded OrderedParamSet and
the hash stored under the "H" column is the hash of the OPS, used to query the row when updating.

ops_tile_0 isn't really what the row looks like in practice.
There is a row naming scheme as follows:

	[shard]:[namespace]:[type]:[tile]:[key]

Where shard and key are optional (can be ""). For our OrderedParamSet, they are indeed "".
namespace is constant for all tracestore data: "ts". Reminder that there's one table per Gold
instance, so if we store more data to BT (e.g. expectations, tryjobs, etc) we need to have such
a namespace.

type for OPS is arbitrarily defined to be "o". tile is a 0 padded 32 bit number, so for the OPS,
the actual row for tile 0 (key 2^32-1 => 2147483646) would be

	:ts:o:2147483646:


