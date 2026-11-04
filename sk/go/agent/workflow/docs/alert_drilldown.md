# Skia Infrastructure Alert Drilldown

This document provides instructions for deeply investigating a specific alert
(or a bundle of related alerts) from Alert Manager (`am.skia.org`) to determine
its root cause.

Because alert investigations follow a branching tree of possibilities depending
on what each step reveals, start with the **Initial Assessment** below and then
choose the relevant **Investigation Sub-Tasks** as needed.

---

## Initial Assessment (Starting Point)

1. **Retrieve Full Alert Details (`get_alert_details`):**
   - Run `sk agent tool get_alert_details --alert_id=<id>`. (If you only have an
     alert name or target name, first locate its ID using
     `sk agent tool get_active_alerts`).
   - Extract the key context from the output:
     - **`description` & Links:** Read the diagnostic message, PromQL expression
       (`link_to_source`), log/pod URLs, and any `PROD.md` playbook links.
     - **Target Identifiers:** Note `project_id`, `cluster`,
       `namespace` / `exported_namespace`, `pod` / `exported_pod`,
       `container` / `exported_container`, `app` / `exported_app`, `bot`,
       `job_name`, `repo`, etc. (Remember that when `exported_*` labels are
       present, they identify the actual target resource, whereas unprefixed
       labels identify the checker service that emitted the metric.)
     - **Timing & Cadence (`Started`, `Duration`, `Recent History`):** Check
       whether the alert is continuously active or oscillating at a regular
       interval.
   - **Check Multiple Targets in a Bundle:** Even when several instances fire
     the same `AlertName` (or pair of alerts), they may have different underlying
     causes. Inspect logs/details for each distinct target in the bundle rather
     than assuming the first target explains all of them.

2. **Consult Local Playbooks (`PROD.md`) & Service Code:**
   - Many alert descriptions link to playbooks in the `buildbot` repository
     (e.g., `https://skia.googlesource.com/buildbot/+/main/<path>`).
   - Read the corresponding `<path>` directly from a local `buildbot` checkout
     (for example, `autoroll/PROD.md`, `k8s-checker/PROD.md`,
     `task_scheduler/PROD.md`, `golden/docs/PROD.md`, `perf/PROD.md`) and check
     the section matching the alert anchor.
   - If the alert is emitted by or involves a Skia infrastructure service (such
     as `k8s-checker`, `k8s-deployer`, `autoroll`, `gold`, `perf`, or
     `task_scheduler`), you can also inspect that service's implementation in
     the local `buildbot` checkout to see how the metric or check is computed.

---

## Investigation Sub-Tasks

Select and combine the sub-tasks below based on what each step reveals.
Always use `curl` (rather than `read_url_content`) for any HTTP requests, as
`read_url_content` may block on interactive domain-permission prompts when
running in a sub-agent.

### Sub-Task: Sync or Reuse Local Repository Checkouts (`git`)

When your investigation requires reading playbooks, service code, Kubernetes
manifests, roller configs, CI job definitions, or commit history (`git log`,
`git blame`, `git grep`), use a local checkout for any of these four core
repositories:

- `https://skia.googlesource.com/buildbot.git`
- `https://skia.googlesource.com/skia.git`
- `https://skia.googlesource.com/skia-autoroll-internal-config.git`
- `https://skia.googlesource.com/k8s-config.git`

1. **Reuse Existing Checkouts First:**
   - Check whether the current workspace or the shared repository directory
     (`<shared_repo_dir>/<repo>`, provided by the coordinator agent or created
     in your scratch directory) already contains a checkout of the needed
     repository.
   - If the directory already exists, use it directly. (If it may be slightly
     behind `origin/main`, you can run `git -C <repo_path> fetch origin`, or
     Wait if another agent is currently cloning/fetching it.)
2. **Clone If Not Yet Present:**
   - If the repository directory does not exist yet, clone the full repository
     into `<shared_repo_dir>/<repo>`:
     ```bash
     git clone https://skia.googlesource.com/<repo>.git <shared_repo_dir>/<repo>
     ```
3. **Treat Shared Checkouts as Strictly Read-Only:**
   - Multiple investigator sub-agents may read the same local checkout
     concurrently. **Never** mutate the working tree or index (`git checkout`,
     `git switch`, `git reset`, `git clean`, or editing files).
   - Pass `--no-optional-locks -C <repo_path>` when running `git` commands so
     concurrent readers do not contend on `.git/index.lock`:
     ```bash
     git --no-optional-locks -C <repo_path> log -n 20 --oneline -- <file_path>
     git --no-optional-locks -C <repo_path> show <commit_or_ref>:<file_path>
     ```

### Sub-Task: Retrieve Container / Pod Logs (`gcloud logging read`)

To inspect logs for a Kubernetes workload, query Cloud Logging using the
target's `project_id`, `cluster`, and `container` (`exported_container`), `pod`
(`exported_pod`), or `namespace` (`exported_namespace`):

1. **Query recent `ERROR` logs first:**

   ```bash
   gcloud logging read 'resource.type="k8s_container" AND resource.labels.cluster_name="<cluster>" AND resource.labels.container_name="<container>" AND severity>=ERROR' --project=<project_id> --limit=10 --format="value(timestamp,textPayload,jsonPayload.message)"
   ```

   - If multiple pods share the same `container_name` (such as generic container
     names like `app` or `fe`), add
     `AND resource.labels.pod_name="<pod>"` or
     `AND resource.labels.namespace_name="<namespace>"`.

2. **Handle huge `sklog.Fatal` stack dumps and multi-line command outputs:**
   - **Fatal goroutine dumps (`sklog.Fatal` / `sklog.Fatalf`):** When a Skia Go
     service exits via `sklog.Fatal`, the final `CRITICAL` / `ALERT` log entry
     starts with the fatal error message followed by a stack trace for every
     goroutine. Querying many entries at once can truncate the top of the entry
     where the actual fatal message lives; use a small `--limit=2` or use
     `kubectl logs ... --tail=5 --limit-bytes=4000` (see below) to capture just
     the fatal error line.
   - **Multi-line command dumps:** When a single `ERROR` log entry contains
     hundreds of lines of command output (such as a multi-resource
     `kubectl apply` in `k8s-deployer`), filter for the failing lines or narrow
     the query with
     `AND (textPayload:("Error" OR "Failed" OR "invalid") OR jsonPayload.message:("Error" OR "Failed" OR "invalid"))`.
3. **Fetch surrounding `INFO` / `WARNING` context when needed:**
   - An `ERROR` log often states _what_ check failed, while the immediately
     preceding `INFO` or `WARNING` logs from the same tick or request record the
     intermediate state that explains _why_ it failed.
   - Run a follow-up query without `severity>=ERROR` (using a small `--limit=25`
     or a narrow `timestamp` window around an error entry) and include
     `severity` in the format:
     ```bash
     gcloud logging read 'resource.type="k8s_container" AND resource.labels.cluster_name="<cluster>" AND resource.labels.container_name="<container>"' --project=<project_id> --limit=25 --format="value(timestamp,severity,textPayload,jsonPayload.message)"
     ```

### Sub-Task: Inspect Kubernetes Workload Status (`kubectl`)

To inspect live cluster state, pod health, or workload configuration:

- **Always pass `--context` and `-n` explicitly** rather than running
  `kubectl config use-context`, which mutates global state. Place the `kubectl`
  subcommand (`get`, `describe`, `logs`) before `--context`. GKE context names
  follow the pattern `gke_<project_id>_<location>_<cluster>` (for example,
  `gke_skia-infra-public_us-central1-a_skia-infra-public`,
  `gke_skia-infra-corp_us-central1-a_skia-infra-corp`,
  `gke_skia-public_us-central1-a_skia-public`). Use
  `kubectl config get-contexts -o name` if you need to verify available
  contexts.
- **Check pod status and restart counts:**
  ```bash
  kubectl get pods --context=gke_<project_id>_<location>_<cluster> -n <namespace> -l app=<app> -o wide
  ```
- **Locate failed or evicted pods across namespaces (eg. for `EvictedPod` alerts):**
  ```bash
  kubectl get pods --context=gke_<project_id>_<location>_<cluster> --all-namespaces --field-selector=status.phase=Failed
  ```
- **Inspect container state, `Last State` exit reasons, and events:**
  ```bash
  kubectl describe pod --context=gke_<project_id>_<location>_<cluster> -n <namespace> <pod>
  ```
  Look at `State`, `Last State` (e.g., `OOMKilled`, `Error`, exit code,
  termination timestamp), `Ready`, and the `Events` table at the bottom (e.g.,
  `FailedScheduling`, `BackOff` pulling image, failed liveness/readiness
  probes).
- **Retrieve live or previous container logs:**

  ```bash
  kubectl logs --context=gke_<project_id>_<location>_<cluster> -n <namespace> <pod> -c <container> --tail=100
  kubectl logs --context=gke_<project_id>_<location>_<cluster> -n <namespace> <pod> -c <container> --previous --tail=100
  ```

  - **Tip for `sklog.Fatal` crashes:** `sklog.Fatal` dumps stack traces for all
    goroutines at the end of the log, which can flood your context window or
    push the fatal error line hundreds of lines above the end of the log (since
    `jcgregorio/logger` prefixes every line of the goroutine dump with `F`).
    - If the container crashed during startup, omit `--tail` and pass
      `--previous --limit-bytes=6000` to read from the start of the log through
      the initial `F...` fatal error message:
      ```bash
      kubectl logs --context=gke_<project_id>_<location>_<cluster> -n <namespace> <pod> -c <container> --previous --limit-bytes=6000
      ```
    - If the container ran for a while before crashing, pass
      `--tail=5 --limit-bytes=4000` (with `--previous` if restarted); if that
      only shows the bottom of a multi-line goroutine dump, increase `--tail`
      (e.g., `--tail=500 --limit-bytes=30000`) to capture the first `F...` line
      at the start of the dump.

- **Inspect the owning Deployment or StatefulSet spec:**
  ```bash
  kubectl get deployment,statefulset --context=gke_<project_id>_<location>_<cluster> -n <namespace> <app> -o yaml
  ```
  Use this to check the live container image digest, command-line flags (which
  may include inline or base64-encoded configuration), environment variables,
  volume mounts, or resource requests/limits. Checked-in Kubernetes manifests
  live in `k8s-config.git` under `<cluster>/` and are applied to the cluster by
  `k8s-deployer`.
- **Inspect Managed Prometheus `ClusterRules` (for `Absent` or alert rule checks):**
  ```bash
  kubectl get clusterrules.monitoring.googleapis.com --context=gke_<project_id>_<location>_<cluster> -o yaml
  ```

### Sub-Task: Query Prometheus Metrics (`prom2.skia.org`)

When an alert is driven by an aggregated rate, liveness metric, or threshold
(such as `HighFirestoreUsageSustained`, `SAKeyExpired`, `Absent`, or error rate
alerts), you can run instant PromQL queries directly against
`https://prom2.skia.org/api/v1/query` to break down a metric by label (e.g., by
`package`, `op`, `path`, or `key_id`) or check whether a renamed metric exists:

1. Obtain an OAuth2 access token:
   ```bash
   gcloud auth application-default print-access-token
   ```
2. Query `prom2.skia.org`:
   ```bash
   curl -G -s -H "Authorization: Bearer <token>" --data-urlencode 'query=<promql_expression>' "https://prom2.skia.org/api/v1/query"
   ```

### Sub-Task: Inspect CI Jobs, Tasks, and Swarming Bots

For Task Scheduler and Swarming alerts (`OverdueJobSpec`,
`TaskSchedulerTooManyCandidates`, `CQTrybotRunningTooLong`, `BotMissing`,
`BotQuarantined`, `BotUnemployed`):

1. **Inspect Job Definitions and Dimension Mappings in `skia.git`:**
   - Check `infra/bots/jobs.json`, `infra/bots/tasks.json`, and
     `infra/bots/gen_tasks_logic/gen_tasks_logic.go` in the local `skia`
     checkout to see the exact Swarming dimensions (`gpu`, `os`, `cpu`,
     `device_type`, `device_os`, `pool`) required by the job.
2. **Check Task Health and Failure Logs via `sk agent tool`:**
   - Run `sk agent tool search_tasks` (use `sk agent tool search_tasks --help`
     to see all filter flags) to list recent task executions
     and their Swarming task IDs, statuses (`FAILURE`, `MISHAP`, `RUNNING`), and
     durations.
   - To inspect step failures or timeouts inside a task, run
     `sk agent tool get_task_steps --task_id=<task_id>` (using the Task
     Scheduler `ID` from `search_tasks`) and then
     `sk agent tool get_recipe_step_logs --swarming_task_id=<swarming_task_id> --log_path="<log_path>"`
     (using the `swarmingTaskId` from `search_tasks`) or
     `sk agent tool get_task_driver_step_logs --task_id=<task_id> --step_id="<step_id>"`
     (see `sk agent workflow task_drilldown` for details).
3. **Query Live Swarming Bot Dimensions and State (`Bots/ListBots`):**
   - To verify how many bots in a pool match a job's dimensions—or whether the
     target bots are `dead`, `quarantined`, busy, or advertising a newer driver/OS
     dimension—query the Swarming pRPC API using an OAuth2 bearer token from
     `gcloud auth application-default print-access-token`:
     ```bash
     curl -s -X POST -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -H "Accept: application/json" "https://chromium-swarm.appspot.com/prpc/swarming.v2.Bots/ListBots" -d '{"dimensions": [{"key": "pool", "value": "Skia"}, {"key": "<dim_key>", "value": "<dim_val>"}]}'
     ```

### Sub-Task: Trace Error Locations in Local Source Code

Skia infrastructure Go services use `skerr`, which attaches compact file-and-line
stack traces to logged errors (e.g.,
`At autoroller.go:773 state_machine.go:319 ...`).

- When a log entry includes a `skerr` stack trace or a distinctive log message:
  1. Locate the file or search for the log string in the local `buildbot`
     checkout.
  2. Read the surrounding function to see the exact condition that produced the
     error and how the relevant state variables were computed.

### Sub-Task: Query Remote Gitiles Repositories and Refs

For repositories outside the four locally cloned repos above (or when checking
remote branch/tag refs on `*.googlesource.com` directly):

- **Query Gitiles endpoints directly:**
  - Append `?format=JSON` (or `&format=JSON` when other query parameters are
    present) for structured JSON metadata. Note that Gitiles prefixes JSON
    responses with a 4-character XSSI guard (`)]}'`).
  - Append `?format=TEXT` to raw file or diff URLs to retrieve base64-encoded
    text content.
- **Authenticate when required:** If an unauthenticated request returns HTTP
  `401` or `403`, attach an OAuth2 bearer token obtained via
  `gcloud auth application-default print-access-token`:
  1. Print the access token:
     ```bash
     gcloud auth application-default print-access-token
     ```
  2. Pass the token in the `Authorization` header:
     ```bash
     curl -s -H "Authorization: Bearer <token>" "<gitiles_url>?format=JSON"
     ```
- **Common Gitiles endpoints:**
  - **Refs, branches, and tags:** `<repo_url>/+refs?format=JSON` (or
    `<repo_url>/+refs/heads?format=JSON`, `<repo_url>/+refs/tags?format=JSON`).
  - **Commit history for a repo, range, or file path:**
    `<repo_url>/+log/<branch>/<path>?format=JSON&n=<limit>` or
    `<repo_url>/+log/<from>..<to>?format=JSON`.
  - **File contents or commit diffs:**
    `<repo_url>/+/<ref>/<path>?format=TEXT` or
    `<repo_url>/+/<commit>%5E%21/<path>?format=TEXT` (base64-encoded).
