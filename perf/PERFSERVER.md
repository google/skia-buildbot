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

**--commit_range_url**="": A URI Usage: Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.

**--config_filename**="": The name of the config file to use. (default: ./configs/nano.json)

**--connection_string**="": Override Usage: the connection_string in the config file.

**--default_sparse**: The default value for 'Sparse' in Alerts.

**--disable_git_update**: Disables updating of the git repository

**--disable_metrics_update**: Disables updating of the database metrics

**--display_group_by**: Show the Group By section of Alert configuration.

**--do_clustering**: If true then run continuous clustering over all the alerts.

**--event_driven_regression_detection**: If true then regression detection is done based on PubSub events.

**--feedback_url**="": Feedback Url to display on the page

**--fetch_chrome_perf_anomalies**: Fetch anomalies and show the bisect button

**--hide_list_of_commits_on_explore**: Hide the commit-detail-panel-sk element on the Explore details tab.

**--interesting**="": The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements. (default: 50)

**--internal_port**="": HTTP service address for internal clients, e.g. probers. No authentication on this port. (default: :9000)

**--key_order**="": The order that keys should be presented in for searching. All keys that don't appear here will appear after. (default: build_flavor,name,sub_result,source_type)

**--local**: Running locally if true. As opposed to in production.

**--noemail**: Do not send emails.

**--num_continuous**="": The number of commits to do continuous clustering over looking for regressions. (default: 50)

**--num_continuous_parallel**="": The number of parallel copies of continuous clustering to run. (default: 3)

**--num_paramsets_for_queries**="": The number of Tiles to look backwards over when building a ParamSet that
is used to present to users for them to build queries.

This number needs to be large enough to hit enough Tiles so that no query
parameters go missing.

For example, let's say "test=foo" only runs once a week, but let's say
the incoming data fills one Tile per day, then you'd need
num_paramsets_for_queries to be at least 7, otherwise "foo" might not
show up as a query option in the UI for the "test" key.
(default: 2)

**--num_shift**="": The number of commits the shift navigation buttons should jump. (default: 10)

**--port**="": HTTP service address (e.g., ':8000') (default: :8000)

**--prom_port**="": Metrics service address (e.g., ':10110') (default: :20000)

**--radius**="": The number of commits to include on either side of a commit when clustering. (default: 7)

**--resources_dir**="": The directory to find templates, JS, and CSS files. If blank then ../../dist relative to the current directory will be used.

**--step_up_only**: Only regressions that look like a step up will be reported.

## maintenance

Starts maintenance tasks.

**--config_filename**="": Instance config file. Must be supplied.

**--connection_string**="": Override the connection_string in the config file.

**--local**: True if running locally and not in production.

**--prom_port**="": Metrics service address (e.g., ':20000') (default: :20000)

## ingest

Run the ingestion process.

**--config_filename**="": Instance config file. Must be supplied.

**--connection_string**="": Override the connection_string in the config file.

**--local**: True if running locally and not in production.

**--num_parallel_ingesters**="": The number of parallel Go routines to have ingesting. (default: 10)

**--prom_port**="": Metrics service address (e.g., ':20000') (default: :20000)

## cluster

Run the regression detection process.

**--commit_range_url**="": A URI Usage: Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.

**--config_filename**="": The name of the config file to use. (default: ./configs/nano.json)

**--connection_string**="": Override Usage: the connection_string in the config file.

**--default_sparse**: The default value for 'Sparse' in Alerts.

**--disable_git_update**: Disables updating of the git repository

**--disable_metrics_update**: Disables updating of the database metrics

**--display_group_by**: Show the Group By section of Alert configuration.

**--do_clustering**: If true then run continuous clustering over all the alerts.

**--event_driven_regression_detection**: If true then regression detection is done based on PubSub events.

**--feedback_url**="": Feedback Url to display on the page

**--fetch_chrome_perf_anomalies**: Fetch anomalies and show the bisect button

**--hide_list_of_commits_on_explore**: Hide the commit-detail-panel-sk element on the Explore details tab.

**--interesting**="": The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements. (default: 50)

**--internal_port**="": HTTP service address for internal clients, e.g. probers. No authentication on this port. (default: :9000)

**--key_order**="": The order that keys should be presented in for searching. All keys that don't appear here will appear after. (default: build_flavor,name,sub_result,source_type)

**--local**: Running locally if true. As opposed to in production.

**--noemail**: Do not send emails.

**--num_continuous**="": The number of commits to do continuous clustering over looking for regressions. (default: 50)

**--num_continuous_parallel**="": The number of parallel copies of continuous clustering to run. (default: 3)

**--num_paramsets_for_queries**="": The number of Tiles to look backwards over when building a ParamSet that
is used to present to users for them to build queries.

This number needs to be large enough to hit enough Tiles so that no query
parameters go missing.

For example, let's say "test=foo" only runs once a week, but let's say
the incoming data fills one Tile per day, then you'd need
num_paramsets_for_queries to be at least 7, otherwise "foo" might not
show up as a query option in the UI for the "test" key.
(default: 2)

**--num_shift**="": The number of commits the shift navigation buttons should jump. (default: 10)

**--port**="": HTTP service address (e.g., ':8000') (default: :8000)

**--prom_port**="": Metrics service address (e.g., ':10110') (default: :20000)

**--radius**="": The number of commits to include on either side of a commit when clustering. (default: 7)

**--resources_dir**="": The directory to find templates, JS, and CSS files. If blank then ../../dist relative to the current directory will be used.

**--step_up_only**: Only regressions that look like a step up will be reported.

## markdown

Generates markdown help for perfserver.

**--help, -h**: show help

## help, h

Shows a list of commands or help for one command
