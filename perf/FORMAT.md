The Skia Perf Format
====================

The Skia Perf Format is a JSON file that looks like:

```
{
    "gitHash": "fe4a4029a080bc955e9588d05a6cd9eb490845d4",
    "key": {
        "arch": "x86",
        "gpu": "GTX660",
        "model": "ShuttleA",
        "os": "Ubuntu12"
    },
    "results": {
        "ChunkAlloc_PushPop_640_480": {
            "nonrendering": {
                "min_ms": 0.01485466666666667,
                "options": {
                    "source_type": "bench"
                }
            }
        },
        "DeferredSurfaceCopy_discardable_640_480": {
            "565": {
                "min_ms": 2.215988,
                "options": {
                    "source_type": "bench"
                }
            },
    ...
```

  * gitHash - The git hash of the build this was tested at.
  * key - A map of key, value pairs that should be part of every result in the
      file. Note that 'config' and 'test' are also added to the key.
  * results - A map of test name to the tests results. Note that test name is
      part of the key and stored at the key 'test'.  Each key under the result
      is mapped to 'config' in the key.  All of the 'options' under each config
      are also added to the key.

In the above example, the key-value pairs that identify the value 2.215988
are:

        arch: x86,
        gpu: GTX660,
        model: ShuttleA,
        os: Ubuntu12
        test: DeferredSurfaceCopy_discardable_640_480
        config: 565
        sub_result: min_ms
        source_type: bench

Key value pair charactes should come from [0-9a-zA-Z\_], particularly
note no spaces or ':' characters.

Storage
=======

Each Perf data file should be stored in Google Cloud Storage in a location
of the following format:

    gs://<bucket>/<one or more dir names>/YYYY/MM/DD/HH/<zero or more dir names><some unique name>.json

Where:

    YYYY - Year
    MM - Month, e.g. 02 for February.
    DD - Day, e.g.  01, 02, etc.
    HH - Hour in 24 hour format, e.g. 00, 01, 02, ..., 22, 23

Example
-------

    gs://skia-perf/nano-json-v1/2018/08/23/22/Android-Clang/7989dad6c3b2efc10defb8f280f7a8a1a731d5d0.json

The Perf ingester will attempt to ingest all files below /HH/ that end in `.json`.
Nothing about the file location or the file name is ingested as data.

Notes
=====
  * Perf only uses the data in the file, and does not parse the GCS file location to get data.
  * The YYYY/MM/DD/HH should represent the time the file was written, not the
    time the data was gathered.
  * Perf is robust to duplicate data, i.e. a file written at a later time can
    contain data that will replace data that has appeared in an older file.
    Where 'older' and 'newer' are defined in terms of the data/time in the GCS
    file path.
  * See [./sys/perf.json5] and [./create-ingestion-pubsub-topics.sh] for configuring the
    ingestion of new data.
