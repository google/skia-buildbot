# Fuchsia to Skia Perf JSON Converter

This tool converts JSON output from Fuchsia performance tests into a format compatible with the Skia Performance ingestion pipeline.

## Temporary Location

This tool is temporarily located in the Skia Buildbot repository (`perf/go/fuchsia_to_skia_perf`) for development and testing purposes. It will eventually be moved to the Fuchsia repository.

## Usage

### Running the tool

To convert a Fuchsia JSON file, run the following command from the root of the Skia Buildbot repository:

```bash
go run ./perf/go/fuchsia_to_skia_perf/main.go \
  -input <path/to/input.json> \
  -output <path/to/output_dir/> \
  -master <master_name>
```

- Replace `<path/to/input.json>` with the path to the Fuchsia JSON file.
- Replace `<path/to/output_dir/>` with the desired directory for the output files.
- Replace `<master_name>` with the appropriate master name (e.g., "turquoise-internal.integration.global.ci").

The tool will generate multiple JSON files in the specified output directory, named according to the pattern: `<build_id>-<benchmark>-<bot>-<master>.json`.

### Testing the tool

To run the tests for the conversion library:

```bash
go test ./perf/go/fuchsia_to_skia_perf/convert/...
```
