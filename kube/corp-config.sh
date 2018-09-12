DIR=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
source "${DIR}/clusters.sh"
# Common config values used by all create-* scripts.

# Your Project ID
PROJECT_ID=google.com:skia-corp

# The Project ID rewritten as GCE does when putting in an email address.
PROJECT_SUBDOMAIN=skia-corp.google.com

# Your Zone. E.g. us-west1-c
ZONE=us-central1-a

# Name for your cluster we will create or modify. E.g. example-secure-cluster
CLUSTER_NAME=skia-corp

# The ID of the security account used by kubernetes.
SA_NAME=${CLUSTER_NAME}-k8s

# Switch gcloud and kubectl to the skia-corp project/cluster.
__skia_corp
