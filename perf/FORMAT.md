# The Skia Perf Format

The Skia Perf format is a JSON file that contains measurements. For example:

```
{
  "version": 1,
  "git_hash": "cd5...663",
  "key": {
      "config": "8888",
      "arch": "x86"
  },
  "results": [
      {
          "key": {
              "test": "some_test_name"
          },
          "measurements": {
              "ms": [
                  {
                      "value": "min",
                      "measurement": 1.2
                  },
                  {
                      "value": "max",
                      "measurement": 2.4
                  },
                  {
                      "value": "median",
                      "measurement": 1.5
                  }
              ]
          }
      }
  ]
}
```

The format is documented
[here](https://pkg.go.dev/go.skia.org/infra/perf/go/ingest/format?tab=doc#Format).

# Storage

Each Perf data file should be stored in Google Cloud Storage in a location of
the following format:

    gs://<bucket>/<one or more dir names>/YYYY/MM/DD/HH/<zero or more dir names><some unique name>.json

Where:

    YYYY - Year
    MM - Month, e.g. 02 for February.
    DD - Day, e.g.  01, 02, etc.
    HH - Hour in 24 hour format, e.g. 00, 01, 02, ..., 22, 23

in UTC.

## Example

    gs://skia-perf/nano-json-v1/2018/08/23/22/Android-Clang/7989dad6c3b2efc10defb8f280f7a8a1a731d5d0.json

The Perf ingester will attempt to ingest all files below /HH/ that end in
`.json`. Nothing about the file location or the file name is ingested as data.

## Validation

You can validate your files conform to the expected schema by installing
`perf-tool`:

    go install go.skia.org/infra/perf/go/perf-tool@latest

And then use it to validate an ingestion file:

    perf-tool ingest validate --in=my-ingestion-file.json

If the format is invalid the tool will print out validation errors, for example

    $ perf-tool ingest validate --in=$HOME/invalid.json
    0 - 0: (root): results is required

    Error: Validation Failed: schema violation

If the format is valid then all the found trace ids and their values
will be printed out, for example:

    $ perf-tool ingest validate --in=$HOME/valid.json
    hash: def820637c3674a2bcecd6992a207fa3b9bed737
    ,arch=arm64,radius=10,test=drawCircle,units=ms, = 1.4800247
    ,arch=arm64,radius=50,test=drawCircle,units=ms, = 261.58847
    ,arch=arm64,radius=90,test=drawCircle,units=ms, = 377.3585

You can run `perf-tool` over
[`./go/ingest/parser/testdata/version_1/success.json`](//perf/go/ingest/parser/testdata/version_1/success.json)
to see how it converts all the keys and values in that file into trace identifiers.

# Notes

- Perf only uses the data in the file, and does not parse the GCS file location
  to get data.
- The YYYY/MM/DD/HH should represent the time the file was written, not the time
  the data was gathered.
- Perf is robust to duplicate data, i.e. a file written at a later time can
  contain data that will replace data that has appeared in an older file. Where
  'older' and 'newer' are defined in terms of the data/time in the GCS file
  path.
- See
  [IngestionConfig](https://pkg.go.dev/go.skia.org/infra/perf/go/config?tab=doc#IngestionConfig)
  for configuring the ingestion of new data.
