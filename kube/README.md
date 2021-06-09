# Kubernetes config and applications

Scripts, YAML files, and utility apps to run our kubernetes cluster(s). Each
cluster will have its own subdirectory that matches the name of the GCE project.

## Ingress

The ingress configs presume that the IP address and certs have already been
created and named, both of which can be done via command line.

Upload certs:

    gcloud compute ssl-certificates create skia-org --certificate=skia.pem --private-key=skia.key

Take care when copying the certs around, for example, download them onto a
ramdrive and unmount the ramdrive after they have been uploaded. See
'create-sa.sh' in this directory.

Reserving a named global IP address:

    gcloud compute addresses create skia-org --global

## pushk and kube/clusters/config.json

The definitive list of clusters and how to talk to each one is stored in
`kube/clusters/config.json`.

This config file also defines the git repo where YAML files are stored and where
to checkout that repo when pushing. The location of the checkout can be set by
setting the PUSHK_GITDIR environment variable.

The k8s YAML files are checked into https://skia.googlesource.com/k8s-config/,
with one sub-directory for each cluster.

See http://go/corp-ssh-helper for details on setting up SSH.

When you run pushk it will update the images for all the clusters and then run
`kubectl apply` for each file and for each cluster.

## Standing up a new cluster in a different project

1. Add a new `__skia_NNN` function to `clusters.sh`.
2. Create the `config-NNN.sh` file.
3. Copy and modify the `create-cluster-corp.sh` script.
4. Add a node pool if necessary using the web UI.
5. Update `kube/clusters/config.json` with info on the new cluster.
