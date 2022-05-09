#/bin/bash

# See the file `skia.org.zone` for how this script is used.

# Note that --zone refers to the DNS zone name, not a [GCS
# zone](https://cloud.google.com/compute/docs/regions-zones).
gcloud dns record-sets import --zone skia-org --zone-file-format skia.org.zone