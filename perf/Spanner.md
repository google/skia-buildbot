The Spanner integration is a work in progress and is only partially supported for now.

# Running the Spanner Emulator

For local runs we can run a local instance of the spanner emulator alongside pgadapter
(which provides a postgresql interface to spanner). This combination is already packaged as a
docker image, so the most convenient way is to execute it via docker.

    make run-spanner-emulator

Each execution of the above cmd tears down the database and creates a new one.
Since we are running the emulator in docker, it is extremely cheap to teardown
and restart the database and not have to worry about leaving things in an inconsistent state.

If you are using `make run-demo-instance` to run a local FE instance, it will automatically
execute the `run-spanner-emulator` target thereby giving you a fresh database instance on each execution.
