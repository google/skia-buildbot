This directory contains the coverage service implementation and it's respective controllers.

The service has 4 endpoints.

1. GetTestSuite
2. InsertFile
3. DeleteFile
4. UpdateFile

# Run CockroachDB

`cd /tmp && cockroach start-single-node --insecure --listen-addr=127.0.0.1`

# Running the coverage service locally

To run a local instance of the coverage service,
simply run the cmd below from the [coverage](../../)
directory.
`make run-coverage`

# How to use the endpoints

`grpc_cli call 127.0.0.1:8006 --channel_creds_type=local `
`coverage.v1.CoverageService.<RPC> --infile demo/<ACTION>.json`

1. GetTestSuite
   - This method returns available test suites based on Source file and Builder.
2. InsertFile
   - Used when a new file and builder will be added to database.
3. DeleteFile
   - Remove existing file and builder pair.
4. UpdateFile
   - Add test suite to existing file and builder pair.
