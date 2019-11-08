#!/bin/bash

# Resolve the IP address to avoid putting "unassigned-hostname" into our SSH authorized_hosts.
ipaddr="$(getent hosts unassigned-hostname | cut -f1 -d ' ')"

ansible-playbook -i "$ipaddr," post-preseed.yaml
