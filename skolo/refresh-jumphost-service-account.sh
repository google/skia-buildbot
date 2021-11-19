#!/bin/bash

set -e

../kube/secrets/generate-new-key-for-service-account.sh skia-public etc skolo-jumphost

printf 'You should now run:\n'
printf '    cd ansible\n'
printf '    ansible-playbook ./switchboard/jumphosts.yml\n'