# Fuchsia to Skia Perf JSON Converter

This tool converts JSON output from Fuchsia performance tests into a format compatible with the Skia Performance ingestion pipeline.

## Temporary Location

This tool is temporarily located in the Skia Buildbot repository (`perf/go/fuchsia_to_skia_perf`) for development and testing purposes. It will eventually be moved to the Fuchsia repository.

## Usage

### Running the tool

To convert a Fuchsia JSON file, run the following command from the root of the Skia Buildbot repository:

```bash
go run perf/go/fuchsia_to_skia_perf/main.go -input <path/to/input.json> -output <path/to/output.json>
```

Replace `<path/to/input.json>` with the path to the Fuchsia JSON file and `<path/to/output.json>` with the desired path for the converted Skia Perf JSON file.

### Testing the tool

To run the tests for the conversion library:

```bash
go test perf/go/fuchsia_to_skia_perf/*_test.go
```
