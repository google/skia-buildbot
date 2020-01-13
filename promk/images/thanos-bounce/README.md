This container sets up a reverse TCP tunnel from the pod it's running in to a
container running the Thanos sidecar. The container running the Thanos sidecar
needs to have netcat installed. This container needs to have netcat and kubectl
installed.

The reverse tunnel is accomplised via: https://layerci.com/blog/container-tcp-tunnel/

## kubeconfig.yaml

To regenerate the 'kubeconfig.yaml' file first define:

    export GET_CMD="gcloud container clusters describe [CLUSTER] --zone=[ZONE]"

Then run:

    cat > kubeconfig.yaml <<EOF
    apiVersion: v1
    kind: Config
    current-context: my-cluster
    contexts: [{name: my-cluster, context: {cluster: cluster-1, user: user-1}}]
    users: [{name: user-1, user: {auth-provider: {name: gcp}}}]
    clusters:
    - name: cluster-1
    cluster:
        server: "https://$(eval "$GET_CMD --format='value(endpoint)'")"
        certificate-authority-data: "$(eval "$GET_CMD --format='value(masterAuth.clusterCaCertificate)'")"
    EOF

Found in https://ahmet.im/blog/authenticating-to-gke-without-gcloud/

The kubeconfig.yaml contains no secret information and can be safely checked in
to the git repository.