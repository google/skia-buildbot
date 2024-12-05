#!/bin/bash

set -e

../kube/secrets/generate-new-key-for-service-account.sh skia-swarming-bots chromium-swarm-bots
../kube/secrets/generate-new-key-for-service-account.sh skia-swarming-bots-internal chrome-swarming-bots

printf 'You should now run:\n'
printf '    cd ansible\n'
printf '    ansible-playbook ./switchboard/build_and_release_metadata_server_ansible.yml\n'
printf '    (wait for the CL to update the metadata server to land)\n'
printf '    ansible-playbook ./switchboard/jumphosts.yml\n'