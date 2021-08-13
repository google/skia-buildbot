#!/bin/bash
set -e
set -o pipefail

# Add a secret to berglas from stdin.
#
# The stdin stream should be a kubernetes secret file formatted as YAML.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

CLUSTER=$1
SECRET_NAME=$2

REL=$(dirname "$0")
source ${REL}/config.sh

# bergals only understands a single line, so we base64 encode the whole file,
# and then use awk to add a single newline to the end of the base64, which
# berglas also needs.
base64 --wrap=0 \
| awk '{ print $0 }' \
| berglas update ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} - --create-if-missing --key ${KEY}

berglas grant ${BUCKET_ID}/${CLUSTER}/${SECRET_NAME} ${ACCESS_CONTROL}