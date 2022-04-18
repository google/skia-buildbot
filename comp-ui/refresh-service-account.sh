#!/bin/bash

set -e

../kube/secrets/generate-new-key-for-service-account.sh skia-public etc perf-comp-ui

printf 'You should now run:\n'
printf '    cd ansible\n'
printf '    ansible-playbook ./switchboard/build_and_release_compui.yml\n'