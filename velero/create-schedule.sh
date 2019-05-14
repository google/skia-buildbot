#/bin/sh

set -e

source ../kube/clusters.sh

__skia_corp
velero create schedule daily-backup --schedule="0 1 * * *"

__skia_public
velero create schedule daily-backup --schedule="0 2 * * *"

