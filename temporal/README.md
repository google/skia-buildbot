# Temporal

[Temporal](https://github.com/temporalio/temporal) is an execution service,
serving jobs orchestration, [see alternatives](go/qa-alter).

# Docker Images

The binaries are being built in go/louhi and the source codes are pulled from the
individual releases.

## Temporal Server [Releases](https://github.com/temporalio/temporal/releases)

There are three binaries:

- temporal-server: the main server binary
- temporal-sql-too: the database tool to initialize and upgrade schemas
- tdbg: the debugging utils

## Temporal CLI [Releases](https://github.com/temporalio/cli/releases)

The CLI tool to admin the service:

`bazelisk run //temporal:temporal-cli --`

This assumes the service is running locally at port 7233.

## Temporal UI Server [Release](https://github.com/temporalio/ui-server/releases)

The Web UI frontend to inspect the service.
