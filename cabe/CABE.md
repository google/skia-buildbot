# NAME

cabe - Command-line tool for working with cabe

# SYNOPSIS

cabe

```
[--help|-h]
[--logging]
```

# DESCRIPTION

cli tools for analyzing and debugging pinpoint A/B experiment tryobs using cabe

**Usage**:

```
cabe [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--help, -h**: show help

**--logging**: Turn on logging while running commands.

# COMMANDS

## analyze

analyze runs the analyzer process locally and pretty-prints a table of results to stdout.

**--benchmark**="": name of benchmark to analyze

**--pinpoint-job**="": ID of the pinpoint job to check

**--record-to-zip**="": Zip file to save replay data to

**--replay-from-zip**="": Zip file to replay data from

**--workload**="": comma separated list of names of benchmark workloads to analyze

## check

prints diagnostic results, and an inferred experiment spec if none was specified

**--benchmark**="": name of benchmark to analyze

**--pinpoint-job**="": ID of the pinpoint job to check

**--record-to-zip**="": Zip file to save replay data to

**--replay-from-zip**="": Zip file to replay data from

**--workload**="": comma separated list of names of benchmark workloads to analyze

## readcas

readcas reads perf results json data from RBE-CAS, located using the provided root-digest.

**--benchmark**="": name of benchmark to analyze

**--cas-instance**="": cas instance (default: projects/chrome-swarming/instances/default_instance)

**--pinpoint-job**="": ID of the pinpoint job to check

**--record-to-zip**="": Zip file to save replay data to

**--replay-from-zip**="": Zip file to replay data from

**--root-digest**="": CAS digest for the root node

**--workload**="": comma separated list of names of benchmark workloads to analyze

## sandwich

sandwich executes a verification workflow based on an existing pinpoint bisect job.

**--attempts**="": iterations verification job will run (default: 30)

**--benchmark**="": name of benchmark to analyze

**--dry-run**: dry run for StartWorkflow; just print CreateExecutionRequest to stdout

**--execution**="": execution id of the workflow

**--location**="": location for workflow execution (default: us-central1)

**--pinpoint-job**="": ID of the pinpoint job to check

**--project**="": GAE project app name (default: chromeperf)

**--record-to-zip**="": Zip file to save replay data to

**--replay-from-zip**="": Zip file to replay data from

**--workflow**="": name of workflow to execute (default: sandwich-verification-workflow-prod)

**--workload**="": comma separated list of names of benchmark workloads to analyze

## markdown

Generates markdown help for cabe.

## help, h

Shows a list of commands or help for one command
