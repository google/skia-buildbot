Kubernetes config and applications
==================================

Scripts, YAML files, and utility apps to run our kubernetes cluster(s). Each
cluster will have its own subdirectory that matches the name of the GCE
project.

Ingress
=======

The ingress configs presume that the IP address and certs have already been
created and named, both of which can be done via command line.

Upload certs:

    gcloud compute ssl-certificates create skia-org --certificate=skia.pem --private-key=skia.key

Take care when copying the certs around, for example, download them onto a
ramdrive and unmount the ramdrive after they have been uploaded. See
'create-sa.sh' in this directory.

Reserving a named global IP address:

    gcloud compute addresses create skia-org --global
