# NAME

perfserver - Command line tool that runs the various components of Perf.

# SYNOPSIS

perfserver

```
[--help|-h]
```

**Usage**:

```
perfserver [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--help, -h**: show help


# COMMANDS

## frontend

The main web UI.

**--auth_bypass_list**="": Space separated list of email addresses allowed access. Usually just service account emails. Bypasses the domain checks.

**--commit_range_url**="": A URI Usage: Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.

**--config_filename**="": The name of the config file to use. (default: ./configs/nano.json)

**--connection_string**="":  Override Usage: the connection_string in the config file.

**--default_sparse**: The default value for 'Sparse' in Alerts.

**--do_clustering**: If true then run continuous clustering over all the alerts.

**--email_client_secret_file**="": OAuth client secret JSON file for sending email. (default: client_secret.json)

**--email_token_cache_file**="": OAuth token cache file for sending email. (default: client_token.json)

**--event_driven_regression_detection**: If true then regression detection is done based on PubSub events.

**--interesting**="": The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements. (default: 50.000000)

**--internal_only**: Require the user to be logged in to see any page.

**--internal_port**="": HTTP service address for internal clients, e.g. probers. No authentication on this port. (default: :9000)

**--key_order**="": The order that keys should be presented in for searching. All keys that don't appear here will appear after. (default: build_flavor,name,sub_result,source_type)

**--local**: Running locally if true. As opposed to in production.

**--noemail**: Do not send emails.

**--num_continuous**="": The number of commits to do continuous clustering over looking for regressions. (default: 50)

**--num_continuous_parallel**="": The number of parallel copies of continuous clustering to run. (default: 3)

**--num_shift**="": The number of commits the shift navigation buttons should jump. (default: 10)

**--port**="": HTTP service address (e.g., ':8000') (default: :8000)

**--prom_port**="": Metrics service address (e.g., ':10110') (default: :20000)

**--radius**="": The number of commits to include on either side of a commit when clustering. (default: 7)

**--step_up_only**: Only regressions that look like a step up will be reported.

## ingest

Run the ingestion process.

**--config_filename**="": Instance config file. Must be supplied.

**--connection_string**="":  Override the connection_string in the config file.

**--local**: True if running locally and not in production.

**--num_parallel_ingesters**="": The number of parallel Go routines to have ingesting. (default: 10)

**--prom_port**="": Metrics service address (e.g., ':20000') (default: :20000)

## cluster

Run the regression detection process.

**--auth_bypass_list**="": Space separated list of email addresses allowed access. Usually just service account emails. Bypasses the domain checks.

**--commit_range_url**="": A URI Usage: Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.

**--config_filename**="": The name of the config file to use. (default: ./configs/nano.json)

**--connection_string**="":  Override Usage: the connection_string in the config file.

**--default_sparse**: The default value for 'Sparse' in Alerts.

**--do_clustering**: If true then run continuous clustering over all the alerts.

**--email_client_secret_file**="": OAuth client secret JSON file for sending email. (default: client_secret.json)

**--email_token_cache_file**="": OAuth token cache file for sending email. (default: client_token.json)

**--event_driven_regression_detection**: If true then regression detection is done based on PubSub events.

**--interesting**="": The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements. (default: 50.000000)

**--internal_only**: Require the user to be logged in to see any page.

**--internal_port**="": HTTP service address for internal clients, e.g. probers. No authentication on this port. (default: :9000)

**--key_order**="": The order that keys should be presented in for searching. All keys that don't appear here will appear after. (default: build_flavor,name,sub_result,source_type)

**--local**: Running locally if true. As opposed to in production.

**--noemail**: Do not send emails.

**--num_continuous**="": The number of commits to do continuous clustering over looking for regressions. (default: 50)

**--num_continuous_parallel**="": The number of parallel copies of continuous clustering to run. (default: 3)

**--num_shift**="": The number of commits the shift navigation buttons should jump. (default: 10)

**--port**="": HTTP service address (e.g., ':8000') (default: :8000)

**--prom_port**="": Metrics service address (e.g., ':10110') (default: :20000)

**--radius**="": The number of commits to include on either side of a commit when clustering. (default: 7)

**--step_up_only**: Only regressions that look like a step up will be reported.

## markdown

Generates markdown help for perfserver.

**--help, -h**: show help

## help, h

Shows a list of commands or help for one command

