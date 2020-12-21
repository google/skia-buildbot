# NAME

perf-tool - Command-line tool for working with Perf data.

# SYNOPSIS

perf-tool

```
[--help|-h]
[--logging]
```

**Usage**:

```
perf-tool [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--help, -h**: show help

**--logging**: Turn on logging while running commands.


# COMMANDS

## config



### create-pubsub-topics

Create PubSub topics for the given big_table_config.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

## tiles



### last

Prints the index of the last (most recent) tile.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

### list

Prints the last N tiles and the number of traces they contain.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

**--num**="": The number of tiles to display. (default: 10)

## traces



### list

Prints the IDs of traces in the last (most recent) tile, or the tile specified by the --tile flag, that match --query.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

**--query**="": The query to run.

**--tile**="": The tile to query. (default: -1)

### export

Writes a JSON files with the traces that match --query for the given range of commits.

**--begin**="": The commit number to start loading data from. Inclusive. (default: -1)

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--end**="": The commit number to load data to. (default: -1)

**--local**: If true then use gcloud credentials.

**--out**="": The output filename.

**--query**="": The query to run.

## ingest



### force-reingest



**--config_filename**="": Load configuration from `FILE`

**--dryrun**: Just display the list of files to send.

**--local**: If true then use gcloud credentials.

**--start**="": Start the ingestion at this time, of the form: 2006-01-02. Default to one week ago.

**--stop**="": Ingest up to this time, of the form: 2006-01-02. Default to now.

## database



### migrate

Migrate the database to the latest version of the schema.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

### backup



#### alerts



**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

**--out**="": The output filename.

#### shortcuts



**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

**--out**="": The output filename.

#### regressions



**--backup_to_date**="": How far back in time to back up Regressions. Defaults to four weeks.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--local**: If true then use gcloud credentials.

**--out**="": The output filename.

### restore



#### alerts



**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--in**="": The input filename.

**--local**: If true then use gcloud credentials.

#### shortcuts



**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--in**="": The input filename.

**--local**: If true then use gcloud credentials.

#### regressions



**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--in**="": The input filename.

**--local**: If true then use gcloud credentials.

## trybot



### reference

Generates a reference file to be used by nanostat for the given trybot file.

**--config_filename**="": Load configuration from `FILE`

**--connection_string**="": Override the connection string in the config file.

**--filename**="": The full URL of a nanobench trybot results files, e.g.: 'gs://skia-perf/...foo.json'

**--local**: If true then use gcloud credentials.

**--num**="": The number of ingestion files to load. (default: 5)

**--out**="": The output filename.

## markdown

Generates markdown help for perf-tool.

**--help, -h**: show help

## help, h

Shows a list of commands or help for one command

