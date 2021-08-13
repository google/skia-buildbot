#!/bin/bash
set -e
set -o pipefail

# Retrieve a secret from berglas, edit, then write back to berglas.

if [ $# -ne 2 ]; then
    echo "$0 <cluster-name> <secret-name>"
    exit 1
fi

REL=$(dirname "$0")
source ${REL}/config.sh

CLUSTER=$1
SECRET_NAME=$2

source ${REL}/../../bash/ramdisk.sh

FILES=$(${REL}/get-secret.sh ${CLUSTER} ${SECRET_NAME} \
  | kubectl apply -f - --dry-run -o json \
  | jq -j '.data | keys | .[] | . + " "')
echo ${FILES}
for FILE in ${FILES}; do
  ${REL}/get-secret.sh ${CLUSTER} ${SECRET_NAME} \
  | kubectl apply -f - --dry-run -o json \
  | jq -r ".data[\"${FILE}\"]" \
  | base64 -d > /tmp/ramdisk/${FILE}
done

echo "Downloaded the ${SECRET_NAME} secrets to /tmp/ramdisk."
echo ""
read -r -p "Press enter when you are done editing..." key

${REL}/add-secret-from-directory.sh /tmp/ramdisk ${CLUSTER} ${SECRET_NAME}