#!/bin/bash

# Linux machines are configured via Ansible (with the exception of CT instances), so no setup steps
# are necessary. We keep this script around in case that ceases to be true in the future.

set -e -x

echo "Please run the //skolo/ansible/switchboard/linux.yml Ansible playbook"
echo "to finish setting up this machine."
