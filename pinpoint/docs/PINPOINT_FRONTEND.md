# Pinpoint Frontend

This directory contains the main application for the Pinpoint frontend service.

## Running the Service

Along side the frontend service, you must run PGAdpater for a database connection in a
seperate terminal connected to the database specified in the connection string. I
recomended you download the required JAR files in a seperate directory from the Skia Buildbot.

```bash
wget https://storage.googleapis.com/pgadapter-jar-releases/pgadapter.tar.gz \
 && tar -xzvf pgadapter.tar.gz
```

```bash
sudo java -jar pgadapter.jar -p skia-infra-corp -i {Experimental Instance ID} -d \
 natnael-test-database -c /root/.config/gcloud/application_default_credentials.json -x
```

To run the service, use the following Bazel command. This will run the Pinpoint Frontend UI
Service on local port 8080 and connect to the default skia experimental instance database:

```bash
bazelisk run //pinpoint/go/frontend/cmd
```

The application supports the following command-line flags for configuration:

--port: The port to listen on for HTTP traffic. (Default: :8080)

--connection_string: The connection string for the Pinpoint backend database.
(Default: postgresql://root@localhost:5432/natnael-test-database?sslmode=disable)

--prod_assest_dir: The directory containing production assets for the UI.
(Default: pinpoint/ui/pages/production)

## Responsibilities

- Provides HTTP endpoints for listing, and viewing Pinpoint jobs.
- Serves the Pinpoint UI (landing and results pages).
- Provides configuration data like available benchmarks, bots, and stories.
- Forwards relevant requests to the underlying Pinpoint gRPC service.

## API Endpoints

The service registers the following HTTP handlers:

- `GET /json/jobs/list`: Lists jobs with support for filtering and pagination.
- `GET /json/job/{jobID}`: Retrieves detailed information for a specific job.
- `GET /benchmarks`: Returns a list of available benchmarks.
- `GET /bots`: Returns a list of available bot configurations. Can be filtered by benchmark.
- `GET /stories`: Returns a list of stories for a given benchmark.
- `GET /`: Serves the main landing page of the Pinpoint UI.
- `GET /results/jobid/{jobID}`: Serves the results page for a specific job.
- `/pinpoint/*`: Handles requests related to the core Pinpoint service, such as scheduling
  new jobs.

## Configuration: `benchmarks.json`

The `benchmarks.json` file is the service's single source of truth for the list of available
benchmarks, the bots that can run each benchmark, and the stories associated with them.

### How it Works

1.  **Structure**: The file is a JSON array where each object represents a benchmark and
    contains three keys:

        - `"benchmark"`: The name of the benchmark.
        - `"stories"`: A list of stories for that benchmark.
        - `"bots"`: A list of bots configured to run that benchmark.

2.  **Service Initialization**: On startup, the service reads and parses `benchmarks.json`
    into memory. This allows for quick access to the configuration data without needing to read
    the file on each request.

3.  **API Endpoints**: The in-memory data is used to power the following configuration endpoints:
    - `GET /benchmarks`: Returns the list of all benchmark names.
    - `GET /bots?benchmark=<name>`: Returns the list of bots for a given benchmark.
    - `GET /stories?benchmark=<name>`: Returns the list of stories for a given benchmark.
