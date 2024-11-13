The Spanner integration is a work in progress and is only partially supported for now.

# Running the Spanner Emulator

For local runs we can run a local instance of the spanner emulator alongside pgadapter
(which provides a postgresql interface to spanner). This combination is already packaged as a
docker image, so the most convenient way is to execute it via docker.

    make run-spanner-emulator

# Running a local instance against the Spanner Emulator

    make run-demo-instance-spanner

**Pro tip:** You can do both the things above with

    make run-spanner-emulator run-demo-instance-spanner

# Currently supporting

- Starting a demo FE server by populating the demo data.
- Querying traces where the query is an exact match to the trace.

# Known Issues

- Partial query match needs some more investigation. This is likely due to the change from BYTES
  data type in CDB to BYTEA in Spanner.
