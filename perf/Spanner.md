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

# Set up Production Instance

This step is only when we are creating a new instance (equivalent of setting up an entirely
new database service deployment). For majority of the cases, you can use an existing instance
and create new databases within the instance.

Instance creation is handled by Terraform. You can update the following files to create new
instances.

**skia-infra-public**: https://source.corp.google.com/piper///depot/google3/configs/cloud/gong/services/skia_infra/skia_infra_public/spanner.tf

**skia-infra-corp**: https://source.corp.google.com/piper///depot/google3/configs/cloud/gong/services/skia_infra/skia_infra_corp/spanner.tf

# Create a new Database

The easiest option to create a new database is via the cloud console. Navigate to the spanner
instance in the GCP project and use the "Create Database" option.

Note that you may need elevated permissions in order to create a database. Run the following
command to get the necessary access.

    grants add --wait_for_twosync --reason="b/377530262 -- <Reason for elevating>" skia-infra-breakglass-policy:2h

# Run Queries

All the team members have been given read permissions for the spanner databases. You can run
the queries in Spanner studio available in the GCP console.

# Run a local instance against Production Database (Readonly)

The spanner instances have been configured to provide readonly access to the developers working
on perf. If you want to run a local FE instance against the production spanner database, run the
following command from the `perf` directory.

    ./run_with_spanner.sh p=<gcp project> i=<spanner instance id> d=<database name> config=<path to perf instance config file>

The values for the arguments can be found in the relevant config file under
`//perf/configs/spanner/*.json`.

You may get authentication errors during server startup. PGAdapter uses the GCP credentials on
your machine to authenticate with Spanner. Refresh your credentials with the following commands.

    gcloud config set project <gcp project>
    gcloud auth application-default login

Follow the instructions printed in these commands to refresh the local credentials and then
run the `./run_with_spanner.sh` command again.
