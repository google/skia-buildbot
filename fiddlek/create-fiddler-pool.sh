#/bin/bash

# Creates the pool of nodes reserved for running fiddlers.

set -e -x
source ../kube/config.sh

gcloud beta container node-pools create fiddler-pool \
      --node-labels=reservedFor=fiddler \
      --node-taints=reservedFor=fiddler:NoSchedule \
      --machine_type=n1-standard-8 \
      --num_nodes=6
