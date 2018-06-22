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

Configuration
=============

The kubernetes configuration files are kept in a separate repo that will
automaticaly be checked out under /tmp by the pushk command.

Continuous Deployment
=====================

To set up continuous deployment create a Dockerfile that builds the image you
want, and then add a trigger to build that image on commits to the repo in the
[GCP Container Builder](https://cloud.google.com/container-builder/).

The continuous-deploy pod listens for pubsub messages of success from
container builder and when it receives them it runs pushk for a set of images.

Update the continuous-deploy yaml file to add the image you want to get
deployed.

See https://cloud.google.com/container-builder/docs/send-build-notifications
for more details.
