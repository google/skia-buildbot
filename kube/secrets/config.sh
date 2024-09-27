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

# Derives a GCP secret name from a project and service account name by adding
# the project name (without "google.com:") as a prefix and "-sa-key" as suffix.
function service_account_secret_name() {
    if [ $# -ne 2 ]; then
        echo "service_account_secret_name <project id> <service-account-name>" >&2
        exit 1
    fi
    PROJECT="$1"
    SA_NAME="$2"
    echo "${PROJECT#"google.com:"}-${SA_NAME}-sa-key"
}

# Converts a berglas secret path of the form $cluster/$secret to a GCP secret name.
function berglas_to_gcp_secret_name() {
    srcSecretName="$(echo "$line" | awk '{print $1;}')"
    cluster="$(echo $srcSecretName | cut -d "/" -f 1)"
    srcSecretBaseName="$(echo $srcSecretName | cut -d "/" -f 2)"

    dstSecretName="${srcSecretBaseName%-service-account}"
    if [ -z "${dstSecretName##*-token}" ] || [ -z "${dstSecretName##*-secret}" ] || [ -z "${dstSecretName##*-secrets}" ]; then
        dstSecretName="${dstSecretName}"
    else
        dstSecretName="${dstSecretName}-sa-key"
    fi
    if [[ "$dstSecretName" == $cluster-* ]] || [[ "$cluster" == "etc" ]]; then
        dstSecretName="$dstSecretName"
    else
        dstSecretName="$cluster-$dstSecretName"
    fi
    echo "$dstSecretName"
}
