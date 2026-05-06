# Anomaly Backfill Listener

This package implements the PubSub listener for event-driven anomaly backfilling.

## Overview

The `Listener` waits for messages on a configured PubSub topic.
Each message contains a `BackfillRequest` with a `RequestID`, an `AlertID`
and an `End` timestamp (which we consider as the processing date).

**Note on the `End` timestamp:**
The anomaly detection logic will load the latest `N` commits (where `N` is determined by the `NumContinuous` flag) ending at or before the specified `End` timestamp. We do not specify the number of commits `N` in the request because commit density can vary, and it is safer to rely on the standard configured number of commits to ensure we cover the expected range for detection.

When a message is received, the listener:

1.  Loads the alert configuration.
2.  Depending on the request:
    - **Trace ID filtering mode**: If a list of `TraceIDs` is provided in the request, it filters out traces that do not match the alert's query and only processes the matching ones.
    - **Chunked mode** (default): Queries for all trace IDs matching the alert and processes them in batches.
    - **All-traces mode** (if `LoadAllTracesTogether` is true): Loads and processes all traces in a single data frame.
3.  Runs the regression detection pipeline for the specified time range.
4.  Saves any found regressions to the database. Notifications are sent only
    if `SendNotifications` is true in the request; otherwise, it only saves
    the data.

## Configuration

To enable the backfill listener, you must configure the following in your instance configuration:

```json
{
  "anomaly_config": {
    "backfill_topic_name": "your-topic-name",
    "backfill_concurrency": 2
  }
}
```

## Usage

The listener is started from `Continuous.Run()` if `backfill_topic_name` is set.

## Example: Publishing a Backfill Request

You can use the [trigger_backfill.sh](buildbot/perf/go/regression/continuous/backfill/trigger_backfill.sh) script to trigger a backfill manually by publishing to the Pub/Sub topic.

> [!NOTE]
> The script will iterate through the date range (from `<start_date>` to `<finish_date>`)
> and publish a separate Pub/Sub message for each day and each alert_id.

Usage:

```bash
./trigger_backfill.sh [-n] [-a] [-i <request_id>] [-f <trace_file>] <project> <topic> <start_date> <finish_date> <alert_id1> [<alert_id2> ...]
```

Options:

- `-n`: Enable sending notifications (default: false).
- `-a`: Load all traces together (default: false).
- `-i <request_id>`: Use a custom request ID (default: auto-generated UUID).
- `-f <trace_file>`: File containing trace IDs (one per line). Use `-` to read from stdin.

Examples:

Standard backfill with notifications:

```bash
./trigger_backfill.sh -n skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345 67890
```

Backfill all traces together with a custom request ID:

```bash
./trigger_backfill.sh -a -i "my-custom-batch-id" skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345 67890
```

Backfill specific traces from a file:

```bash
./trigger_backfill.sh -f traces.txt skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345
```

Backfill specific traces via pipe:

```bash
cat traces.txt | ./trigger_backfill.sh -f - skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345
```

## Manual Backfill Procedure

To perform a backfill, follow these steps:

> [!IMPORTANT]
> To have access to write data into the topic, you need to use breakglass.
> Please refer to the [instructions][breakglass-doc] for details.

### 1. Identify Alerts

Find the IDs of the alerts you want to backfill by querying the `alerts` table in Spanner.

### 2. Run the Script

Use the `trigger_backfill.sh` script to publish requests for the desired date range and alert IDs.

```bash
./trigger_backfill.sh [-n] [-a] [-i <request_id>] [-f <trace_file>] <project> <topic> <start_date> <finish_date> <alert_id1> [<alert_id2> ...]
```

#### Example backfill request:

```bash
./trigger_backfill.sh -a -n -i "my-custom-batch-id" skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345 67890
```

#### Example backfill for specific traces:

```bash
./trigger_backfill.sh -f traces.txt skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345
```

#### Example backfill for specific traces via pipe:

```bash
cat traces.txt | ./trigger_backfill.sh -f - skia-public perf-anomaly-backfill-v8-perf-autopush 2026-01-01 2026-01-05 12345
```

### 3. Monitor Metrics

Monitor the following metrics to track the progress and health of the backfill process:

- `perf_backfill_runs_total`: Total number of backfill requests processed.
- `perf_backfill_success_total`: Number of successful backfill requests.
- `perf_backfill_failure_total`: Number of requests that failed with fatal errors (e.g., missing alert).
- `perf_backfill_errors_total`: Number of requests that encountered non-fatal errors and will be retried.
- **Oldest Unacknowledged Message Age**: Monitor this Pub/Sub subscription metric (available in Cloud Monitoring). If it is growing continuously, it indicates that the application might be stuck or failing to process messages successfully.

### 4. Error Handling and Poison Pills

- **Non-existent Alert IDs**: If a non-existent alert ID is provided, the application will not loop infinitely. It will recognize that the alert is missing, log a failure, and acknowledge (ack) the message so it is removed from the queue.
- **Transient Errors**: Other types of errors (e.g., database timeouts) will cause the message to be returned (nack'd) to the subscription, triggering retries until it completes without errors.
- **Processing Timeout**: There is a hardcoded timeout of 10 minutes for processing each backfill request. If processing takes longer than this, the operation will be cancelled, the message will be returned (nack'd) to the subscription, and it will be retried.
- **Stuck Processing**: In rare cases where the application freezes or gets stuck processing a message that continuously fails, you may need to manually delete or purge messages from the Pub/Sub subscription to unblock the queue.
- **Log Monitoring**: Check the application logs for the following key phrases:
  - `"Failed to process backfill message"`: Indicates a message processing failure. Look at the associated error details.
  - `"Acking unprocessable message"`: Indicates a message was dropped (acked) because it cannot be processed (e.g., invalid alert ID) to prevent infinite retries.

[breakglass-doc]: https://docs.google.com/document/d/1fnOywuK9BUkmHhOnx0vZL-VYGSr6Paw7uo4wRSK-KA8/edit?tab=t.wu1v2mrm3345#bookmark=id.1y870hrocaye
