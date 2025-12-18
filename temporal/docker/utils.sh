#!/bin/bash
# Tailored version from here:
# https://github.com/temporalio/docker-builds/blob/main/docker/auto-setup.sh

set -eu -o pipefail

source /etc/temporal/env-default.sh

# === Helper functions ===

die() {
    echo "$*" 1>&2
    exit 1
}

# === Main database functions ===

validate_db_env() {
    case ${DB} in
      mysql | mysql8)
          if [[ -z ${MYSQL_SEEDS} ]]; then
              die "MYSQL_SEEDS env must be set if DB is ${DB}."
          fi
          ;;
      postgresql | postgres | postgres12)
          if [[ -z ${POSTGRES_SEEDS} ]]; then
              die "POSTGRES_SEEDS env must be set if DB is ${DB}."
          fi
          ;;
      cassandra)
          if [[ -z ${CASSANDRA_SEEDS} ]]; then
              die "CASSANDRA_SEEDS env must be set if DB is ${DB}."
          fi
          ;;
      *)
          die "Unsupported DB type: ${DB}."
          ;;
    esac
}

wait_for_postgres() {
    until nc -z "${POSTGRES_SEEDS%%,*}" "${DB_PORT}"; do
        echo 'Waiting for PostgreSQL to startup.'
        sleep 1
    done

    echo 'PostgreSQL started.'
}

setup_postgres_schema() {
    # This export might not be needed but kept the same as original version.
    export SQL_PASSWORD=${POSTGRES_PWD}

    if [[ ${DB} == "postgres12" ]]; then
      POSTGRES_VERSION_DIR=v12
    else
      POSTGRES_VERSION_DIR=v96
    fi

    SCHEMA_BASE_DIR=${TEMPORAL_HOME}/schema/postgresql/${POSTGRES_VERSION_DIR}
    SCHEMA_DIR=${SCHEMA_BASE_DIR}/temporal/versioned
    # Create database only if its name is different from the user name. Otherwise PostgreSQL
    # container itself will create database.
    if [[ ${DBNAME} != "${POSTGRES_USER}" && ${SKIP_DB_CREATE} != true ]]; then
        temporal-sql-tool \
            --plugin postgres \
            --ep "${POSTGRES_SEEDS}" \
            -u "${POSTGRES_USER}" \
            -p "${DB_PORT}" \
            --db "${DBNAME}" \
            --tls="${POSTGRES_TLS_ENABLED}" \
            --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
            --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
            --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
            --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
            --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
            create
    fi
    temporal-sql-tool \
        --plugin postgres \
        --ep "${POSTGRES_SEEDS}" \
        -u "${POSTGRES_USER}" \
        -p "${DB_PORT}" \
        --db "${DBNAME}" \
        --tls="${POSTGRES_TLS_ENABLED}" \
        --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
        --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
        --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
        --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
        --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
        setup-schema -v 0.0
    temporal-sql-tool \
        --plugin postgres \
        --ep "${POSTGRES_SEEDS}" \
        -u "${POSTGRES_USER}" \
        -p "${DB_PORT}" \
        --db "${DBNAME}" \
        --tls="${POSTGRES_TLS_ENABLED}" \
        --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
        --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
        --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
        --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
        --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
        update-schema -d "${SCHEMA_DIR}"

    VISIBILITY_SCHEMA_DIR=${SCHEMA_BASE_DIR}/visibility/versioned
    if [[ ${VISIBILITY_DBNAME} != "${POSTGRES_USER}" && ${SKIP_DB_CREATE} != true ]]; then
        temporal-sql-tool \
            --plugin postgres \
            --ep "${POSTGRES_SEEDS}" \
            -u "${POSTGRES_USER}" \
            -p "${DB_PORT}" \
            --db "${VISIBILITY_DBNAME}" \
            --tls="${POSTGRES_TLS_ENABLED}" \
            --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
            --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
            --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
            --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
            --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
            create
    fi
    temporal-sql-tool \
        --plugin postgres \
        --ep "${POSTGRES_SEEDS}" \
        -u "${POSTGRES_USER}" \
        -p "${DB_PORT}" \
        --db "${VISIBILITY_DBNAME}" \
        --tls="${POSTGRES_TLS_ENABLED}" \
        --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
        --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
        --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
        --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
        --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
        setup-schema -v 0.0
    temporal-sql-tool \
        --plugin postgres \
        --ep "${POSTGRES_SEEDS}" \
        -u "${POSTGRES_USER}" \
        -p "${DB_PORT}" \
        --db "${VISIBILITY_DBNAME}" \
        --tls="${POSTGRES_TLS_ENABLED}" \
        --tls-disable-host-verification="${POSTGRES_TLS_DISABLE_HOST_VERIFICATION}" \
        --tls-cert-file "${POSTGRES_TLS_CERT_FILE}" \
        --tls-key-file "${POSTGRES_TLS_KEY_FILE}" \
        --tls-ca-file "${POSTGRES_TLS_CA_FILE}" \
        --tls-server-name "${POSTGRES_TLS_SERVER_NAME}" \
        update-schema -d "${VISIBILITY_SCHEMA_DIR}"
}

# === Server setup ===

register_default_namespace() {
    echo "Registering default namespace: ${DEFAULT_NAMESPACE}."
    if ! temporal operator namespace describe "${DEFAULT_NAMESPACE}"; then
        echo "Default namespace ${DEFAULT_NAMESPACE} not found. Creating..."
        temporal operator namespace create --retention "${DEFAULT_NAMESPACE_RETENTION}" \
            --description "Default namespace for Temporal Server." "${DEFAULT_NAMESPACE}"
        echo "Default namespace ${DEFAULT_NAMESPACE} registration complete."
    else
        echo "Default namespace ${DEFAULT_NAMESPACE} already registered."
    fi
}

add_custom_search_attributes() {
    until temporal operator search-attribute list --namespace "${DEFAULT_NAMESPACE}"; do
      echo "Waiting for namespace cache to refresh..."
      sleep 1
    done
    echo "Namespace cache refreshed."

    echo "Adding Custom*Field search attributes."
    temporal operator search-attribute create --namespace "${DEFAULT_NAMESPACE}" \
        --name CustomKeywordField --type Keyword \
        --name CustomStringField --type Text \
        --name CustomTextField --type Text \
        --name CustomIntField --type Int \
        --name CustomDatetimeField --type Datetime \
        --name CustomDoubleField --type Double \
        --name CustomBoolField --type Bool
}

setup_server(){
    echo "Temporal CLI address: ${TEMPORAL_ADDRESS}."

    until temporal operator cluster health | grep -q SERVING; do
        echo "Waiting for Temporal server to start..."
        sleep 1
    done
    echo "Temporal server started."

    if [[ ${SKIP_DEFAULT_NAMESPACE_CREATION} != true ]]; then
        register_default_namespace
    fi

    if [[ ${SKIP_ADD_CUSTOM_SEARCH_ATTRIBUTES} != true ]]; then
        add_custom_search_attributes
    fi
}

start_server(){
    local flags=
    if [[ -n ${SERVICES} ]]; then
        SERVICES="${SERVICES//,/ }"
        for i in $SERVICES; do flags="${flags} --service=$i"; done
    fi
    dockerize -template /config_template.yaml:/etc/temporal/config/docker.yaml

    exec /etc/temporal/temporal-server --env docker start $flags
}
