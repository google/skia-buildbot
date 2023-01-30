#/bin/bash

# See the file `skia.org.zone` for how this script is used.

# Note that --zone refers to the DNS zone name, not a [GCS
# zone](https://cloud.google.com/compute/docs/regions-zones).
#
# The command will throw errors for records which already exist. These can be
# safely ignored.
gcloud dns record-sets import --project skia-public --delete-all-existing --zone skia-org --zone-file-format skia.org.zone