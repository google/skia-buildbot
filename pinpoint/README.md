# Pinpoint Developer Documentation

This developer guide details how to run, execute, query, and debug Go-native
**Pairwise Jobs** (Skia's TryJob equivalent) programmatically and locally.

---

## 1. Run a Local Temporal Dev Server

Temporal orchestrates Pinpoint's build, Swarming, and statistical workflows.

### Step 1. Install & Start Temporal

If Temporal CLI is not installed, run the official installer script on your terminal:

```bash
curl -sSf https://temporal.download/cli.sh | sh
```

_Ensure Temporal is in your PATH: `echo export PATH="\$PATH:\$HOME/.temporalio/bin" >> ~/.bashrc`._

Launch the local Temporal dev server:

```bash
temporal server start-dev
```

_(Optionally pass `--db-filename=temporal-db.db` if you need persistent workflow runs)._

- **Temporal Server Port:** `localhost:7233`
- **Temporal Web UI:** Accessible at [http://localhost:8233](http://localhost:8233).

### Step 2. Create the `perf-internal` Namespace

By default, the Go service runs within the `perf-internal` namespace. You must
register it before scheduling workflows:

```bash
temporal operator namespace create perf-internal
```

---

## 2. Run the Local Go Pinpoint Service Server

To serve the REST HTTP JSON gateways, start the Pinpoint backend server locally
on port `8080`:

```bash
bazelisk run //pinpoint/go/frontend/cmd:cmd -- --port=:8080
```

- **Local REST API Endpoint:** `http://localhost:8080`

---

## 3. Run a Local Temporal Worker

The worker polls the server for queued tasks and executes the actual benchmark
workflows.

Start the worker pointing to the shared `perf.perf-chrome-public.bisect` task queue:

```bash
bazelisk run //pinpoint/go/workflows/worker -- \
  --taskQueue=perf.perf-chrome-public.bisect \
  --namespace=perf-internal \
  --local
```

_(The `--local` flag enables local/development fail-fast bypass of GCP Spanner
writes to facilitate smooth local testing)._

---

## 4. Trigger a Pairwise A/B Job Natively

### Approach A: Via cURL REST HTTP API

Make a `POST` request directly to the local Go Pinpoint API endpoint:

```bash
curl -i -X POST "http://localhost:8080/pinpoint/v1/pairwise" \
  -H "Content-Type: application/json" \
  -d '{
    "start_commit": {
      "main": {
        "repository": "https://chromium.googlesource.com/chromium/src.git",
        "git_hash": "b2d27b144e4e4c5661bafc08f7b8494797f6ee1a"
      }
    },
    "end_commit": {
      "main": {
        "repository": "https://chromium.googlesource.com/chromium/src.git",
        "git_hash": "95b3180e9724995eb6d5a85ac3c93140e4506f7e"
      }
    },
    "bot_name": "win-11-perf",
    "benchmark": "speedometer3",
    "story": "Speedometer3",
    "initial_attempt_count": "30",
    "improvement_dir": "Unknown",
    "user_email": "your_email@google.com",
    "job_name": "Programmatic Local Pairwise Run"
  }'
```

_Successfully returns a JSON payload containing your `"jobId"` (the Temporal
Workflow UUID)._

### Approach B: Via Bazel CLI Sample Utility

Alternatively, you can bypass HTTP APIs entirely and trigger workflows
directly using the Go sample tool:

```bash
bazelisk run //pinpoint/go/workflows/sample -- \
  --taskQueue=perf.perf-chrome-public.bisect \
  --namespace=perf-internal \
  --pairwise \
  --configuration=win-11-perf \
  --benchmark=speedometer3 \
  --story=Speedometer3 \
  --start-git-hash=b2d27b144e4e4c5661bafc08f7b8494797f6ee1a \
  --end-git-hash=95b3180e9724995eb6d5a85ac3c93140e4506f7e
```

---

## 5. Query Job Status & View Results

Once triggered, you can monitor progress and fetch results using three methods:

### A. Via Temporal Web UI (Visual Logs)

Open your local browser and navigate to: **[http://localhost:8233](http://localhost:8233)**.
_Note: If the server is running on a remote Cloudtop VM, you may need to
access this through Chrome Remote Desktop (**go/crd**) or set up port
forwarding._

- Select the **`perf-internal`** namespace from the top-left dropdown.
- Click on your workflow `[jobId]` to view the real-time execution graph,
  pending build or Swarming tasks, and full payloads.

### B. Via Temporal CLI (Command Line)

Fetch the active workflow progress and state details natively in your terminal:

```bash
temporal workflow show \
  -w [your-job-id] \
  -n perf-internal
```

### C. Via Go Pinpoint REST Query API

Once the bisection workflow completes, retrieve the statistical results programmatically:

```bash
curl -i -X GET "http://localhost:8080/pinpoint/v1/query?job_id=[your-job-id]"
```

### D. Via Go Pinpoint REST QueryPairwise API (New)

Once a **Pairwise/Try Job** completes, retrieve the Wilcoxon Signed-Rank
test statistical results, control/treatment medians, p-values, and all
swarming task IDs:

```bash
curl -i -X GET "http://localhost:8080/pinpoint/v1/query-pairwise?job_id=[your-job-id]"
```
