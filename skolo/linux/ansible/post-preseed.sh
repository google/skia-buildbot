#!/bin/bash

set -e

# Resolve the IP address to avoid putting "unassigned-hostname" into our SSH known_hosts.
ipaddr="$(getent hosts unassigned-hostname | cut --fields=1 --delimiter=' ')"

# This will be the first time we connect to the host. Automatically add its host key to known_hosts.
ssh-keyscan -H "${ipaddr}" >> ~/.ssh/known_hosts

# We need to set up authorized_keys before using Ansible.
tmpdir="$(mktemp --directory bot-ssh)"
# Include all authorized_keys from this machine.
cp "${HOME}/.ssh/authorized_keys" "${tmpdir}/authorized_keys"
# Also ensure the current user's key is included.
cat "${HOME}/.ssh/id_rsa.pub" >> "${tmpdir}/authorized_keys"
# Fix permissions and ownership.
chmod --recursive 0600 "${tmpdir}"
chown --recursive chrome-bot:chrome-bot "${tmpdir}"
echo "When prompted, enter \"Debian preseed\" password."
scp -pr "${tmpdir}" "${ipaddr}:/home/chrome-bot/.ssh"
rm -rf "${tmpdir}"

# Run the Ansible script.
ansible-playbook -i "$ipaddr," post-preseed.yaml
