#/bin/bash

# Set the timeout for each backend since we can't current do that via
# kubernetes yaml file.

# Note that this script requires that 'jq' be installed.

for backend in $(gcloud compute backend-services list --format=json | jq -r '.[].name'); do
  echo "Fixing up " ${backend}
  gcloud compute backend-services update ${backend} --global --timeout=600
done
