#!/bin/bash

# Resolve the IP address to avoid putting "unassigned-hostname" into our SSH known_hosts.
ipaddr="$(getent hosts unassigned-hostname | cut -f1 -d ' ')"

# This will be the first time we connect to the host. Automatically add its host key to known_hosts.
ssh-keyscan -H "${ipaddr}" >> ~/.ssh/known_hosts

# We need to set up authorized_keys before using Ansible.
echo "When prompted, enter \"Debian preseed\" password."
# Include all authorized_keys from this machine and also ensure the current user's key is included.
# https://superuser.com/a/400720
cat "${HOME}/.ssh/id_rsa.pub" "${HOME}/.ssh/authorized_keys" | \
  ssh "${ipaddr}" -T "cat > /home/chrome-bot/.ssh/authorized_keys"

# Run the Ansible script.
ansible-playbook -i "$ipaddr," post-preseed.yaml
