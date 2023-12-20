#!/bin/bash

: "${DB:=postgresql}"
: "${SKIP_DB_CREATE:=false}"
: "${TEMPORAL_HOME:=/etc/temporal}"
: "${SERVICES:=}"

# PostgreSQL
: "${DBNAME:=temporal}"
: "${VISIBILITY_DBNAME:=temporal_visibility}"
: "${DB_PORT:=26257}"

: "${POSTGRES_SEEDS:=}"
: "${POSTGRES_USER:=}"
: "${POSTGRES_PWD:=}"

: "${POSTGRES_TLS_ENABLED:=false}"
: "${POSTGRES_TLS_DISABLE_HOST_VERIFICATION:=false}"
: "${POSTGRES_TLS_CERT_FILE:=}"
: "${POSTGRES_TLS_KEY_FILE:=}"
: "${POSTGRES_TLS_CA_FILE:=}"
: "${POSTGRES_TLS_SERVER_NAME:=}"

# Server setup
: "${TEMPORAL_ADDRESS:=}"
: "${SKIP_DEFAULT_NAMESPACE_CREATION:=false}"
: "${DEFAULT_NAMESPACE:=default}"
: "${DEFAULT_NAMESPACE_RETENTION:=1}"

: "${SKIP_ADD_CUSTOM_SEARCH_ATTRIBUTES:=false}"
