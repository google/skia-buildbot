export PROJECT_ID=skia-public
export BUCKET_ID=skia-secrets
export KEY="projects/${PROJECT_ID}/locations/global/keyRings/berglas/cryptoKeys/berglas-key"
export ACCESS_CONTROL="--member group:skia-root@google.com"

# Converts a cluster common name, e.g. "skia-public", into the value that would
# get returned by `kubectl config current-context`. Needed because those are
# very different names under GKE.
#
# $1 - The common name of the cluster, e.g. "skia-public".
function cluster_long_name() {
    if [ "$1" == "skia-public" ]; then
        echo "gke_skia-public_us-central1-a_skia-public"
    elif [ "$1" == "skia-corp" ]; then
        echo "gke_google.com:skia-corp_us-central1-a_skia-corp"
    else
        echo $1
    fi
}

# Confirms that we are currently talking to the desired cluster.
#
# $1 - The common name of the cluster, e.g. "skia-public".
function confirm_cluster() {
    K8S_CLUSTER=$(kubectl config current-context)
    if [ "$K8S_CLUSTER" != "$(cluster_long_name $1)" ]; then
        echo "Wrong cluster, must be run in $CLUSTER."
        exit 1
    fi
}

