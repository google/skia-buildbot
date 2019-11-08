#!/bin/bash

# Resolve the IP address to avoid putting "unassigned-hostname" into our SSH known_hosts.
ipaddr="$(getent hosts unassigned-hostname | cut -f1 -d ' ')"

# This will be the first time we connect to the host. Automatically add its host key to known_hosts.
ssh-keyscan -H "${ipaddr}" >> ~/.ssh/known_hosts

ansible-playbook -i "$ipaddr," post-preseed.yaml
