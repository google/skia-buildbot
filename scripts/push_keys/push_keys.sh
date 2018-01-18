#!/bin/bash

# Push service account keys to the jumphosts.

set -e

JUMPHOSTS="linux-01.skolo"

workdir="/tmp/service_account_keys"
rm -rf ${workdir}
mkdir -p ${workdir}

# Run the Go program to rotate the keys.
go run ./scripts/push_keys/push_keys.go --outdir=${workdir}

for hostname in "$(ls ${workdir})"; do
  for file in "$(ls ${workdir}/${hostname})"; do
    echo "Copying ${file} to ${hostname}"
    cat ${workdir}/${hostname}/${file} | ssh ${hostname} "cat > ${file}"
    suffix=".gpg"
    keyfile="/etc/${file%$suffix}"
    ssh -t ${hostname} "sudo touch ${keyfile} && sudo chown root:root ${keyfile} && sudo chmod 600 ${keyfile} && sudo gpg -o ${keyfile} ${file} && sudo rm ${file}"
  done
done

#rm -rf ${workdir}
