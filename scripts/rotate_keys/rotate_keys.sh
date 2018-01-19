#!/bin/bash

# Rotate service account keys and push them to the jumphosts.

set -e

workdir="/tmp/service_account_keys"
rm -rf ${workdir}
mkdir -p ${workdir}

keydir="/etc"

echo "WARNING: If anything goes wrong during execution of this script, you MUST ensure that any leftover key files are cleaned up."
echo "In particular:"
echo "    rm -rf ${workdir}"
echo "And ensure that no key files remain in the home directory or under ${keydir} on any jumphost."
echo

# Run the Go program to rotate the keys.
echo "Use the following decryption passphrase to decrypt the key files when prompted:"
go run ./scripts/rotate_keys/rotate_keys.go --outdir=${workdir}

for path in ${workdir}/*; do
  echo
  hostname="$(basename ${path})"
  for fullpath in ${path}/*; do
    file="$(basename ${fullpath})"
    echo "Copying ${file} to ${hostname}"
    cat ${workdir}/${hostname}/${file} | ssh ${hostname} "cat > ${file}"
    suffix=".gpg"
    keyfile="${keydir}/${file%$suffix}"
    ssh -t ${hostname} "sudo touch ${keyfile} && sudo chown root:root ${keyfile} && sudo chmod 600 ${keyfile} && sudo gpg -o ${keyfile} ${file} && sudo rm ${file}"
  done
done

rm -rf ${workdir}
