% stagemanager 8

# NAME

stagemanager - stagemanager <subcommand>

# SYNOPSIS

stagemanager

```
[--help|-h]
```

**Usage**:

```
stagemanager [GLOBAL OPTIONS] command [COMMAND OPTIONS] [ARGUMENTS...]
```

# GLOBAL OPTIONS

**--help, -h**: show help

# COMMANDS

## promote

promote <image path> <stage to match> <stage to update>

## apply

apply

## images

images <subcommand>

### add

add <image path> [non-default git repo]

### rm

rm <image path>

## stages

stages <subcommand>

### set

set <image path> <stage name> <git revision | digest>

### rm

rm <image path> <stage name>

## markdown

Generates markdown help for stagemanager.

**--help, -h**: show help

## help, h

Shows a list of commands or help for one command
